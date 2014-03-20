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

	"pttbbs"
	"pttweb/cache"

	"github.com/gorilla/mux"
)

const (
	ArticleCacheTimeout  = time.Minute * 10
	BbsIndexCacheTimeout = time.Minute * 5
)

var (
	ErrOver18CookieNotEnabled = errors.New("board is over18 but cookie not enabled")
)

var ptt pttbbs.Pttbbs
var router *mux.Router
var tmpl TemplateMap
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

	return nil
}

func main() {
	flag.Parse()

	if err := loadConfig(); err != nil {
		log.Println("loadConfig() error:", err)
		return
	}

	// Init RemotePtt
	if config.BoarddAddress == "" {
		log.Println("boardd address not specified")
		return
	}
	ptt = pttbbs.NewRemotePtt(config.BoarddAddress)

	// Init cache manager
	if config.MemcachedAddress == "" {
		log.Println("memcached address not specified")
		return
	}
	cacheMgr = cache.NewCacheManager(config.MemcachedAddress)

	// Load templates
	if t, err := loadTemplates(config.TemplateDirectory, templateFiles); err != nil {
		log.Println("cannot load templates:", err)
		return
	} else {
		tmpl = t
	}

	// Init router
	router = createRouter()
	http.Handle("/", router)

	if len(config.Bind) == 0 {
		log.Fatal("No bind addresses specified in config")
		os.Exit(1)
	}
	for _, addr := range config.Bind {
		part := strings.SplitN(addr, ":", 2)
		if len(part) != 2 {
			log.Fatal("Invalid bind address: ", addr)
			os.Exit(1)
		}
		if listener, err := net.Listen(part[0], part[1]); err != nil {
			log.Fatal("Listen failed for address: ", addr, " error: ", err)
			os.Exit(1)
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
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}{x:/?}`, errorWrapperHandler(handleBbsIndexRedirect))
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index.html`, errorWrapperHandler(handleBbs)).Name("bbsindex")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index{page:\d+}.html`, errorWrapperHandler(handleBbs)).Name("bbsindex_page")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/{filename:[MG]\.\d+\.A(\.[0-9A-F]+)?}.html`, errorWrapperHandler(handleArticle)).Name("bbsarticle")
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
			return !strings.HasPrefix(a.FileName, ".")
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
	}
}

func setCommonResponseHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Server", "Cryophoenix")
	h.Set("Content-Type", "text/html; charset=utf-8")
}

type ErrorPageCapable interface {
	EmitErrorPage(w http.ResponseWriter, r *http.Request) error
}

func errorWrapperHandler(f func(*Context, http.ResponseWriter) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		setCommonResponseHeaders(w)

		if err := handleRequest(w, r, f); err != nil {
			if errpage, ok := err.(ErrorPageCapable); ok {
				if err = errpage.EmitErrorPage(w, r); err != nil {
					log.Println("Failed to emit error page:", err)
				} else {
					log.Println("Emit error page for:", errpage)
				}
				return
			}
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
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

func handleNotFound(c *Context, w http.ResponseWriter) error {
	return NewNotFoundErrorPage(nil)
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
		return tmpl["askover18.html"].Execute(w, map[string]interface{}{
			"From": from,
		})
	}
}

func handleCls(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	bid, err := strconv.Atoi(vars["bid"])
	if err != nil {
		return err
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

	return tmpl["classlist.html"].Execute(w, map[string]interface{}{
		"Boards": boards,
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
	page := 0

	if pg, err := strconv.Atoi(vars["page"]); err == nil {
		page = pg
	}

	var err error

	brd, err := getBoardByName(c, brdname)
	if err != nil {
		return err
	}

	obj, err := cacheMgr.Get(&BbsIndexRequest{
		Brd:  *brd,
		Page: page,
	}, ZeroBbsIndex, BbsIndexCacheTimeout, generateBbsIndex)
	if err != nil {
		return err
	}
	bbsindex := obj.(*BbsIndex)

	if !bbsindex.IsValid {
		return NewNotFoundErrorPage(fmt.Errorf("not a valid cache.BbsIndex: %v/%v", brd.BrdName, page))
	}

	return tmpl["bbsindex.html"].Execute(w, &bbsindex)
}

func handleArticle(c *Context, w http.ResponseWriter) error {
	vars := mux.Vars(c.R)
	brdname := vars["brdname"]
	filename := vars["filename"]

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
	if err != nil {
		return err
	}
	ar := obj.(*Article)

	if !ar.IsValid {
		return handleNotFound(c, w)
	}

	return tmpl["bbsarticle.html"].Execute(w, map[string]interface{}{
		"Title":       ar.ParsedTitle,
		"Description": ar.PreviewContent,
		"Board":       brd,
		"ContentHtml": string(ar.ContentHtml),
	})
}

func getBoardByName(c *Context, brdname string) (*pttbbs.Board, error) {
	bid, err := ptt.BrdName2Bid(brdname)
	if err != nil {
		return nil, NewNotFoundErrorPage(err)
	}

	brd, err := ptt.GetBoard(bid)
	if err != nil {
		return nil, err
	}

	err = hasPermViewBoard(c, brd)
	if err != nil {
		return nil, err
	}

	return &brd, nil
}

func hasPermViewBoard(c *Context, brd pttbbs.Board) error {
	if !pttbbs.IsValidBrdName(brd.BrdName) || brd.Hidden {
		return NewNotFoundErrorPage(fmt.Errorf("no permission: %s", brd.BrdName))
	}
	if brd.Over18 {
		if config.EnableOver18Cookie {
			if c.IsCrawler() {
				// Crawlers don't have age
			} else if !c.IsOver18() {
				return errorRedirectAskOver18(c)
			}
		} else {
			return NewNotFoundErrorPage(ErrOver18CookieNotEnabled)
		}
	}
	return nil
}

func errorRedirectAskOver18(c *Context) error {
	q := make(url.Values)
	q.Set("from", c.R.URL.String())

	u, _ := router.Get("askover18").URLPath()

	return &RedirectErrorPage{
		To: u.String() + "?" + q.Encode(),
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
