package main

import (
	"errors"
)

var (
	ErrCursorOutOfRange = errors.New("Paging cursor out of range")
	ErrPageNoOutOfRange = errors.New("Page number out of range")
)

type Paging struct {
	nrPerPage, nrEntries, nrPages, cursor, curPage int
}

func NewPaging(nrPerPage, nrEntries int) *Paging {
	npages := nrEntries / nrPerPage
	if nrEntries%nrPerPage > 0 {
		npages++
	}
	if npages == 0 {
		npages++
	}
	return &Paging{
		nrPerPage: nrPerPage,
		nrEntries: nrEntries,
		nrPages:   npages,
		cursor:    0,
		curPage:   1,
	}
}

func (p *Paging) HasPrev() bool {
	return p.curPage > 1
}

func (p *Paging) HasNext() bool {
	return p.curPage < p.LastPageNo()
}

func (p *Paging) PrevPageNo() int {
	return p.curPage - 1
}

func (p *Paging) NextPageNo() int {
	return p.curPage + 1
}

func (p *Paging) FirstPageNo() int {
	return 1
}

func (p *Paging) LastPageNo() int {
	return p.nrPages
}

func (p *Paging) SetCursor(i int) error {
	if i < 0 || i >= p.nrEntries {
		return ErrCursorOutOfRange
	}
	p.cursor = i
	return nil
}

func (p *Paging) SetPageNo(no int) error {
	if no < 1 || no > p.LastPageNo() {
		return ErrPageNoOutOfRange
	}
	p.cursor = (no - 1) * p.nrPerPage
	return nil
}

func (p *Paging) Cursor() int {
	return p.cursor
}
