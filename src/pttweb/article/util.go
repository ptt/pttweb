package article

import (
	"ansi"
	"bytes"
	"html"
)

var (
	pushPrefixStrings = []string{"推 ", "噓 ", "→ "}
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

func makeExternalUrlLink(urlString string) (begin, end string) {
	begin = `<a href="` + html.EscapeString(urlString) + `" target="_blank" rel="nofollow">`
	end = `</a>`
	return
}

func matchColor(seg *Segment, fg, bg, flags int) bool {
	return seg.fg == fg && seg.bg == bg && (seg.flags&flags) == flags
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
		matchAny(segs[0].Bytes(), pushPrefixStrings) &&
		(matchColor(&segs[0], 1, 0, ansi.Highlighted) ||
			matchColor(&segs[0], 7, 0, ansi.Highlighted)) &&
		matchColor(&segs[1], 3, 0, ansi.Highlighted) &&
		matchColor(&segs[2], 3, 0, ansi.NoFlags) &&
		matchColor(&segs[3], 7, 0, ansi.NoFlags)
}
