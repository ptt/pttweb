package article

import (
	"bytes"
	"io"
	"strconv"
	"strings"
)

type ExtraFlag int

const (
	PushTag ExtraFlag = 1 << iota
	PushUserId
	PushContent
	PushIpDateTime
	PushMaxVal // Dummy value
)

type Segment struct {
	*bytes.Buffer
	Tag        string
	ExtraFlags ExtraFlag
	TermState  TerminalState
}

var extraFlagClasses = []string{
	ClassPushTag,
	ClassPushUserId,
	ClassPushContent,
	ClassPushIpDatetime,
}

func (s *Segment) WriteOpen(w io.Writer) (int, error) {
	classes := make([]string, 0, 3)
	if s.TermState.Fg() != 7 {
		classes = append(classes, ClassFgPrefix+strconv.Itoa(s.TermState.Fg()))
	}
	if s.TermState.Bg() != 0 {
		classes = append(classes, ClassBgPrefix+strconv.Itoa(s.TermState.Bg()))
	}
	if s.TermState.HasFlags(Highlighted) {
		classes = append(classes, ClassHighlight)
	}
	for i, fl := 0, ExtraFlag(1); fl < PushMaxVal; i, fl = i+1, fl<<1 {
		if s.HasExtraFlags(fl) {
			classes = append(classes, extraFlagClasses[i])
		}
	}
	if len(classes) > 0 {
		return w.Write([]byte(`<` + s.Tag + ` class="` + strings.Join(classes, ` `) + `">`))
	} else {
		s.Tag = ""
	}
	return 0, nil
}

func (s *Segment) HasExtraFlags(fl ExtraFlag) bool {
	return s.ExtraFlags&fl == fl
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

func (s *Segment) TrimRight(cutset string) {
	s.Truncate(len(bytes.TrimRight(s.Bytes(), cutset)))
}
