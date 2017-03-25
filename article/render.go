package article

import (
	"bytes"
	"log"
	"strings"

	"github.com/ptt/pttweb/ansi"
	"github.com/ptt/pttweb/pttbbs"
	"github.com/ptt/pttweb/richcontent"

	"golang.org/x/net/context"
)

const (
	kPreviewContentLines = 5
)

type RenderOption func(*renderer)

func WithContent(content []byte) RenderOption {
	return func(r *renderer) {
		r.content = content
	}
}

func WithContext(ctx context.Context) RenderOption {
	return func(r *renderer) {
		r.ctx = ctx
	}
}

func WithDisableArticleHeader() RenderOption {
	return func(r *renderer) {
		r.disableArticleHeader = true
	}
}

type RenderedArticle interface {
	ParsedTitle() string
	PreviewContent() string
	HTML() []byte
}

func Render(opts ...RenderOption) (RenderedArticle, error) {
	r := newRenderer()
	for _, opt := range opts {
		opt(r)
	}
	if err := r.Render(); err != nil {
		return nil, err
	}
	return r, nil
}

type renderer struct {
	// Options.
	content              []byte
	disableArticleHeader bool
	ctx                  context.Context

	// Internal states.
	buf    bytes.Buffer
	lineNo int

	mapper    *IndexMapper
	lineBuf   bytes.Buffer
	lineSegs  []Segment
	segIndex  int
	segOffset int
	segClosed bool

	terminalState TerminalState

	acceptMetaLines bool

	title string

	previewContent   string
	previewLineCount int
}

func newRenderer() *renderer {
	ar := &renderer{
		ctx:      context.TODO(),
		mapper:   NewIndexMapper(2),
		lineSegs: make([]Segment, 0, 8),
	}
	ar.init()
	return ar
}

func (r *renderer) init() {
	r.buf.Reset()
	r.lineNo = 1

	r.mapper.Reset()
	r.lineBuf.Reset()
	r.lineSegs = r.lineSegs[0:0]
	r.segIndex = 0
	r.segOffset = 0
	r.segClosed = true

	r.terminalState.Reset()

	r.acceptMetaLines = true

	r.title = ""

	r.previewContent = ""
	r.previewLineCount = 0
}

func (r *renderer) ParsedTitle() string {
	return r.title
}

func (r *renderer) PreviewContent() string {
	return r.previewContent
}

func (r *renderer) HTML() []byte {
	return r.buf.Bytes()
}

func (r *renderer) Render() error {
	converter := &ansi.AnsiParser{
		Rune:   r.oneRune,
		Escape: r.escape,
	}
	if err := converter.ConvertFromUTF8(r.content); err != nil {
		return err
	}
	// Simulate end of line if there isn't one at the end.
	if r.lineBuf.Len() > 0 {
		r.endOfLine()
	}
	return nil
}

func (r *renderer) currSeg() *Segment {
	if len(r.lineSegs) == 0 || r.segClosed {
		r.startSegment()
	}
	return &r.lineSegs[len(r.lineSegs)-1]
}

func (r *renderer) escape(esc ansi.EscapeSequence) {
	r.terminalState.ApplyEscapeSequence(esc)
	if r.segClosed || !r.terminalState.Equal(&r.currSeg().TermState) {
		r.startSegment()
	}
}

func (r *renderer) startSegment() {
	if !r.segClosed {
		r.endSegment()
	}
	r.lineSegs = append(r.lineSegs, Segment{
		Tag:       "span",
		TermState: r.terminalState,
		Buffer:    &bytes.Buffer{},
	})
	r.segClosed = false
}

func (r *renderer) endSegment() {
	// Remove empty segment
	if r.lineSegs[len(r.lineSegs)-1].Len() == 0 {
		r.lineSegs = r.lineSegs[:len(r.lineSegs)-1]
	}

	r.segClosed = true
}

func (r *renderer) oneRune(ru rune) {
	seg := r.currSeg()
	r.mapper.Record(r.lineBuf.Len(), len(r.lineSegs)-1, seg.Len())
	fastWriteHtmlEscapedRune(seg.Buffer, ru)
	r.lineBuf.WriteRune(ru)

	if ru == '\n' {
		r.endOfLine()
	}
}

func (r *renderer) outputToSegment(i, off int) {
	for ; r.segIndex < i; r.segIndex++ {
		s := &r.lineSegs[r.segIndex]
		r.maybeOpenCurrentSegment()
		r.buf.Write(s.InnerBytes()[r.segOffset:])
		r.maybeCloseCurrentSegment()
		// advance to next segment at offset 0.
		r.segOffset = 0
	}
	if off > 0 {
		s := &r.lineSegs[r.segIndex]
		r.maybeOpenCurrentSegment()
		r.buf.Write(s.InnerBytes()[r.segOffset:off])
		r.segOffset = off
	}
}

func (r *renderer) skipToSegment(i, off int) {
	r.maybeCloseCurrentSegment()
	r.segIndex = i
	r.segOffset = off
}

func (r *renderer) maybeOpenCurrentSegment() {
	if r.segClosed {
		r.lineSegs[r.segIndex].WriteOpen(&r.buf)
		r.segClosed = false
	}
}

func (r *renderer) maybeCloseCurrentSegment() {
	if !r.segClosed {
		r.lineSegs[r.segIndex].WriteClose(&r.buf)
		r.segClosed = true
	}
}

func (r *renderer) matchFirstLineAndOutput(line []byte) bool {
	tag1, val1, tag2, val2, ok := pttbbs.ParseArticleFirstLine(r.lineBuf.Bytes())
	if !ok {
		return false
	}

	r.writeMetaLine(tag1, val1, ClassArticleMetaLine)
	r.writeMetaLine(tag2, val2, ClassArticleMetaLineRight)
	return true
}

func (r *renderer) writeMetaLine(tag, val []byte, divClass string) {
	r.buf.WriteString(`<div class="` + divClass + `"><span class="` + ClassArticleMetaTag + `">`)
	fastWriteHtmlEscaped(&r.buf, string(tag))
	r.buf.WriteString(`</span>`)
	r.buf.WriteString(`<span class="` + ClassArticleMetaValue + `">`)
	fastWriteHtmlEscaped(&r.buf, string(val))
	r.buf.WriteString(`</span></div>`)
}

func (r *renderer) endOfLine() {
	r.segClosed = true

	// Map pass the end of line to end of seg.
	r.mapper.Record(r.lineBuf.Len(), len(r.lineSegs), 0)
	line := r.lineBuf.Bytes()
	parsed := false

	if !r.disableArticleHeader && r.acceptMetaLines && r.lineNo < 5 {
		if r.lineNo == 1 && r.matchFirstLineAndOutput(line) {
			parsed = true
		} else if tag, val, ok := pttbbs.ParseArticleMetaLine(line); ok {
			if bytes.Equal(tag, []byte(pttbbs.ArticleTitle)) {
				r.title = string(val)
			}
			r.writeMetaLine(tag, val, ClassArticleMetaLine)
			parsed = true
		} else {
			r.acceptMetaLines = false
		}
	}

	if !parsed {
		isMainContent := false
		if len(r.lineSegs) > 0 {
			if pttbbs.MatchPrefixBytesToStrings(line, pttbbs.QuotePrefixStrings) {
				r.lineSegs[0].TermState.SetColor(6, DefaultBg, NoFlags)
			} else if pttbbs.MatchPrefixBytesToStrings(line, pttbbs.SignaturePrefixStrings) {
				r.lineSegs[0].TermState.SetColor(2, DefaultBg, NoFlags)
			} else {
				// Non-empty, not quote, and not signature line.
				isMainContent = true
			}
		}

		// Collect non-empty lines as preview starting at first main
		// content line.
		isEmpty := len(strings.TrimSpace(string(line))) == 0
		canCollect := !isEmpty && (r.previewLineCount == 0 && isMainContent || r.previewLineCount > 0)
		if canCollect && r.previewLineCount < kPreviewContentLines {
			r.previewContent += string(line)
			r.previewLineCount++
		}
		r.processNormalContentLine(line)
	}

	// Reset and update variables
	r.mapper.Reset()
	r.lineBuf.Reset()
	r.lineSegs = r.lineSegs[0:0]
	r.segIndex = 0
	r.segOffset = 0
	r.segClosed = true
	r.lineNo++
}

func (r *renderer) processNormalContentLine(line []byte) {
	// Detect push line
	isPush := false
	if matchPushLine(r.lineSegs) {
		r.lineSegs[0].ExtraFlags |= PushTag
		r.lineSegs[1].ExtraFlags |= PushUserId
		r.lineSegs[2].ExtraFlags |= PushContent
		r.lineSegs[3].ExtraFlags |= PushIpDateTime
		// Remove trailing spaces
		r.lineSegs[2].TrimRight(" ")
		r.buf.WriteString(`<div class="` + ClassPushDiv + `">`)
		isPush = true
	}

	rcs, err := richcontent.Find(r.ctx, line)
	if err != nil {
		rcs = nil
		log.Println("warning: rendering article: richcontent.Find:", err)
	}

	for _, rc := range rcs {
		linkBegin, linkEnd := makeExternalUrlLink(rc.URLString())

		lbegin, lend := rc.Pos()
		begin := r.mapper.Get(lbegin)
		end := r.mapper.Get(lend)
		r.outputToSegment(begin[0], begin[1])
		if begin[0] == end[0] {
			// same segment: embed
			r.maybeOpenCurrentSegment()
			r.buf.WriteString(linkBegin)
			r.outputToSegment(end[0], end[1])
			r.buf.WriteString(linkEnd)
		} else {
			// different segments: split, wrap-around
			r.maybeCloseCurrentSegment()
			r.buf.WriteString(linkBegin)
			r.outputToSegment(end[0], end[1])
			r.maybeCloseCurrentSegment()
			r.buf.WriteString(linkEnd)
		}
	}
	r.outputToSegment(len(r.lineSegs), 0)

	if isPush {
		r.buf.WriteString(`</div>`)
	}

	// Append rich contents to next line.
	for _, rc := range rcs {
		for _, comp := range rc.Components() {
			r.buf.WriteString(`<div class="richcontent">` + comp.HTML() + `</div>`)
		}
	}
}
