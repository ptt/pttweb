package main

import (
	"sync"
	"time"

	"github.com/ptt/pttweb/pttbbs"
)

type brdCacheEntry struct {
	Board  *pttbbs.Board
	Expire time.Time
}

var (
	brdCache   = make(map[string]*brdCacheEntry)
	brdCacheLk sync.Mutex
)

const (
	brdCacheExpire = time.Minute * 5
)

func getBrdCache(brdname string) *pttbbs.Board {
	brdCacheLk.Lock()
	defer brdCacheLk.Unlock()

	entry := brdCache[brdname]
	if entry != nil {
		if time.Now().Before(entry.Expire) {
			return entry.Board
		} else {
			delete(brdCache, brdname)
			return nil
		}
	}
	return nil
}

func setBrdCache(brdname string, board *pttbbs.Board) {
	brdCacheLk.Lock()
	defer brdCacheLk.Unlock()

	brdCache[brdname] = &brdCacheEntry{
		Board:  board,
		Expire: time.Now().Add(brdCacheExpire),
	}
}

func getBoardByNameCached(brdname string) (*pttbbs.Board, error) {
	if brd := getBrdCache(brdname); brd != nil {
		return brd, nil
	}

	bid, err := ptt.BrdName2Bid(brdname)
	if err == pttbbs.ErrTooBusy {
		return nil, err
	} else if err != nil {
		return nil, NewNotFoundErrorPage(err)
	}

	board, err := ptt.GetBoard(bid)
	if err != nil {
		return nil, err
	}

	setBrdCache(brdname, &board)
	return &board, nil
}
