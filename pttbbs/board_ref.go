package pttbbs

import apipb "github.com/ptt/pttweb/proto/api"

type boardRefByBid BoardID

func (r boardRefByBid) boardRef() *apipb.BoardRef {
	return &apipb.BoardRef{Ref: &apipb.BoardRef_Bid{uint32(r)}}
}

func BoardRefByBid(bid BoardID) BoardRef {
	return boardRefByBid(bid)
}

func BoardRefsByBid(bids []BoardID) []BoardRef {
	refs := make([]BoardRef, len(bids))
	for i := range bids {
		refs[i] = BoardRefByBid(bids[i])
	}
	return refs
}

type boardRefByName string

func (r boardRefByName) boardRef() *apipb.BoardRef {
	return &apipb.BoardRef{Ref: &apipb.BoardRef_Name{string(r)}}
}

func BoardRefByName(name string) BoardRef {
	return boardRefByName(name)
}
