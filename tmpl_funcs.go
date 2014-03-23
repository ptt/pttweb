package main

import (
	"strconv"

	"github.com/ptt/pttweb/pttbbs"
)

func colored_counter(num int) string {
	if num >= 100 {
		return `<span class="hl f1">爆</span>`
	} else if num >= 10 {
		return `<span class="hl f3">` + strconv.Itoa(num) + `</span>`
	} else if num > 0 {
		return `<span class="hl f2">` + strconv.Itoa(num) + `</span>`
	} else if num <= -100 {
		return `<span class="hl f0">XX</span>`
	} else if num <= -10 {
		return `<span class="hl f0">X` + strconv.Itoa(-num/10) + `</span>`
	}
	return ""
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
