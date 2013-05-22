package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"pttbbs"
	"pttweb/article"
	"pttweb/cache"
	"runtime/pprof"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var (
	ArticleCacheTimeout = time.Minute * 10
)

var ptt pttbbs.Pttbbs
var router *mux.Router
var tmpl TemplateMap
var cacheMgr *cache.CacheManager

var bindAddress string
var boarddAddress string
var memcachedAddress string
var templateDir string
var cpuProfile string
var memProfile string

func init() {
	flag.StringVar(&bindAddress, "bind", "127.0.0.1:8891", "bind address of the server (host:port)")
	flag.StringVar(&boarddAddress, "boardd", "", "boardd address (host:port)")
	flag.StringVar(&memcachedAddress, "memcached", "", "memcached address (host:port)")
	flag.StringVar(&templateDir, "tmpldir", "templates", "template directory, loads all *.html")
}

func main() {
	flag.Parse()

	// Init RemotePtt
	if boarddAddress == "" {
		panic("boardd address not specified")
	}
	ptt = pttbbs.NewRemotePtt(boarddAddress)

	// Init cache manager
	if memcachedAddress == "" {
		panic("memcached address not specified")
	}
	cacheMgr = cache.NewCacheManager(memcachedAddress)

	// Load templates
	if t, err := loadTemplates(templateDir, templateFiles); err != nil {
		panic(err)
	} else {
		tmpl = t
	}

	// Init router
	router = createRouter()
	http.Handle("/", router)

	go func() {
		if err := http.ListenAndServe(bindAddress, nil); err != nil {
			log.Fatal("ListenAndServe: ", err)
			os.Exit(1)
		}
	}()

	progExit := make(chan os.Signal)
	signal.Notify(progExit, os.Interrupt)
	<-progExit
}

func createRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(`/cls/{bid:[0-9]+}`, errorWrapperHandler(handleCls)).Name("classlist")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}{x:/?}`, errorWrapperHandler(handleBbsIndexRedirect))
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index.html`, errorWrapperHandler(handleBbs)).Name("bbsindex")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/index{page:\d+}.html`, errorWrapperHandler(handleBbs)).Name("bbsindex_page")
	router.HandleFunc(`/bbs/{brdname:[A-Za-z][0-9a-zA-Z_\.\-]+}/{filename:[MG]\.\d+(\.A\.[0-9A-F]+)?}.html`, errorWrapperHandler(handleArticle)).Name("bbsarticle")
	return router
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"route_bbsindex": func(b pttbbs.Board) (*url.URL, error) {
			return router.Get("bbsindex").URLPath("brdname", b.BrdName)
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
		"route": func(where string, attrs ...string) (*url.URL, error) {
			return router.Get(where).URLPath(attrs...)
		},
		"colored_counter": colored_counter,
		"post_mark":       post_mark,
	}
}

func setCommonResponseHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Server", "Cryophoenix")
	h.Set("Content-Type", "text/html; charset=utf-8")
}

func errorWrapperHandler(f func(http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		setCommonResponseHeaders(w)
		if err := f(w, r); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func handleNotFound(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusNotFound)
	return tmpl["notfound.html"].Execute(w, nil)
}

func handleCls(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
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

func handleBbsIndexRedirect(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	if url, err := router.Get("bbsindex").URLPath("brdname", vars["brdname"]); err != nil {
		return err
	} else {
		w.Header().Set("Location", url.String())
	}
	w.WriteHeader(http.StatusFound)
	return nil
}

func handleBbs(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	brdname := vars["brdname"]
	page := 0

	if pg, err := strconv.Atoi(vars["page"]); err == nil {
		page = pg
	}

	var err error
	params := make(map[string]interface{})

	bid, err := ptt.BrdName2Bid(brdname)
	if err != nil {
		return err
	}

	brd, err := ptt.GetBoard(bid)
	if err != nil {
		return err
	}
	params["Board"] = brd

	err = hasPermViewBoard(brd)
	if err != nil {
		return err
	}

	count, err := ptt.GetArticleCount(bid)
	if err != nil {
		return err
	}

	// Handle paging
	paging := NewPaging(20, count)
	if page == 0 {
		page = paging.LastPageNo()
		paging.SetPageNo(page)
	} else if err = paging.SetPageNo(page); err != nil {
		return err
	}

	// Fetch article list
	params["Articles"], err = ptt.GetArticleList(bid, paging.Cursor())
	if err != nil {
		return err
	}

	// Page links
	pageLink := func(i int) *url.URL {
		if i < 1 || i > paging.LastPageNo() {
			return nil
		}
		if u, err := router.Get("bbsindex_page").URLPath("brdname", brdname, "page", strconv.Itoa(i)); err == nil {
			return u
		}
		return nil
	}
	params["PrevPage"] = pageLink(page - 1)
	params["NextPage"] = pageLink(page + 1)
	params["FirstPage"] = pageLink(1)
	params["LastPage"], err = router.Get("bbsindex").URLPath("brdname", brdname)
	if err != nil {
		return err
	}

	return tmpl["bbsindex.html"].Execute(w, params)
}

func handleArticle(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	brdname := vars["brdname"]
	filename := vars["filename"]

	var err error

	bid, err := ptt.BrdName2Bid(brdname)
	if err != nil {
		return err
	}

	brd, err := ptt.GetBoard(bid)
	if err != nil {
		return err
	}

	err = hasPermViewBoard(brd)
	if err != nil {
		return err
	}

	// Render content
	resultChan := cacheMgr.Get(
		fmt.Sprintf("pttweb:bbs/%s/%s", brd.BrdName, filename),
		func(key string) (res cache.Result) {
			a := generateArticle(bid, filename)
			res.Expire = ArticleCacheTimeout
			res.Output, res.Err = a.EncodeToBytes()
			if res.Err != nil {
				res.Output = nil
			}
			return
		})

	result := <-resultChan
	var ar cache.Article
	if err = cache.GobDecode(result.Output, &ar); err != nil {
		return err
	}

	if !ar.IsValid {
		return handleNotFound(w, r)
	}

	return tmpl["bbsarticle.html"].Execute(w, map[string]interface{}{
		"Title":       ar.ParsedTitle,
		"Description": ar.PreviewContent,
		"Board":       brd,
		"ContentHtml": string(ar.ContentHtml),
	})
}

func generateArticle(bid int, filename string) (a cache.Article) {
	content, err := ptt.GetArticleContent(bid, filename)
	if err != nil || content == nil {
		return
	}

	ar := article.NewRenderer()
	buf, err := ar.Render(content)
	if err == nil {
		a.ParsedTitle = ar.ParsedTitle()
		a.PreviewContent = ar.PreviewContent()
		a.ContentHtml = buf.Bytes()
		a.IsValid = true
	}
	return
}

func hasPermViewBoard(brd pttbbs.Board) error {
	if !pttbbs.IsValidBrdName(brd.BrdName) || brd.Over18 || brd.Hidden {
		return fmt.Errorf("No permission: %s", brd.BrdName)
	}
	return nil
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
