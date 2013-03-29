package article

import (
	"bytes"
	"html"
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
