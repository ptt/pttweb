package pttbbs

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/youtube/vitess/go/memcache"
)

var (
	ErrInvalidArticleLine   = errors.New("article line format error")
	ErrAttributeFetchFailed = errors.New("cannot fetch some attributes")
	ErrUnknownValueType     = errors.New("unknown value type")
	ErrArgumentCount        = errors.New("invalid argument count")
	ErrResultCountMismatch  = errors.New("result count returned from memcached mismatch")
	ErrNoMetaLine           = errors.New("no meta line in result")
	ErrMetaLineFormat       = errors.New("meta line format error")
	ErrNotFound             = errors.New("not found")
)

type RemotePtt struct {
	BoarddAddr string

	connPool *MemcacheConnPool
}

func NewRemotePtt(boarddAddr string, maxOpen int) *RemotePtt {
	return &RemotePtt{
		BoarddAddr: boarddAddr,
		connPool:   NewMemcacheConnPool(boarddAddr, maxOpen),
	}
}

func key(bid int, attr string) string {
	return fmt.Sprintf("%d.%s", bid, attr)
}

func (p *RemotePtt) queryMemd(types string, keyvalue ...interface{}) (int, error) {
	if 2*len(types) != len(keyvalue) {
		return 0, ErrArgumentCount
	}

	var err error
	var memd *memcache.Connection
	if memd, err = p.connPool.GetConn(); err != nil {
		return 0, err
	}
	defer func() {
		p.connPool.ReleaseConn(memd, err)
	}()

	// Prepare keys to fetch in batch
	keys := make([]string, len(types))
	for i := range keys {
		keys[i] = keyvalue[2*i].(string)
	}

	res, err := memd.Get(keys...)
	if err != nil {
		return 0, err
	}

	// Put results back
	j := 0
	for i, key := range keys {
		var val []byte
		if j < len(res) && key == res[j].Key {
			// Consume the result
			val = res[j].Value
			j++
		}
		if err = setVal(types[i], keyvalue[2*i+1], val); err != nil {
			return i, err
		}
	}

	return len(types), nil
}

func setVal(t uint8, dst interface{}, src []byte) (err error) {
	if src != nil {
		switch t {
		case 's':
			*dst.(*string) = string(src)
		case 'i':
			*dst.(*int), err = strconv.Atoi(string(src))
		case 'b':
			*dst.(*[]byte) = src
		default:
			err = ErrUnknownValueType
		}
	} else {
		switch t {
		case 's':
			*dst.(*string) = ""
		case 'i':
			*dst.(*int) = 0
		case 'b':
			*dst.(*[]byte) = nil
		default:
			err = ErrUnknownValueType
		}
	}
	return
}

func (p *RemotePtt) GetBoardChildren(bid int) (children []int, err error) {
	var result []byte
	if _, err = p.queryMemd("b", key(bid, "children"), &result); err != nil {
		return
	}

	children = make([]int, 0, 16)
	buf := bytes.NewReader(result)
	for {
		child := 0
		if n, err := fmt.Fscanf(buf, "%d,", &child); n != 1 || err != nil {
			break
		}
		children = append(children, child)
	}
	return
}

func (p *RemotePtt) GetBoard(bid int) (brd Board, err error) {
	var isboard, over18, hidden int
	if _, err = p.queryMemd("iiissssii",
		key(bid, "isboard"), &isboard,
		key(bid, "over18"), &over18,
		key(bid, "hidden"), &hidden,
		key(bid, "brdname"), &brd.BrdName,
		key(bid, "title"), &brd.Title,
		key(bid, "class"), &brd.Class,
		key(bid, "BM"), &brd.BM,
		key(bid, "parent"), &brd.Parent,
		key(bid, "nuser"), &brd.Nuser,
	); err != nil {
		err = ErrAttributeFetchFailed
		return
	}
	brd.IsBoard = isboard == 1
	brd.Over18 = over18 == 1
	brd.Hidden = hidden == 1
	brd.Bid = bid
	return
}

func (p *RemotePtt) GetArticleCount(bid int) (count int, err error) {
	_, err = p.queryMemd("i", key(bid, "count"), &count)
	return
}

func (p *RemotePtt) GetArticleList(bid, offset int) (articles []Article, err error) {
	var result string
	if _, err = p.queryMemd("s", key(bid, "articles."+strconv.Itoa(offset)), &result); err != nil {
		return nil, err
	}
	return parseDirList(result)
}

func (p *RemotePtt) GetBottomList(bid int) (articles []Article, err error) {
	var result string
	if _, err = p.queryMemd("s", key(bid, "bottoms"), &result); err != nil {
		return nil, err
	}
	return parseDirList(result)
}

func parseDirList(result string) (articles []Article, err error) {
	articles = make([]Article, 0, 20)

	for _, line := range strings.Split(result, "\n") {
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ",", 7)
		if len(parts) != 7 {
			return nil, ErrInvalidArticleLine
		}

		var off, rec, mode int
		if off, err = strconv.Atoi(parts[0]); err != nil {
			return nil, err
		}
		if rec, err = strconv.Atoi(parts[3]); err != nil {
			return nil, err
		}
		if mode, err = strconv.Atoi(parts[4]); err != nil {
			return nil, err
		}

		// Use the filename time or a zero value as the modified time.
		modified, _ := ParseFileNameTime(parts[1])
		articles = append(articles, Article{
			Offset:    off,
			FileName:  parts[1],
			Date:      parts[2],
			Recommend: rec,
			FileMode:  mode,
			Owner:     parts[5],
			Title:     parts[6],
			Modified:  modified,
		})
	}
	return articles, nil
}

func (p *RemotePtt) GetArticleContent(bid int, filename string) (content []byte, err error) {
	_, err = p.queryMemd("b", key(bid, "article."+filename), &content)
	return
}

func (p *RemotePtt) BrdName2Bid(brdname string) (bid int, err error) {
	_, err = p.queryMemd("i", "tobid."+brdname, &bid)
	return
}

func parseMetaLine(p *ArticlePart, line string) (err error) {
	comp := strings.Split(line, ",")
	if len(comp) != 4 {
		return ErrMetaLineFormat
	}
	p.CacheKey = comp[0]
	p.FileSize, err = strconv.Atoi(comp[1])
	if err != nil {
		return
	}
	p.Offset, err = strconv.Atoi(comp[2])
	if err != nil {
		return
	}
	p.Length, err = strconv.Atoi(comp[3])
	if err != nil {
		return
	}
	return
}

func (p *RemotePtt) GetArticleSelect(bid int, meth SelectMethod, filename, cacheKey string, offset, maxlen int) (*ArticlePart, error) {
	var res []byte
	var metaLen int
	_, err := p.queryMemd("b", fmt.Sprintf("%v.%v.%v.%v.%v.%v", bid, string(meth), cacheKey, offset, maxlen, filename), &res)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, ErrNotFound
	}
	for i, ch := range res {
		if ch == '\n' {
			metaLen = i
			break
		}
	}
	if metaLen == 0 {
		return nil, ErrNoMetaLine
	}
	part := new(ArticlePart)
	if err := parseMetaLine(part, string(res[:metaLen])); err != nil {
		return nil, err
	}
	part.Content = res[metaLen+1:]
	return part, nil
}

func (p *RemotePtt) Hotboards() ([]Board, error) {
	bids, err := p.hotboardBids()
	if err != nil {
		return nil, err
	}
	return p.GetBoards(bids)
}

func (p *RemotePtt) hotboardBids() ([]int, error) {
	var bidListStr string
	_, err := p.queryMemd("s", "hotboards", &bidListStr)
	if err != nil {
		return nil, err
	}
	var bids []int
	for _, bidStr := range strings.Split(bidListStr, ",") {
		if len(bidStr) == 0 {
			continue
		}
		bid, err := strconv.Atoi(bidStr)
		if err != nil {
			return nil, err
		}
		bids = append(bids, bid)
	}
	return bids, nil
}

func (p *RemotePtt) GetBoards(bids []int) ([]Board, error) {
	boards := make([]Board, 0, 16)
	for _, bid := range bids {
		brd, err := p.GetBoard(bid)
		if err != nil {
			// Ignore errors.
			continue
		}
		// List only valid boards.
		if IsValidBrdName(brd.BrdName) && !brd.Hidden {
			boards = append(boards, brd)
		}
	}
	return boards, nil
}
