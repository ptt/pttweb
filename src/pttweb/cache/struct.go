package cache

import (
	"bytes"
	"encoding/gob"
)

type Article struct {
	ParsedTitle    string
	PreviewContent string
	ContentHtml    []byte

	IsValid bool
}

func init() {
	gob.Register(Article{})
}

func (a Article) EncodeToBytes() ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(&a)
	return buf.Bytes(), err
}

func GobDecode(in []byte, out interface{}) error {
	buf := bytes.NewBuffer(in)
	return gob.NewDecoder(buf).Decode(out)
}
