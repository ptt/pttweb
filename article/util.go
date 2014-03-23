package article

import (
	"bytes"
	"html"

	"github.com/ptt/pttweb/pttbbs"
)

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

func fastWriteHtmlEscaped(buf *bytes.Buffer, str string) {
	for _, ru := range str {
		if ru == 0xFFFD {
			// Invalid UTF-8 sequence
			continue
		}
		fastWriteHtmlEscapedRune(buf, ru)
	}
}

func makeExternalUrlLink(urlString string) (begin, end string) {
	begin = `<a href="` + html.EscapeString(urlString) + `" target="_blank" rel="nofollow">`
	end = `</a>`
	return
}

func matchColor(t *TerminalState, fg, bg, flags int) bool {
	return t.Fg() == fg && t.Bg() == bg && t.HasFlags(flags)
}

func matchAny(b []byte, patt []string) bool {
	for _, p := range patt {
		if bytes.Equal(b, []byte(p)) {
			return true
		}
	}
	return false
}

func matchPushLine(segs []Segment) bool {
	return len(segs) == 4 &&
		matchAny(segs[0].Bytes(), pttbbs.ArticlePushPrefixStrings) &&
		(matchColor(&segs[0].TermState, 1, 0, Highlighted) ||
			matchColor(&segs[0].TermState, 7, 0, Highlighted)) &&
		matchColor(&segs[1].TermState, 3, 0, Highlighted) &&
		matchColor(&segs[2].TermState, 3, 0, NoFlags) &&
		matchColor(&segs[3].TermState, 7, 0, NoFlags)
}
