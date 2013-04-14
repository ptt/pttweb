package pttbbs

type Pttbbs interface {
	GetBoardChildren(bid int) (children []int, err error)
	GetBoard(bid int) (brd Board, err error)
	GetArticleCount(bid int) (int, error)
	GetArticleList(bid, offset int) ([]Article, error)
	GetArticleContent(bid int, filename string) ([]byte, error)
	BrdName2Bid(brdname string) (int, error)
}

type Board struct {
	Bid     int
	IsBoard bool
	Over18  bool
	Hidden  bool
	BrdName string
	Title   string
	Class   string
	BM      string
	Parent  int
}

type Article struct {
	Offset    int
	FileName  string
	Date      string
	Recommend int
	FileMode  int
	Owner     string
	Title     string
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
