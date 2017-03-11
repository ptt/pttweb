package pttbbs

type Pttbbs interface {
	GetBoardChildren(bid int) (children []int, err error)
	GetBoard(bid int) (brd Board, err error)
	GetBoards(bids []int) (brd []Board, err error)
	GetArticleCount(bid int) (int, error)
	GetArticleList(bid, offset int) ([]Article, error)
	GetBottomList(bid int) ([]Article, error)
	GetArticleContent(bid int, filename string) ([]byte, error)
	BrdName2Bid(brdname string) (int, error)
	GetArticleSelect(bid int, meth SelectMethod, filename, cacheKey string, offset, maxlen int) (*ArticlePart, error)
	Hotboards() ([]Board, error)
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
	Nuser   int
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
