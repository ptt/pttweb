package ansi

import (
	"unicode/utf8"
)

// States
const (
	Default = iota
	Escaping
	ParsingControl
	InControl
	SkipOne
)

type AnsiParser struct {
	Rune   func(r rune)
	Escape func(e EscapeSequence)
}

func (a *AnsiParser) ConvertFromUTF8(input []byte) error {
	s := Default
	buf := make([]rune, 0, 16)
	var esc EscapeSequence

	for i, n := 0, len(input); i < n; {
		r, sz := utf8.DecodeRune(input[i:])
		if r == utf8.RuneError {
			i++
			continue
		}
		switch s {
		case Default:
			switch r {
			case 033:
				s = Escaping
				buf = buf[0:0]
				esc.Reset()
			default:
				a.Rune(r)
			}
		case Escaping:
			switch r {
			case '*':
				// special case in ptt, not implemented here
				s = SkipOne
			case '[':
				// multi-byte control sequence
				s = ParsingControl
			default:
				if r >= '@' && r <= '_' {
					// 2-char control code, not supported
					s = SkipOne
				} else {
					// error! but be nice
					a.Rune(r)
					s = Default
				}
			}
		case ParsingControl:
			switch {
			case r >= ' ' && r <= '/':
				esc.Trailings = append(esc.Trailings, r)
			case r >= '@' && r <= '~':
				esc.Mode = r
				esc.ParseNumbers(buf)
				a.Escape(esc)
				s = Default
			default:
				buf = append(buf, r)
			}
		case SkipOne:
			// just skip
			s = Default
		}
		i += sz
	}
	return nil
}
