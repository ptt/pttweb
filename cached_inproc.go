package main

import (
	"strings"
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

	brdname = strings.ToLower(brdname)
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

	brdCache[strings.ToLower(brdname)] = &brdCacheEntry{
		Board:  board,
		Expire: time.Now().Add(brdCacheExpire),
	}
}

func getBoardByNameCached(brdname string) (*pttbbs.Board, error) {
	if brd := getBrdCache(brdname); brd != nil {
		return brd, nil
	}

	board, err := pttbbs.OneBoard(ptt.GetBoards(pttbbs.BoardRefByName(brdname)))
	if err != nil {
		return nil, err
	}

	setBrdCache(brdname, &board)
	return &board, nil
}
