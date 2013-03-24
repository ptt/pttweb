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
)

type RemotePtt struct {
	BoarddAddr string
}

func NewRemotePtt(boarddAddr string) *RemotePtt {
	return &RemotePtt{
		BoarddAddr: boarddAddr,
	}
}

func key(bid int, attr string) string {
	return fmt.Sprintf("%d.%s", bid, attr)
}

func queryString(brdd *memcache.Connection, key string, ret *string) (err error) {
	var result []byte
	if result, _, err = brdd.Get(key); err == nil {
		*ret = bytes.NewBuffer(result).String()
	}
	return
}

func queryInt(brdd *memcache.Connection, key string, ret *int) (err error) {
	var result []byte
	if result, _, err = brdd.Get(key); err == nil {
		_, err = fmt.Fscanf(bytes.NewReader(result), "%d", ret)
	}
	return
}

func (p *RemotePtt) GetBoardChildren(bid int) (children []int, err error) {
	var memd *memcache.Connection
	if memd, err = memcache.Connect(p.BoarddAddr); err != nil {
		return
	}
	defer memd.Close()

	var result []byte
	if result, _, err = memd.Get(key(bid, "children")); err != nil {
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
	var memd *memcache.Connection
	if memd, err = memcache.Connect(p.BoarddAddr); err != nil {
		return
	}
	defer memd.Close()

	var isboard, over18, hidden int
	if queryInt(memd, key(bid, "isboard"), &isboard) != nil ||
		queryInt(memd, key(bid, "over18"), &over18) != nil ||
		queryInt(memd, key(bid, "hidden"), &hidden) != nil ||
		queryString(memd, key(bid, "brdname"), &brd.BrdName) != nil ||
		queryString(memd, key(bid, "title"), &brd.Title) != nil ||
		queryString(memd, key(bid, "class"), &brd.Class) != nil ||
		queryString(memd, key(bid, "BM"), &brd.BM) != nil ||
		queryInt(memd, key(bid, "parent"), &brd.Parent) != nil {
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
	var memd *memcache.Connection
	if memd, err = memcache.Connect(p.BoarddAddr); err != nil {
		return
	}
	defer memd.Close()

	err = queryInt(memd, key(bid, "count"), &count)
	return
}

func (p *RemotePtt) GetArticleList(bid, offset int) (articles []Article, err error) {
	var memd *memcache.Connection
	if memd, err = memcache.Connect(p.BoarddAddr); err != nil {
		return
	}
	defer memd.Close()

	result, _, err := memd.Get(key(bid, "articles."+strconv.Itoa(offset)))
	if err != nil {
		return nil, err
	}

	articles = make([]Article, 0, 20)

	for _, line := range strings.Split(string(result), "\n") {
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
	var memd *memcache.Connection
	if memd, err = memcache.Connect(p.BoarddAddr); err != nil {
		return
	}
	defer memd.Close()

	content, _, err = memd.Get(key(bid, "article."+filename))
	if err != nil {
		return nil, err
	}
	return
}

func (p *RemotePtt) BrdName2Bid(brdname string) (bid int, err error) {
	var memd *memcache.Connection
	if memd, err = memcache.Connect(p.BoarddAddr); err != nil {
		return
	}
	defer memd.Close()

	err = queryInt(memd, "tobid."+brdname, &bid)
	return
}
