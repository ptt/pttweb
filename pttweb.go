package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"html/template"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"golang.org/x/net/context"

	"github.com/ptt/pttweb/atomfeed"
	"github.com/ptt/pttweb/cache"
	"github.com/ptt/pttweb/captcha"
	"github.com/ptt/pttweb/extcache"
	"github.com/ptt/pttweb/page"
	manpb "github.com/ptt/pttweb/proto/man"
	"github.com/ptt/pttweb/pttbbs"
	"github.com/ptt/pttweb/pushstream"

	"github.com/gorilla/mux"
)

const (
	ArticleCacheTimeout           = time.Minute * 10
	BbsIndexCacheTimeout          = time.Minute * 5
	BbsIndexLastPageCacheTimeout  = time.Minute * 1
	BbsSearchCacheTimeout         = time.Minute * 10
	BbsSearchLastPageCacheTimeout = time.Minute * 3
)

var (
	ErrOver18CookieNotEnabled = errors.New("board is over18 but cookie not enabled")
	ErrSigMismatch            = errors.New("push stream signature mismatch")
)

var ptt pttbbs.Pttbbs
var pttSearch pttbbs.Pttbbs
var mand manpb.ManServiceClient
var router *mux.Router
var cacheMgr *cache.CacheManager
var extCache extcache.ExtCache
var atomConverter *atomfeed.Converter

var configPath string
var config PttwebConfig

func init() {
	flag.StringVar(&configPath, "conf", "config.json", "config file")
}

func loadConfig() error {
	f, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return err
	}

	return config.CheckAndFillDefaults()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.Parse()

	if err := loadConfig(); err != nil {
		log.Fatal("loadConfig:", err)
	}

	// Init RemotePtt
	var err error
	ptt, err = pttbbs.NewGrpcRemotePtt(config.BoarddAddress)
	if err != nil {
		log.Fatal("cannot connect to boardd:", config.BoarddAddress, err)
	}

	if config.SearchAddress != "" {
		pttSearch, err = pttbbs.NewGrpcRemotePtt(config.SearchAddress)
		if err != nil {
			log.Fatal("cannot connect to boardd:", config.SearchAddress, err)
		}
	} else {
		pttSearch = ptt
	}

	// Init mand connection
	if conn, err := grpc.Dial(config.MandAddress, grpc.WithInsecure(), grpc.WithBackoffMaxDelay(time.Second*5)); err != nil {
		log.Fatal("cannot connect to mand:", config.MandAddress, err)
	} else {
		mand = manpb.NewManServiceClient(conn)
	}

	// Init cache manager
	cacheMgr = cache.NewCacheManager(config.MemcachedAddress, config.MemcachedMaxConn)

	// Init extcache module if configured
	extCache = extcache.New(config.ExtCacheConfig)

	// Init atom converter.
	atomConverter = &atomfeed.Converter{
		FeedTitleTemplate: template.Must(template.New("").Parse(config.AtomFeedTitleTemplate)),
		LinkFeed: func(brdname string) (string, error) {
			return config.FeedPrefix + "/" + brdname + ".xml", nil
		},
		LinkArticle: func(brdname, filename string) (string, error) {
			u, err := router.Get("bbsarticle").URLPath("brdname", brdname, "filename", filename)
			if err != nil {
				return "", err
			}
			return config.SitePrefix + u.String(), nil
		},
	}

	// Load templates
	if err := page.LoadTemplates(config.TemplateDirectory, templateFuncMap()); err != nil {
		log.Fatal("cannot load templates:", err)
	}

	// Init router
	router = createRouter()
	http.Handle("/", router)

	if len(config.Bind) == 0 {
		log.Fatal("No bind addresses specified in config")
	}
	for _, addr := range config.Bind {
		part := strings.SplitN(addr, ":", 2)
		if len(part) != 2 {
			log.Fatal("Invalid bind address: ", addr)
		}
		if listener, err := net.Listen(part[0], part[1]); err != nil {
			log.Fatal("Listen failed for address: ", addr, " error: ", err)
		} else {
			if part[0] == "unix" {
				os.Chmod(part[1], 0777)
				// Ignores errors, we can't do anything to those.
			}
			svr := &http.Server{
				MaxHeaderBytes: 64 * 1024,
			}
			go svr.Serve(listener)
		}
	}

	progExit := make(chan os.Signal)
	signal.Notify(progExit, os.Interrupt)
	<-progExit
}

func ReplaceVars(p string) string {
	var subs = [][]string{
		{`aidc`, `[0-9A-Za-z\-_]+`},
		{`brdname`, `[0-9A-Za-z][0-9A-Za-z_\.\-]+`},
		{`filename`, `[MG]\.\d+\.A(?:\.[0-9A-F]+)?`},
		{`fullpath`, `[0-9A-Za-z_\.\-\/]+`},
		{`page`, `\d+`},
	}
	for _, s := range subs {
		p = strings.Replace(p, fmt.Sprintf(`{%v}`, s[0]), fmt.Sprintf(`{%v:%v}`, s[0], s[1]), -1)
	}
	return p
}

func createRouter() *mux.Router {
	r := mux.NewRouter()

	staticFileServer := http.FileServer(http.Dir(filepath.Join(config.TemplateDirectory, `static`)))
	r.PathPrefix(`/static/`).
		Handler(http.StripPrefix(`/static/`, staticFileServer))

	// Classlist
	r.Path(ReplaceVars(`/cls/{bid:[0-9]+}`)).
		Handler(ErrorWrapper(handleCls)).
		Name("classlist")
	r.Path(ReplaceVars(`/bbs/{x:|index\.html|hotboards\.html}`)).
		Handler(ErrorWrapper(handleHotboards))

	// Board
	r.Path(ReplaceVars(`/bbs/{brdname}{x:/?}`)).
		Handler(ErrorWrapper(handleBbsIndexRedirect))
	r.Path(ReplaceVars(`/bbs/{brdname}/index.html`)).
		Handler(ErrorWrapper(handleBbs)).
		Name("bbsindex")
	r.Path(ReplaceVars(`/bbs/{brdname}/index{page}.html`)).
		Handler(ErrorWrapper(handleBbs)).
		Name("bbsindex_page")
	r.Path(ReplaceVars(`/bbs/{brdname}/search`)).
		Handler(ErrorWrapper(handleBbsSearch)).
		Name("bbssearch")

	// Feed
	r.Path(ReplaceVars(`/atom/{brdname}.xml`)).
		Handler(ErrorWrapper(handleBoardAtomFeed)).
		Name("atomfeed")

	// Post
	r.Path(ReplaceVars(`/bbs/{brdname}/{filename}.html`)).
		Handler(ErrorWrapper(handleArticle)).
		Name("bbsarticle")
	r.Path(ReplaceVars(`/b/{brdname}/{aidc}`)).
		Handler(ErrorWrapper(handleAidc)).
		Name("bbsaidc")

	if config.EnablePushStream {
		r.Path(ReplaceVars(`/poll/{brdname}/{filename}.html`)).
			Handler(ErrorWrapper(handleArticlePoll)).
			Name("bbsarticlepoll")
	}

	r.Path(ReplaceVars(`/ask/over18`)).
		Handler(ErrorWrapper(handleAskOver18)).
		Name("askover18")

	// Man
	r.Path(ReplaceVars(`/man/{fullpath}.html`)).
		Handler(ErrorWrapper(handleMan)).
		Name("manentry")

	// Captcha
	if cfg := config.captchaConfig(); cfg.Enabled {
		if err := captcha.Install(cfg, r); err != nil {
			log.Fatal("captcha.Install:", err)
		}
	}

	return r
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"route_bbsindex": func(b pttbbs.Board) (*url.URL, error) {
			return router.Get("bbsindex").URLPath("brdname", b.BrdName)
		},
		"route_bbsindex_page": func(b pttbbs.Board, pg int) (*url.URL, error) {
			return router.Get("bbsindex_page").URLPath("brdname", b.BrdName, "page", strconv.Itoa(pg))
		},
		"route_classlist_bid": func(bid int) (*url.URL, error) {
			return router.Get("classlist").URLPath("bid", strconv.Itoa(bid))
		},
		"route_classlist": func(b pttbbs.Board) (*url.URL, error) {
			return router.Get("classlist").URLPath("bid", strconv.FormatUint(uint64(b.Bid), 10))
		},
		"valid_article": func(a pttbbs.Article) bool {
			return pttbbs.IsValidArticleFileName(a.FileName)
		},
		"route_bbsarticle": func(brdname, filename, title string) (*url.URL, error) {
			if config.EnableLinkOriginalInAllPost && brdname == pttbbs.AllPostBrdName {
				if origBrdName, ok := pttbbs.BrdNameFromAllPostTitle(title); ok {
					brdname = origBrdName
				}
			}
			return router.Get("bbsarticle").URLPath("brdname", brdname, "filename", filename)
		},
		"route_askover18": func() (*url.URL, error) {
			return router.Get("askover18").URLPath()
		},
		"route_manentry": func(e *manpb.Entry) (*url.URL, error) {
			var index string
			if e.IsDir {
				index = "index"
			}
			return router.Get("manentry").URLPath("fullpath", path.Join(e.BoardName, e.Path, index))
		},
		"route_manarticle": func(brdname, p string) (*url.URL, error) {
			return router.Get("manentry").URLPath("fullpath", path.Join(brdname, p))
		},
		"route_manindex": func(brdname, p string) (*url.URL, error) {
			return router.Get("manentry").URLPath("fullpath", path.Join(brdname, p, "index"))
		},
		"route_manparent": func(brdname, p string) (*url.URL, error) {
			dir := path.Join(brdname, path.Dir(p))
			return router.Get("manentry").URLPath("fullpath", path.Join(dir, "index"))
		},
		"route": func(where string, attrs ...string) (*url.URL, error) {
			return router.Get(where).URLPath(attrs...)
		},
		"route_search_author": func(b pttbbs.Board, author string) (*url.URL, error) {
			if !pttbbs.IsValidUserID(author) {
				return nil, nil
			}
			return bbsSearchURL(b, "author:"+author)
		},
		"route_search_thread": func(b pttbbs.Board, title string) (*url.URL, error) {
			return bbsSearchURL(b, "thread:"+pttbbs.Subject(title))
		},
		"static_prefix": func() string {
			return config.StaticPrefix
		},
		"colored_counter":      colored_counter,
		"decorate_board_nuser": decorate_board_nuser,
		"post_mark":            post_mark,
		"ga_account": func() string {
			return config.GAAccount
		},
		"ga_domain": func() string {
			return config.GADomain
		},
		"slice": func(args ...interface{}) []interface{} {
			return args
		},
	}
}

func setCommonResponseHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Server", "Cryophoenix")
	h.Set("Content-Type", "text/html; charset=utf-8")
}

type ErrorWrapper func(*Context, http.ResponseWriter) error

func (fn ErrorWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCommonResponseHeaders(w)

	if err := clarifyRemoteError(handleRequest(w, r, fn)); err != nil {
		if pg, ok := err.(page.Page); ok {
			if err = page.ExecutePage(w, pg); err != nil {
				log.Println("Failed to emit error page:", err)
			}
			return
		}
		internalError(w, err)
	}
}

func internalError(w http.ResponseWriter, err error) {
	log.Println(err)
	w.WriteHeader(http.StatusInternalServerError)
	page.ExecutePage(w, &page.Error{
		Title:       `500 - Internal Server Error`,
		ContentHtml: `500 - Internal Server Error / Server Too Busy.`,
	})
}

func handleRequest(w http.ResponseWriter, r *http.Request, f func(*Context, http.ResponseWriter) error) error {
	c := new(Context)
	if err := c.MergeFromRequest(r); err != nil {
		return err
	}

	if err := f(c, w); err != nil {
		return err
	}

	return nil
}

func handleAskOver18(c *Context, w http.ResponseWriter) error {
	from := c.R.FormValue("from")
	if from == "" || !isSafeRedirectURI(from) {
		from = "/"
	}

	if c.R.Method == "POST" {
		if c.R.PostFormValue("yes") != "" {
			setOver18Cookie(w)
			w.Header().Set("Location", from)
		} else {
			w.Header().Set("Location", "/")
		}
		w.WriteHeader(http.StatusFound)
		return nil
	} else {
		return page.ExecutePage(w, &page.AskOver18{
			From: from,
		})
	}
}

func handleClsRoot(c *Context, w http.ResponseWriter) error {
	return handleClsWithBid(c, w, pttbbs.BoardID(1))
}

func handleCls(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	bid, err := strconv.Atoi(vars["bid"])
	if err != nil {
		return err
	}
	return handleClsWithBid(c, w, pttbbs.BoardID(bid))
}

func handleClsWithBid(c *Context, w http.ResponseWriter, bid pttbbs.BoardID) error {
	if bid < 1 {
		return NewNotFoundError(fmt.Errorf("invalid bid: %v", bid))
	}

	board, err := pttbbs.OneBoard(ptt.GetBoards(pttbbs.BoardRefByBid(bid)))
	if err != nil {
		return err
	}
	children, err := ptt.GetBoards(pttbbs.BoardRefsByBid(board.Children)...)
	if err != nil {
		return err
	}
	return page.ExecutePage(w, &page.Classlist{
		Boards: validBoards(children),
	})
}

func handleHotboards(c *Context, w http.ResponseWriter) error {
	boards, err := ptt.Hotboards()
	if err != nil {
		return err
	}
	return page.ExecutePage(w, &page.Classlist{
		Boards:         validBoards(boards),
		IsHotboardList: true,
	})
}

func validBoards(boards []pttbbs.Board) []pttbbs.Board {
	var valids []pttbbs.Board
	for _, b := range boards {
		if pttbbs.IsValidBrdName(b.BrdName) && !b.Hidden {
			valids = append(valids, b)
		}
	}
	return valids
}

func handleBbsIndexRedirect(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	if url, err := router.Get("bbsindex").URLPath("brdname", vars["brdname"]); err != nil {
		return err
	} else {
		w.Header().Set("Location", url.String())
	}
	w.WriteHeader(http.StatusFound)
	return nil
}

func handleBbs(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]

	// Note: TODO move timeout into the generating function.
	// We don't know if it is the last page without entry count.
	pageNo := 0
	timeout := BbsIndexLastPageCacheTimeout

	if pg, err := strconv.Atoi(vars["page"]); err == nil {
		pageNo = pg
		timeout = BbsIndexCacheTimeout
	}

	brd, err := getBoardByName(c, brdname)
	if err != nil {
		return err
	}

	obj, err := cacheMgr.Get(&BbsIndexRequest{
		Brd:  *brd,
		Page: pageNo,
	}, ZeroBbsIndex, timeout, generateBbsIndex)
	if err != nil {
		return err
	}
	bbsindex := obj.(*BbsIndex)

	if !bbsindex.IsValid {
		return NewNotFoundError(fmt.Errorf("not a valid cache.BbsIndex: %v/%v", brd.BrdName, pageNo))
	}

	return page.ExecutePage(w, (*page.BbsIndex)(bbsindex))
}

func bbsSearchURL(b pttbbs.Board, query string) (*url.URL, error) {
	u, err := router.Get("bbssearch").URLPath("brdname", b.BrdName)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("q", query)
	u.RawQuery = q.Encode()
	return u, nil
}

func parseKeyValueTerm(term string) (pttbbs.SearchPredicate, bool) {
	kv := strings.SplitN(term, ":", 2)
	if len(kv) != 2 {
		return nil, false
	}
	k, v := strings.ToLower(kv[0]), kv[1]
	if len(v) == 0 {
		return nil, false
	}
	switch k {
	case "author":
		return pttbbs.WithAuthor(v), true
	case "recommend":
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, false
		}
		return pttbbs.WithRecommend(n), true
	}
	return nil, false
}

func parseQuery(query string) ([]pttbbs.SearchPredicate, error) {
	// Special case, thread takes up all the query.
	if strings.HasPrefix(query, "thread:") {
		return []pttbbs.SearchPredicate{
			pttbbs.WithExactTitle(strings.TrimSpace(strings.TrimPrefix(query, "thread:"))),
		}, nil
	}

	segs := strings.Split(query, " ")
	var titleSegs []string
	var preds []pttbbs.SearchPredicate
	for _, s := range segs {
		if p, ok := parseKeyValueTerm(s); ok {
			preds = append(preds, p)
		} else {
			titleSegs = append(titleSegs, s)
		}
	}
	title := strings.TrimSpace(strings.Join(titleSegs, " "))
	if title != "" {
		// Put title first.
		preds = append([]pttbbs.SearchPredicate{
			pttbbs.WithTitle(title),
		}, preds...)
	}
	return preds, nil
}

func handleBbsSearch(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]

	if c.R.ParseForm() != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	form := c.R.Form
	query := strings.TrimSpace(form.Get("q"))

	pageNo := 1
	// Note: TODO move timeout into the generating function.
	timeout := BbsSearchLastPageCacheTimeout

	if pageStr := form.Get("page"); pageStr != "" {
		pg, err := strconv.Atoi(pageStr)
		if err != nil || pg <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
		pageNo = pg
		timeout = BbsSearchCacheTimeout
	}

	preds, err := parseQuery(query)
	if err != nil {
		return err
	}

	brd, err := getBoardByName(c, brdname)
	if err != nil {
		return err
	}

	obj, err := cacheMgr.Get(&BbsSearchRequest{
		Brd:   *brd,
		Page:  pageNo,
		Query: query,
		Preds: preds,
	}, ZeroBbsIndex, timeout, generateBbsSearch)
	if err != nil {
		return err
	}
	bbsindex := obj.(*BbsIndex)

	if !bbsindex.IsValid {
		return NewNotFoundError(fmt.Errorf("not a valid cache.BbsIndex: %v/%v", brd.BrdName, pageNo))
	}

	return page.ExecutePage(w, (*page.BbsIndex)(bbsindex))
}

func handleBoardAtomFeed(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]

	timeout := BbsIndexLastPageCacheTimeout

	c.SetSkipOver18()
	brd, err := getBoardByName(c, brdname)
	if err != nil {
		return err
	}

	obj, err := cacheMgr.Get(&BoardAtomFeedRequest{
		Brd: *brd,
	}, ZeroBoardAtomFeed, timeout, generateBoardAtomFeed)
	if err != nil {
		return err
	}
	baf := obj.(*BoardAtomFeed)

	if !baf.IsValid {
		return NewNotFoundError(fmt.Errorf("not a valid cache.BoardAtomFeed: %v", brd.BrdName))
	}

	w.Header().Set("Content-Type", "application/xml")
	if _, err = w.Write([]byte(xml.Header)); err != nil {
		return err
	}
	return xml.NewEncoder(w).Encode(baf.Feed)
}

func handleArticle(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]
	filename := vars["filename"]
	return handleArticleCommon(c, w, brdname, filename)
}

func handleAidc(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]
	aid, err := pttbbs.ParseAid(vars["aidc"])
	if err != nil {
		return NewNotFoundError(fmt.Errorf("board %v, invalid aid: %v", brdname, err))
	}
	return handleArticleCommon(c, w, brdname, aid.Filename())
}

func handleArticleCommon(c *Context, w http.ResponseWriter, brdname, filename string) error {
	var err error

	brd, err := getBoardByName(c, brdname)
	if err != nil {
		return err
	}

	// Render content
	obj, err := cacheMgr.Get(&ArticleRequest{
		Namespace: "bbs",
		Brd:       *brd,
		Filename:  filename,
		Select: func(m pttbbs.SelectMethod, offset, maxlen int) (*pttbbs.ArticlePart, error) {
			return ptt.GetArticleSelect(brd.Ref(), m, filename, "", offset, maxlen)
		},
	}, ZeroArticle, ArticleCacheTimeout, generateArticle)
	// Try older filename when not found.
	if err == pttbbs.ErrNotFound {
		if name, ok := oldFilename(filename); ok {
			if handleArticleCommon(c, w, brdname, name) == nil {
				return nil
			}
		}
	}
	if err != nil {
		return err
	}
	ar := obj.(*Article)

	if !ar.IsValid {
		return NewNotFoundError(nil)
	}

	if len(ar.ContentHtml) > TruncateSize {
		log.Println("Large rendered article:", brd.BrdName, filename, len(ar.ContentHtml))
	}

	pollUrl, longPollUrl, err := uriForPolling(brd.BrdName, filename, ar.CacheKey, ar.NextOffset)
	if err != nil {
		return err
	}

	return page.ExecutePage(w, &page.BbsArticle{
		Title:            ar.ParsedTitle,
		Description:      ar.PreviewContent,
		Board:            brd,
		FileName:         filename,
		Content:          template.HTML(string(ar.ContentHtml)),
		ContentTail:      template.HTML(string(ar.ContentTailHtml)),
		ContentTruncated: ar.IsTruncated,
		PollUrl:          pollUrl,
		LongPollUrl:      longPollUrl,
		CurrOffset:       ar.NextOffset,
	})
}

// oldFilename returns the old filename of an article if any.  Older articles
// have no random suffix. This will result into ".000" suffix when converted
// from AID.
func oldFilename(filename string) (string, bool) {
	if !strings.HasSuffix(filename, ".000") {
		return "", false
	}
	return filename[:len(filename)-4], true
}

func verifySignature(brdname, filename string, size int64, sig string) bool {
	return (&pushstream.PushNotification{
		Brdname:   brdname,
		Filename:  filename,
		Size:      size,
		Signature: sig,
	}).CheckSignature(config.PushStreamSharedSecret)
}

func handleArticlePoll(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]
	filename := vars["filename"]
	cacheKey := c.R.FormValue("cacheKey")
	offset, err := strconv.Atoi(c.R.FormValue("offset"))
	if err != nil {
		return err
	}
	size, err := strconv.Atoi(c.R.FormValue("size"))
	if err != nil {
		return err
	}

	if !verifySignature(brdname, filename, int64(offset), c.R.FormValue("offset-sig")) ||
		!verifySignature(brdname, filename, int64(size), c.R.FormValue("size-sig")) {
		return ErrSigMismatch
	}

	brd, err := getBoardByName(c, brdname)
	if err != nil {
		return err
	}

	obj, err := cacheMgr.Get(&ArticlePartRequest{
		Brd:      *brd,
		Filename: filename,
		CacheKey: cacheKey,
		Offset:   offset,
	}, ZeroArticlePart, time.Minute, generateArticlePart)
	if err != nil {
		return err
	}
	ap := obj.(*ArticlePart)

	res := new(page.ArticlePollResp)
	res.Success = ap.IsValid
	if ap.IsValid {
		res.ContentHtml = ap.ContentHtml
		res.PollUrl, _, err = uriForPolling(brdname, filename, ap.CacheKey, ap.NextOffset)
		if err != nil {
			return err
		}
	}
	return page.WriteAjaxResp(w, res)
}

func uriForPolling(brdname, filename, cacheKey string, offset int) (poll, longPoll string, err error) {
	if !config.EnablePushStream {
		return
	}

	pn := pushstream.PushNotification{
		Brdname:  brdname,
		Filename: filename,
		Size:     int64(offset),
	}
	pn.Sign(config.PushStreamSharedSecret)

	next, err := router.Get("bbsarticlepoll").URLPath("brdname", brdname, "filename", filename)
	if err != nil {
		return
	}
	args := make(url.Values)
	args.Set("cacheKey", cacheKey)
	args.Set("offset", strconv.FormatInt(pn.Size, 10))
	args.Set("offset-sig", pn.Signature)
	poll = next.String() + "?" + args.Encode()

	lpArgs := make(url.Values)
	lpArgs.Set("id", pushstream.GetPushChannel(&pn, config.PushStreamSharedSecret))
	longPoll = config.PushStreamSubscribeLocation + "?" + lpArgs.Encode()
	return
}

func getBoardByName(c *Context, brdname string) (*pttbbs.Board, error) {
	if !pttbbs.IsValidBrdName(brdname) {
		return nil, NewNotFoundError(fmt.Errorf("invalid board name: %s", brdname))
	}

	brd, err := getBoardByNameCached(brdname)
	if err != nil {
		return nil, err
	}

	err = hasPermViewBoard(c, brd)
	if err != nil {
		return nil, err
	}

	return brd, nil
}

func hasPermViewBoard(c *Context, brd *pttbbs.Board) error {
	if !pttbbs.IsValidBrdName(brd.BrdName) || brd.Hidden {
		return NewNotFoundError(fmt.Errorf("no permission: %s", brd.BrdName))
	}
	if brd.Over18 {
		if !config.EnableOver18Cookie {
			return NewNotFoundError(ErrOver18CookieNotEnabled)
		}
		if !c.IsCrawler() && !c.IsOver18() {
			return shouldAskOver18Error(c)
		}
		// Ok
	}
	return nil
}

func shouldAskOver18Error(c *Context) error {
	q := make(url.Values)
	q.Set("from", c.R.URL.String())

	u, _ := router.Get("askover18").URLPath()

	err := new(ShouldAskOver18Error)
	err.To = u.String() + "?" + q.Encode()
	return err
}

func clarifyRemoteError(err error) error {
	if err == pttbbs.ErrNotFound {
		return NewNotFoundError(err)
	}
	return translateGrpcError(err)
}

func translateGrpcError(err error) error {
	switch grpc.Code(err) {
	case codes.NotFound, codes.PermissionDenied:
		return NewNotFoundError(err)
	}
	return err
}

func handleMan(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	fullpath := strings.Split(vars["fullpath"], "/")
	if len(fullpath) < 2 {
		return NewNotFoundError(fmt.Errorf("invalid path: %v", fullpath))
	}
	brdname := fullpath[0]

	brd, err := getBoardByName(c, brdname)
	if err != nil {
		return err
	}

	if fullpath[len(fullpath)-1] == "index" {
		return handleManIndex(c, w, brd, strings.Join(fullpath[1:len(fullpath)-1], "/"))
	}
	return handleManArticle(c, w, brd, strings.Join(fullpath[1:], "/"))
}

func handleManIndex(c *Context, w http.ResponseWriter, brd *pttbbs.Board, path string) error {
	res, err := mand.List(context.TODO(), &manpb.ListRequest{
		BoardName: brd.BrdName,
		Path:      path,
	}, grpc.FailFast(true))
	if err != nil {
		return err
	}
	return page.ExecutePage(w, &page.ManIndex{
		Board:   *brd,
		Path:    path,
		Entries: res.Entries,
	})
}

func handleManArticle(c *Context, w http.ResponseWriter, brd *pttbbs.Board, path string) error {
	obj, err := cacheMgr.Get(&ArticleRequest{
		Namespace: "man",
		Brd:       *brd,
		Filename:  path,
		Select: func(m pttbbs.SelectMethod, offset, maxlen int) (*pttbbs.ArticlePart, error) {
			res, err := mand.Article(context.TODO(), &manpb.ArticleRequest{
				BoardName:  brd.BrdName,
				Path:       path,
				SelectType: manSelectType(m),
				Offset:     int64(offset),
				MaxLength:  int64(maxlen),
			}, grpc.FailFast(true))
			if err != nil {
				return nil, err
			}
			return &pttbbs.ArticlePart{
				CacheKey: res.CacheKey,
				FileSize: int(res.FileSize),
				Offset:   int(res.SelectedOffset),
				Length:   int(res.SelectedSize),
				Content:  res.Content,
			}, nil
		},
	}, ZeroArticle, ArticleCacheTimeout, generateArticle)
	if err != nil {
		return err
	}
	ar := obj.(*Article)

	if !ar.IsValid {
		return NewNotFoundError(nil)
	}

	if len(ar.ContentHtml) > TruncateSize {
		log.Println("Large rendered article:", brd.BrdName, path, len(ar.ContentHtml))
	}

	return page.ExecutePage(w, &page.ManArticle{
		Title:            ar.ParsedTitle,
		Description:      ar.PreviewContent,
		Board:            brd,
		Path:             path,
		Content:          template.HTML(string(ar.ContentHtml)),
		ContentTail:      template.HTML(string(ar.ContentTailHtml)),
		ContentTruncated: ar.IsTruncated,
	})
}

func manSelectType(m pttbbs.SelectMethod) manpb.ArticleRequest_SelectType {
	switch m {
	case pttbbs.SelectHead:
		return manpb.ArticleRequest_SELECT_HEAD
	case pttbbs.SelectTail:
		return manpb.ArticleRequest_SELECT_TAIL
	case pttbbs.SelectPart:
		return manpb.ArticleRequest_SELECT_FULL
	default:
		panic("unknown select type")
	}
}

func fastStrHash64(s string) uint64 {
	h := fnv.New64()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
