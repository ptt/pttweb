package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/ptt/pttweb/cache"
	"github.com/ptt/pttweb/page"
	"github.com/ptt/pttweb/pttbbs"

	"github.com/gorilla/mux"
)

const (
	ArticleCacheTimeout          = time.Minute * 10
	BbsIndexCacheTimeout         = time.Minute * 5
	BbsIndexLastPageCacheTimeout = time.Minute * 1
)

var (
	ErrOver18CookieNotEnabled = errors.New("board is over18 but cookie not enabled")
)

var ptt pttbbs.Pttbbs
var router *mux.Router
var cacheMgr *cache.CacheManager

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
	ptt = pttbbs.NewRemotePtt(config.BoarddAddress, config.BoarddMaxConn)

	// Init cache manager
	cacheMgr = cache.NewCacheManager(config.MemcachedAddress, config.MemcachedMaxConn)

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
	router.HandleFunc(`/bbs/`, errorWrapperHandler(handleClsRoot))
	router.HandleFunc(`/bbs/index.html`, errorWrapperHandler(handleClsRoot))
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}{x:/?}`, errorWrapperHandler(handleBbsIndexRedirect))
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index.html`, errorWrapperHandler(handleBbs)).Name("bbsindex")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index{page:\d+}.html`, errorWrapperHandler(handleBbs)).Name("bbsindex_page")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/{filename:[MG]\.\d+\.A(\.[0-9A-F]+)?}.html`, errorWrapperHandler(handleArticle)).Name("bbsarticle")
	router.HandleFunc(`/b/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/{aidc:[0-9A-Za-z\-_]+}`, errorWrapperHandler(handleAidc)).Name("bbsaidc")
	router.HandleFunc(`/ask/over18`, errorWrapperHandler(handleAskOver18)).Name("askover18")
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
		"route": func(where string, attrs ...string) (*url.URL, error) {
			return router.Get(where).URLPath(attrs...)
		},
		"static_prefix": func() string {
			return config.StaticPrefix
		},
		"colored_counter": colored_counter,
		"post_mark":       post_mark,
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

	boards := make([]pttbbs.Board, 0, 16)
	for _, bid := range children {
		if brd, err := ptt.GetBoard(bid); err == nil {
			if pttbbs.IsValidBrdName(brd.BrdName) && !brd.Over18 && !brd.Hidden {
				boards = append(boards, brd)
			}
		}
	}

	return page.ExecutePage(w, &page.Classlist{
		Boards: boards,
	})
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
		Brd:      *brd,
		Filename: filename,
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

	return page.ExecutePage(w, &page.BbsArticle{
		Title:            ar.ParsedTitle,
		Description:      ar.PreviewContent,
		Board:            brd,
		FileName:         filename,
		ContentHtml:      string(ar.ContentHtml),
		ContentTailHtml:  string(ar.ContentTailHtml),
		ContentTruncated: ar.IsTruncated,
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

func getBoardByName(c *Context, brdname string) (*pttbbs.Board, error) {
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
	return err
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
