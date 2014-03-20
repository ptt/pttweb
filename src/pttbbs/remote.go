package pttbbs

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.google.com/p/vitess/go/memcache"
)

var (
	ErrInvalidArticleLine   = errors.New("article line format error")
	ErrAttributeFetchFailed = errors.New("cannot fetch some attributes")
	ErrUnknownValueType     = errors.New("unknown value type")
	ErrArgumentCount        = errors.New("invalid argument count")
	ErrResultCountMismatch  = errors.New("result count returned from memcached mismatch")
)

type RemotePtt struct {
	BoarddAddr string

	connPool *MemcacheConnPool
}

func NewRemotePtt(boarddAddr string) *RemotePtt {
	return &RemotePtt{
		BoarddAddr: boarddAddr,
		connPool:   NewMemcacheConnPool(boarddAddr, 16),
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
	} else if len(res) != len(keys) {
		return 0, ErrResultCountMismatch
	}

	// Put results back
	for i, r := range res {
		val := keyvalue[2*i+1]
		switch types[i] {
		case 's':
			*val.(*string) = string(r.Value)
		case 'i':
			*val.(*int), err = strconv.Atoi(string(r.Value))
		case 'b':
			*val.(*[]byte) = r.Value
		default:
			return i, ErrUnknownValueType
		}
		if err != nil {
			return i, err
		}
	}

	return len(types), nil
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
	if _, err = p.queryMemd("iiissssi",
		key(bid, "isboard"), &isboard,
		key(bid, "over18"), &over18,
		key(bid, "hidden"), &hidden,
		key(bid, "brdname"), &brd.BrdName,
		key(bid, "title"), &brd.Title,
		key(bid, "class"), &brd.Class,
		key(bid, "BM"), &brd.BM,
		key(bid, "parent"), &brd.Parent); err != nil {
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

		articles = append(articles, Article{
			Offset:    off,
			FileName:  parts[1],
			Date:      parts[2],
			Recommend: rec,
			FileMode:  mode,
			Owner:     parts[5],
			Title:     parts[6],
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
