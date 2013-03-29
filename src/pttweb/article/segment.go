package article

import (
	"ansi"
	"bytes"
	"io"
	"strconv"
	"strings"
)

type Segment struct {
	*bytes.Buffer
	Tag string

	fg, bg, flags int
}

func (s *Segment) WriteOpen(w io.Writer) (int, error) {
	classes := make([]string, 0, 3)
	if s.fg != 7 {
		classes = append(classes, `f`+strconv.Itoa(s.fg))
	}
	if s.bg != 0 {
		classes = append(classes, `b`+strconv.Itoa(s.bg))
	}
	if s.flags&ansi.Highlighted == ansi.Highlighted {
		classes = append(classes, `hl`)
	}
	if len(classes) > 0 {
		return w.Write([]byte(`<` + s.Tag + ` class="` + strings.Join(classes, ` `) + `">`))
	} else {
		s.Tag = ""
	}
	return 0, nil
}

func (s *Segment) WriteInner(w io.Writer) (int, error) {
	return w.Write(s.Bytes())
}

func (s *Segment) WriteClose(w io.Writer) (int, error) {
	if s.Tag != "" {
		return w.Write([]byte(`</` + s.Tag + `>`))
	}
	return 0, nil
}

func (s *Segment) WriteTo(w io.Writer) (total int64, err error) {
	wrote, err := s.WriteOpen(w)
	total += int64(wrote)
	if err != nil {
		return
	}

	wrote, err = s.WriteInner(w)
	total += int64(wrote)
	if err != nil {
		return
	}

	wrote, err = s.WriteClose(w)
	total += int64(wrote)
	return
}

func (s *Segment) InnerBytes() []byte {
	return s.Bytes()
}

func (s *Segment) Slice(i, j int) Segment {
	b := s.Bytes()
	if j < 0 {
		j += len(b)
	}
	return Segment{
		fg:     s.fg,
		bg:     s.bg,
		flags:  s.flags,
		Buffer: bytes.NewBuffer(b),
	}
}

func (s *Segment) Split(at int) (Segment, Segment) {
	return s.Slice(0, at), s.Slice(at, -1)
}
