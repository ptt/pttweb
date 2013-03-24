package main

import (
	"ansi"
	"bytes"
	"fmt"
	"html"
	"pttbbs"
	"strings"
)

type ArticleRenderer struct {
	buf         bytes.Buffer
	lineBufHtml bytes.Buffer
	lineBuf     bytes.Buffer
	indexMap    map[int]int
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
	r.indexMap = make(map[int]int)
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
		classes = append(classes, fmt.Sprintf("f%d", fg))
	}
	if bg != 0 {
		classes = append(classes, fmt.Sprintf("b%d", bg))
	}
	if flags&ansi.Highlighted == ansi.Highlighted {
		classes = append(classes, "hl")
	}
	r.lineBufHtml.WriteString("<span class=\"" + strings.Join(classes, " ") + "\">")
}

func (r *ArticleRenderer) endColor() {
	r.lineBufHtml.WriteString("</span>")
}

func (r *ArticleRenderer) oneRune(ru rune) {
	r.indexMap[r.lineBuf.Len()] = r.lineBufHtml.Len()
	r.lineBufHtml.WriteString(html.EscapeString(string(ru)))
	r.lineBuf.WriteString(string(ru))
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
				r.buf.Write(lineHtml[last:r.indexMap[u[i]]])
				r.buf.WriteString(`<a href="` + html.EscapeString(urlString) + `" target="_blank" rel="nofollow">`)
				r.buf.Write(lineHtml[r.indexMap[u[i]]:r.indexMap[u[i+1]]])
				r.buf.WriteString(`</a>`)
				last = r.indexMap[u[i+1]]
			}
		}
	}
	r.buf.Write(lineHtml[last:])
	r.indexMap = make(map[int]int)

	if rcs, err := FindRichContents(r.lineBuf.Bytes()); err == nil {
		for _, rc := range rcs {
			r.buf.WriteString(`<div class="richcontent">` + rc.ContentHtml + `</div>`)
		}
	}

	r.lineBufHtml.Reset()
	r.lineBuf.Reset()
	r.lineNo++
}
