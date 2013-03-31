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
	"path/filepath"
	"pttbbs"
	"pttweb/article"
	"runtime/pprof"
	"strconv"
	"strings"
	"text/template"
)

var ptt pttbbs.Pttbbs
var router *mux.Router
var tmpl *template.Template

var bindAddress string
var boarddAddress string
var templateDir string
var cpuProfile string
var memProfile string

func init() {
	flag.StringVar(&bindAddress, "bind", "127.0.0.1:8891", "bind address of the server (host:port)")
	flag.StringVar(&boarddAddress, "boardd", "", "boardd address (host:port)")
	flag.StringVar(&templateDir, "tmpldir", "templates", "template directory, loads all *.html")
	flag.StringVar(&cpuProfile, "cpuprofile", "", "write cpu profile to file")
	flag.StringVar(&memProfile, "memprofile", "", "write memory profile to this file")
}

func main() {
	flag.Parse()

	// CPU Profiling
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Init RemotePtt
	if boarddAddress == "" {
		panic("boardd address not specified")
	}
	ptt = pttbbs.NewRemotePtt(boarddAddress)

	// Load templates
	if t, err := loadTemplates(templateDir); err != nil {
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

	if memProfile != "" {
		f, err := os.Create(memProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
	}
}

func createRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/cls/{bid:[0-9]+}", errorWrapperHandler(handleCls)).Name("classlist")
	router.HandleFunc("/bbs/{brdname:[0-9a-zA-Z_\\-]+}{x:/?}", errorWrapperHandler(handleBbsIndexRedirect))
	router.HandleFunc("/bbs/{brdname:[0-9a-zA-Z_\\-]+}/index.html", errorWrapperHandler(handleBbs)).Name("bbsindex")
	router.HandleFunc("/bbs/{brdname:[0-9a-zA-Z_\\-]+}/index{page:\\d+}.html", errorWrapperHandler(handleBbs)).Name("bbsindex_page")
	router.HandleFunc("/bbs/{brdname:[0-9a-zA-Z_\\-]+}/{filename:[MG]\\.\\d+(\\.A\\.[0-9A-F]+)?}.html", errorWrapperHandler(handleArticle)).Name("bbsarticle")
	return router
}

func loadTemplates(dir string) (*template.Template, error) {
	t := template.New("root").Funcs(template.FuncMap{
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
	})
	return t.ParseGlob(filepath.Join(dir, "*.html"))
}

func errorWrapperHandler(f func(http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
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

	w.WriteHeader(http.StatusOK)
	return tmpl.ExecuteTemplate(w, "classlist.html", map[string]interface{}{
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

	w.WriteHeader(http.StatusOK)
	return tmpl.ExecuteTemplate(w, "bbsindex.html", params)
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

	content, err := ptt.GetArticleContent(bid, filename)
	if err != nil {
		return err
	}

	// Render content
	ar := article.NewRenderer()
	buf, err := ar.Render(content)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	return tmpl.ExecuteTemplate(w, "bbsarticle.html", map[string]interface{}{
		"Title":       ar.ParsedTitle(),
		"Description": ar.PreviewContent(),
		"Board":       brd,
		"ContentHtml": buf.String(),
	})
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
