package pttbbs

import (
	"errors"
	"time"

	apipb "github.com/ptt/pttweb/proto/api"
)

var (
	ErrNotFound = errors.New("not found")
)

type BoardID uint32

type BoardRef interface {
	boardRef() *apipb.BoardRef
}

type Pttbbs interface {
	GetBoards(refs ...BoardRef) ([]Board, error)
	GetArticleList(ref BoardRef, offset, length int) ([]Article, error)
	GetBottomList(ref BoardRef) ([]Article, error)
	GetArticleSelect(ref BoardRef, meth SelectMethod, filename, cacheKey string, offset, maxlen int) (*ArticlePart, error)
	Hotboards() ([]Board, error)
	Search(ref BoardRef, preds []SearchPredicate, offset, length int) (articles []Article, totalPosts int, err error)
}

func OneBoard(boards []Board, err error) (Board, error) {
	if err != nil {
		return Board{}, err
	}
	if len(boards) != 1 {
		return Board{}, errors.New("expect one board")
	}
	return boards[0], nil
}

type Board struct {
	Bid      BoardID
	IsBoard  bool
	Over18   bool
	Hidden   bool
	BrdName  string
	Title    string
	Class    string
	BM       string
	Parent   int
	Nuser    int
	NumPosts int
	Children []BoardID
}

func (b Board) Ref() BoardRef {
	return BoardRefByBid(b.Bid)
}

type Article struct {
	Offset    int
	FileName  string
	Date      string
	Recommend int
	FileMode  int
	Owner     string
	Title     string
	Modified  time.Time
}

type ArticlePart struct {
	CacheKey string
	FileSize int
	Offset   int
	Length   int
	Content  []byte
}

// Non-mail file modes
const (
	FileLocal = 1 << iota
	FileMarked
	FileDigest
	FileBottom
	FileSolved
)

// Mail file modes
const (
	FileRead = 1 << iota
	_        // FileMarked
	FileReplied
	FileMulti
)

type SelectMethod string

const (
	SelectPart SelectMethod = `articlepart`
	SelectHead              = `articlehead`
	SelectTail              = `articletail`
)

const (
	BoardGroup  uint32 = 0x00000008
	BoardOver18        = 0x01000000
)
