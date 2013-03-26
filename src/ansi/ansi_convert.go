package ansi

import (
	"unicode/utf8"
)

// Flags
const (
	NoFlags = 1 << iota
	Highlighted
)

const (
	DefaultFg = 7
	DefaultBg = 0
)

// States
const (
	Default = iota
	Escaping
	ParsingControl
	InControl
	SkipOne
)

type Ansi2Html struct {
	StartColor func(fg, bg, flags int)
	EndColor   func()
	Rune       func(r rune)
	EndOfLine  func()

	fg, bg, flags int
	inColor       bool
	isLineStart   bool
}

func (a *Ansi2Html) ConvertFromUTF8(input []byte) error {
	s := Default
	buf := make([]rune, 0, 16)
	var esc EscapeSequence

	a.isLineStart = true

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
				if a.isLineStart {
					if matchPrefixBytesToStrings(input[i:], quotePrefixes()) {
						a.setColor(6, DefaultBg, NoFlags)
					} else if matchPrefixBytesToStrings(input[i:], sigPrefixes()) {
						a.setColor(2, DefaultBg, NoFlags)
					} else {
						a.resetColorState()
					}
				}
				if r == '\n' && a.inColor {
					a.resetColorState()
				}
				a.Rune(r)
				if r == '\n' {
					a.EndOfLine()
				}
			}
			a.isLineStart = r == '\n'
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
				if err := a.applyEscSeq(&esc); err != nil {
					return err
				}
				s = Default
			default:
				buf = append(buf, r)
			}
		case SkipOne:
			// just skip
		}
		i += sz
	}
	if a.inColor {
		a.EndColor()
	}
	return nil
}

func (a *Ansi2Html) resetColorState() {
	a.setColor(DefaultFg, DefaultBg, NoFlags)
}

func (a *Ansi2Html) isDefaultColorState() bool {
	return a.fg == DefaultFg && a.bg == DefaultBg && a.flags == NoFlags
}

func (a *Ansi2Html) setColor(fg, bg, flags int) error {
	if a.inColor {
		a.EndColor()
	}
	a.fg = fg
	a.bg = bg
	a.flags = flags
	if !a.isDefaultColorState() {
		a.StartColor(a.fg, a.bg, a.flags)
		a.inColor = true
	} else {
		a.inColor = false
	}
	return nil
}

func (a *Ansi2Html) applyEscSeq(esc *EscapeSequence) error {
	switch esc.Mode {
	case 'm':
		if len(esc.Nums) == 0 {
			a.resetColorState()
			return nil
		}
		fg, bg, flags := a.fg, a.bg, a.flags
		for _, ctl := range esc.Nums {
			switch {
			case ctl == 0:
				fg = DefaultFg
				bg = DefaultBg
				flags = NoFlags
			case ctl == 1:
				flags |= Highlighted
			case ctl == 22:
				flags &= ^Highlighted
			case ctl >= 30 && ctl <= 37:
				fg = ctl % 10
			case ctl >= 40 && ctl <= 47:
				bg = ctl % 10
			default:
				// be nice
			}
		}
		a.setColor(fg, bg, flags)
	}
	return nil
}
