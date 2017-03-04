package main

import (
	"bytes"
	"encoding/gob"

	"golang.org/x/tools/blog/atom"

	"github.com/ptt/pttweb/cache"
	"github.com/ptt/pttweb/page"
)

// Useful when calling |NewFromBytes|
var (
	ZeroArticle       *Article
	ZeroBbsIndex      *BbsIndex
	ZeroBoardAtomFeed *BoardAtomFeed
)

func gobEncodeBytes(obj interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(obj); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gobDecode(in []byte, out interface{}) error {
	buf := bytes.NewBuffer(in)
	return gob.NewDecoder(buf).Decode(out)
}

func gobDecodeCacheable(data []byte, obj cache.Cacheable) (cache.Cacheable, error) {
	if err := gobDecode(data, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

type Article struct {
	ParsedTitle     string
	PreviewContent  string
	ContentHtml     []byte
	ContentTailHtml []byte
	IsPartial       bool
	IsTruncated     bool

	IsValid bool
}

func (_ *Article) NewFromBytes(data []byte) (cache.Cacheable, error) {
	return gobDecodeCacheable(data, new(Article))
}

func (a *Article) EncodeToBytes() ([]byte, error) {
	return gobEncodeBytes(a)
}

type BbsIndex page.BbsIndex

func (_ *BbsIndex) NewFromBytes(data []byte) (cache.Cacheable, error) {
	return gobDecodeCacheable(data, new(BbsIndex))
}

func (bi *BbsIndex) EncodeToBytes() ([]byte, error) {
	return gobEncodeBytes(bi)
}

type BoardAtomFeed struct {
	Feed    *atom.Feed
	IsValid bool
}

func (_ *BoardAtomFeed) NewFromBytes(data []byte) (cache.Cacheable, error) {
	return gobDecodeCacheable(data, new(BoardAtomFeed))
}

func (bi *BoardAtomFeed) EncodeToBytes() ([]byte, error) {
	return gobEncodeBytes(bi)
}

func init() {
	gob.Register(Article{})
	gob.Register(BbsIndex{})
	gob.Register(BoardAtomFeed{})

	// Make sure they are |Cacheable|
	checkCacheable(new(Article))
	checkCacheable(new(BbsIndex))
	checkCacheable(new(BoardAtomFeed))
}

func checkCacheable(c cache.Cacheable) {
	// Empty
}
