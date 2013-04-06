package article

import (
	"bytes"
	"io"
	"strconv"
	"strings"
)

type ExtraFlag int

const (
	_                 = iota
	PushTag ExtraFlag = 1 << iota
	PushUserId
	PushContent
	PushIpDateTime
)

type Segment struct {
	*bytes.Buffer
	Tag        string
	ExtraFlags ExtraFlag
	TermState  TerminalState
}

func (s *Segment) WriteOpen(w io.Writer) (int, error) {
	classes := make([]string, 0, 3)
	if s.TermState.Fg() != 7 {
		classes = append(classes, `f`+strconv.Itoa(s.TermState.Fg()))
	}
	if s.TermState.Bg() != 0 {
		classes = append(classes, `b`+strconv.Itoa(s.TermState.Bg()))
	}
	if s.TermState.HasFlags(Highlighted) {
		classes = append(classes, `hl`)
	}
	if s.ExtraFlags&PushTag == PushTag {
		classes = append(classes, `push_tag`)
	}
	if s.ExtraFlags&PushUserId == PushUserId {
		classes = append(classes, `push_userid`)
	}
	if s.ExtraFlags&PushContent == PushContent {
		classes = append(classes, `push_content`)
	}
	if s.ExtraFlags&PushIpDateTime == PushIpDateTime {
		classes = append(classes, `push_ipdatetime`)
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
		TermState: s.TermState,
		Buffer:    bytes.NewBuffer(b),
	}
}

func (s *Segment) Split(at int) (Segment, Segment) {
	return s.Slice(0, at), s.Slice(at, -1)
}
