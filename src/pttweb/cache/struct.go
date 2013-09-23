package cache

import (
	"bytes"
	"encoding/gob"
	"pttbbs"
)

type Article struct {
	ParsedTitle    string
	PreviewContent string
	ContentHtml    []byte

	IsValid bool
}

type BbsIndex struct {
	Board pttbbs.Board

	HasPrevPage bool
	HasNextPage bool
	PrevPage    int
	NextPage    int
	TotalPage   int

	Articles []pttbbs.Article

	IsValid bool
}

func init() {
	gob.Register(Article{})
	gob.Register(BbsIndex{})
}

func (a Article) EncodeToBytes() ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(&a)
	return buf.Bytes(), err
}

func (bi BbsIndex) EncodeToBytes() ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(&bi)
	return buf.Bytes(), err
}

func GobDecode(in []byte, out interface{}) error {
	buf := bytes.NewBuffer(in)
	return gob.NewDecoder(buf).Decode(out)
}
