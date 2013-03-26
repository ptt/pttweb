package main

import (
	"ansi"
	"bytes"
	"html"
	"pttbbs"
	"strconv"
	"strings"
)

const (
	lineBufferPrealloc = 160
)

type ArticleRenderer struct {
	buf         bytes.Buffer
	lineBufHtml bytes.Buffer
	lineBuf     bytes.Buffer
	indexArr    []int
	lineNo      int

	title string
}

func NewArticleRenderer() *ArticleRenderer {
	ar := &ArticleRenderer{}
	ar.Reset()
	return ar
}

func (r *ArticleRenderer) Reset() {
	r.buf.Reset()
	r.lineBufHtml.Reset()
	r.lineBuf.Reset()
	r.indexArr = make([]int, lineBufferPrealloc, lineBufferPrealloc)
	r.title = ""
	r.lineNo = 1
}

func (r *ArticleRenderer) ParsedTitle() string {
	return r.title
}

func (r *ArticleRenderer) Render(content []byte) (*bytes.Buffer, error) {
	converter := &ansi.Ansi2Html{
		StartColor: func(fg, bg, flags int) {
			r.startColor(fg, bg, flags)
		},
		EndColor: func() {
			r.endColor()
		},
		Rune: func(ru rune) {
			r.oneRune(ru)
		},
		EndOfLine: func() {
			r.endOfLine()
		},
	}
	if err := converter.ConvertFromUTF8(content); err != nil {
		return nil, err
	}
	return &r.buf, nil
}

func (r *ArticleRenderer) startColor(fg, bg, flags int) {
	classes := make([]string, 0, 3)
	if fg != 7 {
		classes = append(classes, `f`+strconv.Itoa(fg))
	}
	if bg != 0 {
		classes = append(classes, `b`+strconv.Itoa(bg))
	}
	if flags&ansi.Highlighted == ansi.Highlighted {
		classes = append(classes, `hl`)
	}
	r.lineBufHtml.WriteString(`<span class="` + strings.Join(classes, ` `) + `">`)
}

func (r *ArticleRenderer) endColor() {
	r.lineBufHtml.WriteString("</span>")
}

func fastWriteHtmlEscapedRune(buf *bytes.Buffer, ru rune) {
	if ru >= 256 {
		buf.WriteRune(ru)
		return
	}
	switch ru {
	case '&':
		buf.WriteString(`&amp;`)
	case '\'':
		buf.WriteString(`&#39;`)
	case '<':
		buf.WriteString(`&lt;`)
	case '>':
		buf.WriteString(`&gt;`)
	case '"':
		buf.WriteString(`&#34;`)
	default:
		buf.WriteRune(ru)
	}
}

func (r *ArticleRenderer) oneRune(ru rune) {
	at := r.lineBuf.Len()
	if at >= cap(r.indexArr) {
		newArr := make([]int, 2*cap(r.indexArr), 2*cap(r.indexArr))
		copy(newArr, r.indexArr)
		r.indexArr = newArr
	}
	r.indexArr[at] = r.lineBufHtml.Len()
	fastWriteHtmlEscapedRune(&r.lineBufHtml, ru)
	r.lineBuf.WriteRune(ru)
}

func (r *ArticleRenderer) endOfLine() {
	line := r.lineBuf.Bytes()
	lineHtml := r.lineBufHtml.Bytes()

	if r.lineNo < 5 {
		if bytes.HasPrefix(line, []byte(pttbbs.ArticleTitlePrefix)) {
			r.title = string(bytes.TrimSpace(line[len([]byte(pttbbs.ArticleTitlePrefix)):]))
		}
	}

	last := 0
	if urls := urlPattern.FindAllIndex(line, -1); urls != nil {
		for _, u := range urls {
			for i, n := 0, len(u); i < n; i += 2 {
				urlString := string(line[u[i]:u[i+1]])
				r.buf.Write(lineHtml[last:r.indexArr[u[i]]])
				r.buf.WriteString(`<a href="` + html.EscapeString(urlString) + `" target="_blank" rel="nofollow">`)
				r.buf.Write(lineHtml[r.indexArr[u[i]]:r.indexArr[u[i+1]]])
				r.buf.WriteString(`</a>`)
				last = r.indexArr[u[i+1]]
			}
		}
	}
	r.buf.Write(lineHtml[last:])
	// No need to reset indexArr here, since needed values will be overwritten.

	if rcs, err := FindRichContents(r.lineBuf.Bytes()); err == nil {
		for _, rc := range rcs {
			r.buf.WriteString(`<div class="richcontent">` + rc.ContentHtml + `</div>`)
		}
	}

	r.lineBufHtml.Reset()
	r.lineBuf.Reset()
	r.lineNo++
}
