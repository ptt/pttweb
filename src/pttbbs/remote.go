package pttbbs

import (
	"bytes"
	"code.google.com/p/vitess/go/memcache"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidArticleLine   = errors.New("Article line format error")
	ErrAttributeFetchFailed = errors.New("Cannot fetch some attributes")
	ErrUnknownValueType     = errors.New("Unknown value type")
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
	var err error
	var memd *memcache.Connection
	if memd, err = p.connPool.GetConn(); err != nil {
		return 0, err
	}
	defer func() {
		p.connPool.ReleaseConn(memd, err)
	}()

	j := 0
	for i, t := range types {
		var result []byte
		key := keyvalue[j].(string)
		val := keyvalue[j+1]
		switch t {
		case 's':
			if result, _, err = memd.Get(key); err == nil {
				*val.(*string) = string(result)
			}
		case 'i':
			if result, _, err = memd.Get(key); err == nil {
				*val.(*int), err = strconv.Atoi(string(result))
			}
		case 'b':
			if result, _, err = memd.Get(key); err == nil {
				*val.(*[]byte) = result
			}
		default:
			return i, ErrUnknownValueType
		}
		if err != nil {
			return i, err
		}
		j += 2
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
