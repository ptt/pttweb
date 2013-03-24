package ansi

import (
	"strconv"
	"strings"
)

type EscapeSequence struct {
	IsCSI        bool
	PrivateModes []rune // not supported, will not be parsed
	Nums         []int
	Trailings    []rune
	Mode         rune
}

func (e *EscapeSequence) ParseNumbers(buf []rune) {
	for _, part := range strings.Split(string(buf), ";") {
		num, err := strconv.Atoi(part)
		if err != nil {
			continue // be nice
		}
		e.Nums = append(e.Nums, num)
	}
}
