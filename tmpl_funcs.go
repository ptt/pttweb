package main

import (
	"html/template"
	"strconv"

	"github.com/ptt/pttweb/pttbbs"
)

func colored_counter(num int) template.HTML {
	switch {
	case num >= 100:
		return template.HTML(`<span class="hl f1">爆</span>`)
	case num >= 10:
		return template.HTML(`<span class="hl f3">` + strconv.Itoa(num) + `</span>`)
	case num > 0:
		return template.HTML(`<span class="hl f2">` + strconv.Itoa(num) + `</span>`)
	case num > -10:
		return ""
	case num > -100:
		return template.HTML(`<span class="hl f0">X` + strconv.Itoa(-num/10) + `</span>`)
	default:
		return template.HTML(`<span class="hl f0">XX</span>`)
	}
}

func decorate_board_nuser(num int) template.HTML {
	switch {
	case num < 1:
		return ""
	case num <= 10:
		return template.HTML(strconv.Itoa(num))
	case num <= 50:
		return template.HTML(`<span class="hl f3">` + strconv.Itoa(num) + `</span>`)
	case num < 2000:
		return template.HTML(`<span class="hl">` + strconv.Itoa(num) + `</span>`)
	case num < 5000:
		return template.HTML(`<span class="hl f1">` + strconv.Itoa(num) + `</span>`)
	case num < 10000:
		return template.HTML(`<span class="hl f4">` + strconv.Itoa(num) + `</span>`)
	case num < 30000:
		return template.HTML(`<span class="hl f6">` + strconv.Itoa(num) + `</span>`)
	case num < 60000:
		return template.HTML(`<span class="hl f2">` + strconv.Itoa(num) + `</span>`)
	case num < 100000:
		return template.HTML(`<span class="hl f3">` + strconv.Itoa(num) + `</span>`)
	default:
		return template.HTML(`<span class="hl f5">` + strconv.Itoa(num) + `</span>`)
	}
}

func post_mark(mode int) string {
	if mode&(pttbbs.FileMarked|pttbbs.FileSolved) == pttbbs.FileMarked|pttbbs.FileSolved {
		return `!`
	} else if mode&pttbbs.FileMarked == pttbbs.FileMarked {
		return `M`
	} else if mode&pttbbs.FileSolved == pttbbs.FileSolved {
		return `S`
	}
	return ``
}
