package page

import (
	"net/http"

	bbspb "github.com/ptt/pttweb/proto"
	"github.com/ptt/pttweb/pttbbs"
)

type Page interface {
	TemplateName() string
}

type NoContent struct{}

func (NoContent) TemplateName() string { return "" }

type Redirect struct {
	NoContent
	To string
}

func (p *Redirect) WriteHeaders(w http.ResponseWriter) error {
	w.Header().Set("Location", p.To)
	w.WriteHeader(http.StatusFound)
	return nil
}

func NewRedirect(to string) *Redirect {
	return &Redirect{
		To: to,
	}
}

type NotFound struct{}

func (NotFound) TemplateName() string { return TnameNotFound }

func (p *NotFound) WriteHeaders(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNotFound)
	return nil
}

type Error struct {
	Title       string
	ContentHtml string
}

func (Error) TemplateName() string { return TnameError }

type AskOver18 struct {
	From string
}

func (AskOver18) TemplateName() string { return TnameAskOver18 }

type Classlist struct {
	Boards []pttbbs.Board
}

func (Classlist) TemplateName() string { return TnameClasslist }

type BbsIndex struct {
	Board pttbbs.Board

	HasPrevPage bool
	HasNextPage bool
	PrevPage    int
	NextPage    int
	TotalPage   int

	Articles []pttbbs.Article
	Bottoms  []pttbbs.Article

	IsValid bool
}

func (BbsIndex) TemplateName() string { return TnameBbsIndex }

type BbsArticle struct {
	Title            string
	Description      string
	Board            *pttbbs.Board
	FileName         string
	ContentHtml      string
	ContentTailHtml  string
	ContentTruncated bool
}

func (BbsArticle) TemplateName() string { return TnameBbsArticle }

type ManIndex struct {
	Board   pttbbs.Board
	Path    string
	Entries []*bbspb.Entry
}

func (ManIndex) TemplateName() string { return TnameManIndex }

type ManArticle struct {
	Title            string
	Description      string
	Board            *pttbbs.Board
	Path             string
	ContentHtml      string
	ContentTailHtml  string
	ContentTruncated bool
}

func (ManArticle) TemplateName() string { return TnameManArticle }
