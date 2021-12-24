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
	ZeroArticlePart   *ArticlePart
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

func makeSerializer[V any]() func(*V) ([]byte, error) {
	return func(val *V) ([]byte, error) {
		return gobEncodeBytes(val)
	}
}

func makeDeserializer[V any]() func([]byte) (*V, error) {
	return func(data []byte) (*V, error) {
		val := new(V)
		if err := gobDecode(data, val); err != nil {
			return nil, err
		}
		return val, nil
	}
}

func makeTypedCache[K cache.Key, V any](c *cache.CacheManager, gen cache.Generator[K, *V]) *cache.TypedManager[*cache.CacheManager, K, *V] {
	return cache.NewTyped(c, gen, makeSerializer[V](), makeDeserializer[V]())
}

type Article struct {
	ParsedTitle     string
	PreviewContent  string
	ContentHtml     []byte
	ContentTailHtml []byte
	IsPartial       bool
	IsTruncated     bool

	CacheKey   string
	NextOffset int

	IsValid bool
}

type ArticlePart struct {
	ContentHtml string
	CacheKey    string
	NextOffset  int
	IsValid     bool
}

type BbsIndex page.BbsIndex

type BoardAtomFeed struct {
	Feed    *atom.Feed
	IsValid bool
}

func init() {
	gob.Register(Article{})
	gob.Register(ArticlePart{})
	gob.Register(BbsIndex{})
	gob.Register(BoardAtomFeed{})
}
