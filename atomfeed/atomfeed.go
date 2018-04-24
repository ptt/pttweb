package atomfeed

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/ptt/pttweb/pttbbs"
	"golang.org/x/tools/blog/atom"
)

type PostEntry struct {
	Article pttbbs.Article
	Snippet string
}

type Converter struct {
	FeedTitleTemplate *template.Template
	LinkFeed          func(brdname string) (string, error)
	LinkArticle       func(brdname, filename string) (string, error)
}

func (c *Converter) Convert(board pttbbs.Board, posts []*PostEntry) (*atom.Feed, error) {
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
	for i := len(posts) - 1; i >= 0; i-- {
		entry, err := c.convertArticle(posts[i], board.BrdName)
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
		Updated: atom.Time(firstArticleTimeOrNow(posts)),
		Entry:   entries,
	}, nil
}

func (c *Converter) convertArticle(p *PostEntry, brdname string) (*atom.Entry, error) {
	a := p.Article
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
		Updated:   atom.Time(a.Modified),
		Content: &atom.Text{
			Type: "html",
			Body: fmt.Sprintf("<pre>%v</pre>", p.Snippet),
		},
	}, nil
}

func firstArticleTimeOrNow(posts []*PostEntry) time.Time {
	for _, p := range posts {
		if t, err := pttbbs.ParseFileNameTime(p.Article.FileName); err == nil {
			return t
		}
	}
	return time.Now()
}
