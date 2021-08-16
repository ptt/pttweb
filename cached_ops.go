package main

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"

	"github.com/ptt/pttweb/article"
	"github.com/ptt/pttweb/atomfeed"
	"github.com/ptt/pttweb/cache"
	"github.com/ptt/pttweb/extcache"
	"github.com/ptt/pttweb/pttbbs"

	"golang.org/x/net/context"
)

const (
	EntryPerPage = 20

	CtxKeyBoardname = `ContextBoardname`
)

type BbsIndexRequest struct {
	Brd  pttbbs.Board
	Page int
}

func (r *BbsIndexRequest) String() string {
	return fmt.Sprintf("pttweb:bbsindex/%v/%v", r.Brd.BrdName, r.Page)
}

func generateBbsIndex(key cache.Key) (cache.Cacheable, error) {
	r := key.(*BbsIndexRequest)
	page := r.Page

	bbsindex := &BbsIndex{
		Board:   r.Brd,
		IsValid: true,
	}

	// Handle paging
	paging := NewPaging(EntryPerPage, r.Brd.NumPosts)
	if page == 0 {
		page = paging.LastPageNo()
		paging.SetPageNo(page)
	} else if err := paging.SetPageNo(page); err != nil {
		return nil, NewNotFoundError(err)
	}

	// Fetch article list
	var err error
	bbsindex.Articles, err = ptt.GetArticleList(r.Brd.Ref(), paging.Cursor(), EntryPerPage)
	if err != nil {
		return nil, err
	}

	// Fetch bottoms when at last page
	if page == paging.LastPageNo() {
		bbsindex.Bottoms, err = ptt.GetBottomList(r.Brd.Ref())
		if err != nil {
			return nil, err
		}
	}

	// Page links
	if u, err := router.Get("bbsindex").URLPath("brdname", r.Brd.BrdName); err == nil {
		bbsindex.LastPage = u.String()
	}
	pageLink := func(n int) string {
		u, err := router.Get("bbsindex_page").URLPath("brdname", r.Brd.BrdName, "page", strconv.Itoa(n))
		if err != nil {
			return ""
		}
		return u.String()
	}
	bbsindex.FirstPage = pageLink(1)
	if page > 1 {
		bbsindex.PrevPage = pageLink(page - 1)
	}
	if page < paging.LastPageNo() {
		bbsindex.NextPage = pageLink(page + 1)
	}

	return bbsindex, nil
}

type BbsSearchRequest struct {
	Brd   pttbbs.Board
	Page  int
	Query string
	Preds []pttbbs.SearchPredicate
}

func (r *BbsSearchRequest) String() string {
	queryHash := sha256.Sum256([]byte(r.Query))
	query := base64.URLEncoding.EncodeToString(queryHash[:])
	return fmt.Sprintf("pttweb:bbssearch/%v/%v/%v", r.Brd.BrdName, r.Page, query)
}

func generateBbsSearch(key cache.Key) (cache.Cacheable, error) {
	r := key.(*BbsSearchRequest)
	page := r.Page
	if page == 0 {
		page = 1
	}
	offset := -EntryPerPage * page

	bbsindex := &BbsIndex{
		Board:   r.Brd,
		Query:   r.Query,
		IsValid: true,
	}

	// Search articles
	articles, totalPosts, err := pttSearch.Search(r.Brd.Ref(), r.Preds, offset, EntryPerPage)
	if err != nil {
		return nil, err
	}

	// Handle paging
	paging := NewPaging(EntryPerPage, totalPosts)
	if lastPage := paging.LastPageNo(); page > lastPage {
		articles = nil
		bbsindex.IsValid = false
	} else if page == lastPage {
		// We may get extra entries for last page.
		n := totalPosts % EntryPerPage
		if n < len(articles) {
			articles = articles[:n]
		}
	}

	// Show the page in reverse order.
	for i, j := 0, len(articles)-1; i < j; i, j = i+1, j-1 {
		articles[i], articles[j] = articles[j], articles[i]
	}
	bbsindex.Articles = articles

	// Page links, in newest first order.
	pageLink := func(n int) string {
		u, err := router.Get("bbssearch").URLPath("brdname", r.Brd.BrdName)
		if err != nil {
			return ""
		}
		q := url.Values{}
		q.Set("q", r.Query)
		q.Set("page", strconv.Itoa(n))
		u.RawQuery = q.Encode()
		return u.String()
	}
	bbsindex.FirstPage = pageLink(paging.LastPageNo())
	bbsindex.LastPage = pageLink(1)
	if page > 1 {
		bbsindex.NextPage = pageLink(page - 1)
	}
	if page < paging.LastPageNo() {
		bbsindex.PrevPage = pageLink(page + 1)
	}

	return bbsindex, nil
}

type BoardAtomFeedRequest struct {
	Brd pttbbs.Board
}

func (r *BoardAtomFeedRequest) String() string {
	return fmt.Sprintf("pttweb:atomfeed/%v", r.Brd.BrdName)
}

func generateBoardAtomFeed(key cache.Key) (cache.Cacheable, error) {
	r := key.(*BoardAtomFeedRequest)

	if atomConverter == nil {
		return nil, errors.New("atom feed not configured")
	}

	// Fetch article list
	articles, err := ptt.GetArticleList(r.Brd.Ref(), -EntryPerPage, EntryPerPage)
	if err != nil {
		return nil, err
	}
	// Fetch snippets and contruct posts.
	var posts []*atomfeed.PostEntry
	for _, article := range articles {
		// Use an empty string when error.
		snippet, _ := getArticleSnippet(r.Brd, article.FileName)
		posts = append(posts, &atomfeed.PostEntry{
			Article: article,
			Snippet: snippet,
		})
	}

	feed, err := atomConverter.Convert(r.Brd, posts)
	if err != nil {
		log.Println("atomfeed: Convert:", err)
		// Don't return error but cache that it's invalid.
	}
	return &BoardAtomFeed{
		Feed:    feed,
		IsValid: err == nil,
	}, nil
}

const SnippetHeadSize = 16 * 1024 // Enough for 8 pages of 80x24.

func getArticleSnippet(brd pttbbs.Board, filename string) (string, error) {
	p, err := ptt.GetArticleSelect(brd.Ref(), pttbbs.SelectHead, filename, "", 0, SnippetHeadSize)
	if err != nil {
		return "", err
	}
	if len(p.Content) == 0 {
		return "", pttbbs.ErrNotFound
	}

	ra, err := article.Render(article.WithContent(p.Content))
	if err != nil {
		return "", err
	}
	return ra.PreviewContent(), nil
}

const (
	TruncateSize    = 1048576
	TruncateMaxScan = 1024

	HeadSize = 100 * 1024
	TailSize = 50 * 1024
)

type ArticleRequest struct {
	Namespace string
	Brd       pttbbs.Board
	Filename  string
	Select    func(m pttbbs.SelectMethod, offset, maxlen int) (*pttbbs.ArticlePart, error)
}

func (r *ArticleRequest) String() string {
	return fmt.Sprintf("pttweb:%v/%v/%v", r.Namespace, r.Brd.BrdName, r.Filename)
}

func (r *ArticleRequest) Boardname() string {
	return r.Brd.BrdName
}

func generateArticle(key cache.Key) (cache.Cacheable, error) {
	r := key.(*ArticleRequest)
	ctx := context.TODO()
	ctx = context.WithValue(ctx, CtxKeyBoardname, r)
	if config.Experiments.ExtCache.Enabled(fastStrHash64(r.Filename)) {
		ctx = extcache.WithExtCache(ctx, extCache)
	}

	p, err := r.Select(pttbbs.SelectHead, 0, HeadSize)
	if err != nil {
		return nil, err
	}

	// We don't want head and tail have duplicate content
	if p.FileSize > HeadSize && p.FileSize <= HeadSize+TailSize {
		p, err = r.Select(pttbbs.SelectPart, 0, p.FileSize)
		if err != nil {
			return nil, err
		}
	}

	if len(p.Content) == 0 {
		return nil, pttbbs.ErrNotFound
	}

	a := new(Article)

	a.IsPartial = p.Length < p.FileSize
	a.IsTruncated = a.IsPartial

	if a.IsPartial {
		// Get and render tail
		ptail, err := r.Select(pttbbs.SelectTail, -TailSize, TailSize)
		if err != nil {
			return nil, err
		}
		if len(ptail.Content) > 0 {
			ra, err := article.Render(
				article.WithContent(ptail.Content),
				article.WithContext(ctx),
				article.WithDisableArticleHeader(),
			)
			if err != nil {
				return nil, err
			}
			a.ContentTailHtml = ra.HTML()
		}
		a.CacheKey = ptail.CacheKey
		a.NextOffset = ptail.FileSize - TailSize + ptail.Offset + ptail.Length
	} else {
		a.CacheKey = p.CacheKey
		a.NextOffset = p.Length
	}

	ra, err := article.Render(
		article.WithContent(p.Content),
		article.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	a.ParsedTitle = ra.ParsedTitle()
	a.PreviewContent = ra.PreviewContent()
	a.ContentHtml = ra.HTML()
	a.IsValid = true
	return a, nil
}

type ArticlePartRequest struct {
	Brd      pttbbs.Board
	Filename string
	CacheKey string
	Offset   int
}

func (r *ArticlePartRequest) String() string {
	return fmt.Sprintf("pttweb:bbs/%v/%v#%v,%v", r.Brd.BrdName, r.Filename, r.CacheKey, r.Offset)
}

func (r *ArticlePartRequest) Boardname() string {
	return r.Brd.BrdName
}

func generateArticlePart(key cache.Key) (cache.Cacheable, error) {
	r := key.(*ArticlePartRequest)
	ctx := context.TODO()
	ctx = context.WithValue(ctx, CtxKeyBoardname, r)
	if config.Experiments.ExtCache.Enabled(fastStrHash64(r.Filename)) {
		ctx = extcache.WithExtCache(ctx, extCache)
	}

	p, err := ptt.GetArticleSelect(r.Brd.Ref(), pttbbs.SelectHead, r.Filename, r.CacheKey, r.Offset, -1)
	if err == pttbbs.ErrNotFound {
		// Returns an invalid result
		return new(ArticlePart), nil
	}
	if err != nil {
		return nil, err
	}

	ap := new(ArticlePart)
	ap.IsValid = true
	ap.CacheKey = p.CacheKey
	ap.NextOffset = r.Offset + p.Offset + p.Length

	if len(p.Content) > 0 {
		ra, err := article.Render(
			article.WithContent(p.Content),
			article.WithContext(ctx),
			article.WithDisableArticleHeader(),
		)
		if err != nil {
			return nil, err
		}
		ap.ContentHtml = string(ra.HTML())
	}

	return ap, nil
}

func truncateLargeContent(content []byte, size, maxScan int) []byte {
	if len(content) <= size {
		return content
	}
	for i := size - 1; i >= size-maxScan && i >= 0; i-- {
		if content[i] == '\n' {
			return content[:i+1]
		}
	}
	return content[:size]
}
