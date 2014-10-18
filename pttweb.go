package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
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
	"text/template"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"golang.org/x/net/context"

	"github.com/ptt/pttweb/atomfeed"
	"github.com/ptt/pttweb/cache"
	"github.com/ptt/pttweb/page"
	manpb "github.com/ptt/pttweb/proto/man"
	"github.com/ptt/pttweb/pttbbs"
	"github.com/ptt/pttweb/pushstream"

	"github.com/gorilla/mux"
)

const (
	ArticleCacheTimeout          = time.Minute * 10
	BbsIndexCacheTimeout         = time.Minute * 5
	BbsIndexLastPageCacheTimeout = time.Minute * 1
)

var (
	ErrOver18CookieNotEnabled = errors.New("board is over18 but cookie not enabled")
	ErrSigMismatch            = errors.New("push stream signature mismatch")
)

var ptt pttbbs.Pttbbs
var mand manpb.ManServiceClient
var router *mux.Router
var cacheMgr *cache.CacheManager
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
	flag.Parse()

	if err := loadConfig(); err != nil {
		log.Fatal("loadConfig:", err)
	}

	// Init RemotePtt
	if config.UseGrpcForBoardd {
		var err error
		ptt, err = pttbbs.NewGrpcRemotePtt(config.BoarddAddress)
		if err != nil {
			log.Fatal("cannot connect to boardd:", config.BoarddAddress, err)
		}
	} else {
		ptt = pttbbs.NewRemotePtt(config.BoarddAddress, config.BoarddMaxConn)
	}

	// Init mand connection
	if conn, err := grpc.Dial(config.MandAddress, grpc.WithInsecure(), grpc.WithBackoffMaxDelay(time.Second*5)); err != nil {
		log.Fatal("cannot connect to mand:", config.MandAddress, err)
	} else {
		mand = manpb.NewManServiceClient(conn)
	}

	// Init cache manager
	cacheMgr = cache.NewCacheManager(config.MemcachedAddress, config.MemcachedMaxConn)

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

func createRouter() *mux.Router {
	router := mux.NewRouter()
	router.PathPrefix(`/static/`).Handler(http.StripPrefix(`/static/`, http.FileServer(http.Dir(filepath.Join(config.TemplateDirectory, `static`)))))
	router.HandleFunc(`/cls/{bid:[0-9]+}`, errorWrapperHandler(handleCls)).Name("classlist")
	router.HandleFunc(`/bbs/`, errorWrapperHandler(handleHotboards))
	router.HandleFunc(`/bbs/index.html`, errorWrapperHandler(handleHotboards))
	router.HandleFunc(`/bbs/hotboards.html`, errorWrapperHandler(handleHotboards))
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}{x:/?}`, errorWrapperHandler(handleBbsIndexRedirect))
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index.html`, errorWrapperHandler(handleBbs)).Name("bbsindex")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index{page:\d+}.html`, errorWrapperHandler(handleBbs)).Name("bbsindex_page")
	router.HandleFunc(`/atom/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}.xml`, errorWrapperHandler(handleBoardAtomFeed))
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/{filename:[MG]\.\d+\.A(?:\.[0-9A-F]+)?}.html`, errorWrapperHandler(handleArticle)).Name("bbsarticle")
	router.HandleFunc(`/b/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/{aidc:[0-9A-Za-z\-_]+}`, errorWrapperHandler(handleAidc)).Name("bbsaidc")
	if config.EnablePushStream {
		router.HandleFunc(`/poll/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/{filename:[MG]\.\d+\.A(\.[0-9A-F]+)?}.html`, errorWrapperHandler(handleArticlePoll)).Name("bbsarticlepoll")
	}
	router.HandleFunc(`/ask/over18`, errorWrapperHandler(handleAskOver18)).Name("askover18")
	router.HandleFunc(`/man/{fullpath:[0-9a-zA-Z_\.\-\/]+}.html`, errorWrapperHandler(handleMan)).Name("manentry")
	return router
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
			return router.Get("classlist").URLPath("bid", strconv.Itoa(b.Bid))
		},
		"valid_article": func(a pttbbs.Article) bool {
			return pttbbs.IsValidArticleFileName(a.FileName)
		},
		"route_bbsarticle": func(brdname, filename string) (*url.URL, error) {
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

func errorWrapperHandler(f func(*Context, http.ResponseWriter) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		setCommonResponseHeaders(w)

		if err := clarifyRemoteError(handleRequest(w, r, f)); err != nil {
			if pg, ok := err.(page.Page); ok {
				if err = page.ExecutePage(w, pg); err != nil {
					log.Println("Failed to emit error page:", err)
				}
				return
			}
			internalError(w, err)
		}
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
	return handleClsWithBid(c, w, 1)
}

func handleCls(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	bid, err := strconv.Atoi(vars["bid"])
	if err != nil {
		return err
	}
	return handleClsWithBid(c, w, bid)
}

func handleClsWithBid(c *Context, w http.ResponseWriter, bid int) error {
	if bid < 1 {
		return NewNotFoundError(fmt.Errorf("invalid bid: %v", bid))
	}

	children, err := ptt.GetBoardChildren(bid)
	if err != nil {
		return err
	}
	boards, err := ptt.GetBoards(children)
	if err != nil {
		return err
	}
	return page.ExecutePage(w, &page.Classlist{
		Boards: validBoards(boards),
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

	var err error

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

func handleBoardAtomFeed(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]

	timeout := BbsIndexLastPageCacheTimeout

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
			return ptt.GetArticleSelect(brd.Bid, m, filename, "", offset, maxlen)
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
		ContentHtml:      string(ar.ContentHtml),
		ContentTailHtml:  string(ar.ContentTailHtml),
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
		if config.EnableOver18Cookie {
			if c.IsCrawler() {
				// Crawlers don't have age
			} else if !c.IsOver18() {
				return shouldAskOver18Error(c)
			}
		} else {
			return NewNotFoundError(ErrOver18CookieNotEnabled)
		}
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
		ContentHtml:      string(ar.ContentHtml),
		ContentTailHtml:  string(ar.ContentTailHtml),
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

func boardlist(ptt pttbbs.Pttbbs, indent string, root int, loop map[int]bool) {
	if loop[root] {
		//fmt.Println(indent, "loop skipped")
		return
	}
	loop[root] = true

	children, err := ptt.GetBoardChildren(root)
	if err != nil {
		fmt.Print(err)
		return
	}

	for _, bid := range children {
		if brd, err := ptt.GetBoard(bid); err == nil {
			if !brd.Hidden {
				//fmt.Println(indent, bid, brd)
				if !brd.IsBoard {
					boardlist(ptt, indent+"  ", bid, loop)
				} else {
					fmt.Println(brd.BrdName)
				}
			}
		} else {
			//fmt.Println(indent, bid, err)
		}
	}
}
