package ansi

import (
	"strconv"
)

type EscapeSequence struct {
	IsCSI        bool
	PrivateModes []rune // not supported, will not be parsed
	Nums         []int
	Trailings    []rune
	Mode         rune
}

func (e *EscapeSequence) Reset() {
	e.IsCSI = false

	if e.PrivateModes == nil {
		e.PrivateModes = make([]rune, 0, 4)
	} else {
		e.PrivateModes = e.PrivateModes[0:0]
	}

	if e.Nums == nil {
		e.Nums = make([]int, 0, 4)
	} else {
		e.Nums = e.Nums[0:0]
	}

	if e.Trailings == nil {
		e.Trailings = make([]rune, 0, 4)
	} else {
		e.Trailings = e.Trailings[0:0]
	}

	e.Mode = 0
}

func (e *EscapeSequence) ParseNumbers(buf []rune) {
	part := make([]rune, 0, 4)
	for i, r := range buf {
		if r != ';' {
			part = append(part, r)
		}
		if r == ';' || i == len(buf)-1 {
			switch len(part) {
			case 0:
				// Treat empty parameter as 0 (eg. "\e[;34m" as "\e[0;34m").
				// It is not stated in the spec whether it's valid. But most
				// terminal does this and are being relied by ascii art
				// creators. Let's allow this.
				e.Nums = append(e.Nums, 0)
			default:
				num, err := strconv.Atoi(string(part))
				if err != nil {
					continue // be nice
				}
				e.Nums = append(e.Nums, num)
				part = part[0:0]
			}
		}
	}
}
