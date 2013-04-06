package article

import (
	"ansi"
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

type TerminalState struct {
	fg, bg, flags int
}

func (t *TerminalState) Reset() {
	t.fg = DefaultFg
	t.bg = DefaultBg
	t.flags = NoFlags
}

func (t *TerminalState) IsDefaultState() bool {
	return t.fg == DefaultFg && t.bg == DefaultBg && t.flags == NoFlags
}

func (t *TerminalState) SetColor(fg, bg, flags int) {
	t.fg = fg
	t.bg = bg
	t.flags = flags
}

func (t *TerminalState) ApplyEscapeSequence(esc ansi.EscapeSequence) {
	switch esc.Mode {
	case 'm':
		if len(esc.Nums) == 0 {
			t.Reset()
			return
		}
		fg, bg, flags := t.fg, t.bg, t.flags
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
		t.SetColor(fg, bg, flags)
	}
}

func (t *TerminalState) Equal(u *TerminalState) bool {
	return t.fg == u.fg && t.bg == u.bg && t.flags == u.flags
}

func (t *TerminalState) Fg() int {
	return t.fg
}

func (t *TerminalState) Bg() int {
	return t.bg
}

func (t *TerminalState) Flags() int {
	return t.flags
}

func (t *TerminalState) HasFlags(f int) bool {
	return t.flags&f == f
}
