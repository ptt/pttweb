package atomfeed

import (
	"bytes"
	"text/template"
	"time"

	"github.com/ptt/pttweb/pttbbs"
	"golang.org/x/tools/blog/atom"
)

type Converter struct {
	FeedTitleTemplate *template.Template
	LinkFeed          func(brdname string) (string, error)
	LinkArticle       func(brdname, filename string) (string, error)
}

func (c *Converter) Convert(board pttbbs.Board, articles []pttbbs.Article) (*atom.Feed, error) {
	var title bytes.Buffer
	if err := c.FeedTitleTemplate.Execute(&title, board); err != nil {
		return nil, err
	}

	feedURL, err := c.LinkFeed(board.BrdName)
	if err != nil {
		return nil, err
	}

	var entries []*atom.Entry
	// Reverse (time) order.
	for i := len(articles) - 1; i >= 0; i-- {
		entry, err := c.convertArticle(articles[i], board.BrdName)
		if err != nil {
			// Ignore errors.
			continue
		}
		entries = append(entries, entry)
	}

	return &atom.Feed{
		Title: title.String(),
		ID:    feedURL,
		Link: []atom.Link{{
			Rel:  "self",
			Href: feedURL,
		}},
		Updated: atom.Time(firstArticleTimeOrNow(articles)),
		Entry:   entries,
	}, nil
}

func (c *Converter) convertArticle(a pttbbs.Article, brdname string) (*atom.Entry, error) {
	articleURL, err := c.LinkArticle(brdname, a.FileName)
	if err != nil {
		return nil, err
	}
	// Will use a zero time if unable to parse.
	published, _ := pttbbs.ParseFileNameTime(a.FileName)
	return &atom.Entry{
		Author: &atom.Person{
			Name: a.Owner,
		},
		Title: a.Title,
		ID:    articleURL,
		Link: []atom.Link{{
			Rel:  "alternate",
			Type: "text/html",
			Href: articleURL,
		}},
		Published: atom.Time(published),
		Updated:   atom.Time(published), // TODO: support this in boardd.
	}, nil
}

func firstArticleTimeOrNow(articles []pttbbs.Article) time.Time {
	for _, a := range articles {
		if t, err := pttbbs.ParseFileNameTime(a.FileName); err == nil {
			return t
		}
	}
	return time.Now()
}
