package pttbbs

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidAid = errors.New("invalid aid")
)

const (
	aidTable = `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_`
)

type Aid uint64

func ParseAid(s string) (Aid, error) {
	// Not to overflow
	if len(s) > 10 {
		return Aid(0), ErrInvalidAid
	}

	var aid Aid
parseLoop:
	for _, c := range s {
		aid <<= 6
		switch {
		case c >= '0' && c <= '9':
			aid += Aid(c - '0')
		case c >= 'A' && c <= 'Z':
			aid += Aid(c - 'A' + 10)
		case c >= 'a' && c <= 'z':
			aid += Aid(c - 'a' + 36)
		case c == '-':
			aid += 62
		case c == '_':
			aid += 63
		case c == '@':
			break parseLoop
		default:
			return Aid(0), ErrInvalidAid
		}
	}
	return aid, nil
}

func (aid Aid) String() string {
	s := make([]rune, 10)
	var i int
	for i = len(s) - 1; aid > 0 && i >= 0; i-- {
		s[i] = rune(aidTable[aid%64])
		aid /= 64
	}
	return string(s[i+1:])
}

func (aid Aid) Filename() string {
	tp := uint64((aid >> 44) & 0xF)
	v1 := uint64((aid >> 12) & 0xFFFFFFFF)
	v2 := uint64(aid & 0xFFF)

	var tpc rune
	if tp == 0 {
		tpc = 'M'
	} else {
		tpc = 'G'
	}

	return fmt.Sprintf("%c.%v.A.%03X", tpc, v1, v2)
}
