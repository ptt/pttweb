package main

import (
	"fmt"
	"log"

	"github.com/ptt/pttweb/article"
	"github.com/ptt/pttweb/cache"
	"github.com/ptt/pttweb/pttbbs"
)

const (
	EntryPerPage = 20
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

	count, err := ptt.GetArticleCount(r.Brd.Bid)
	if err != nil {
		return nil, err
	}

	// Handle paging
	paging := NewPaging(EntryPerPage, count)
	if page == 0 {
		page = paging.LastPageNo()
		paging.SetPageNo(page)
	} else if err = paging.SetPageNo(page); err != nil {
		return nil, err
	}
	bbsindex.TotalPage = paging.LastPageNo()

	// Fetch article list
	bbsindex.Articles, err = ptt.GetArticleList(r.Brd.Bid, paging.Cursor())
	if err != nil {
		return nil, err
	}

	// Page links
	if page > 1 {
		bbsindex.HasPrevPage = true
		bbsindex.PrevPage = page - 1
	}
	if page < paging.LastPageNo() {
		bbsindex.HasNextPage = true
		bbsindex.NextPage = page + 1
	}

	return bbsindex, nil
}

const (
	TruncateSize    = 1048576
	TruncateMaxScan = 1024
)

type ArticleRequest struct {
	Brd      pttbbs.Board
	Filename string
}

func (r *ArticleRequest) String() string {
	return fmt.Sprintf("pttweb:bbs/%v/%v", r.Brd.BrdName, r.Filename)
}

func generateArticle(key cache.Key) (cache.Cacheable, error) {
	r := key.(*ArticleRequest)

	content, err := ptt.GetArticleContent(r.Brd.Bid, r.Filename)
	if err != nil {
		return nil, err
	} else if content == nil {
		return nil, NewNotFoundErrorPage(fmt.Errorf("no content: %v/%v", r.Brd.BrdName, r.Filename))
	}

	a := new(Article)

	if len(content) > TruncateSize {
		log.Println("Large file:", key, len(content))
		content = truncateLargeContent(content, TruncateSize, TruncateMaxScan)
		a.IsTruncated = true
	}

	ar := article.NewRenderer()
	buf, err := ar.Render(content)
	if err != nil {
		return nil, err
	}
	a.ParsedTitle = ar.ParsedTitle()
	a.PreviewContent = ar.PreviewContent()
	a.ContentHtml = buf.Bytes()
	a.IsValid = true
	return a, nil
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
