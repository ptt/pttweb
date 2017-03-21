package pttbbs

import (
	"context"
	"time"

	"google.golang.org/grpc"

	apipb "github.com/ptt/pttweb/proto/api"
)

var grpcCallOpts = []grpc.CallOption{grpc.FailFast(true)}

type GrpcRemotePtt struct {
	service apipb.BoardServiceClient
}

func NewGrpcRemotePtt(boarddAddr string) (*GrpcRemotePtt, error) {
	conn, err := grpc.Dial(boarddAddr, grpc.WithInsecure(), grpc.WithBackoffMaxDelay(time.Second*5))
	if err != nil {
		return nil, err
	}
	return &GrpcRemotePtt{
		service: apipb.NewBoardServiceClient(conn),
	}, nil
}

func (p *GrpcRemotePtt) GetBoards(brefs ...BoardRef) ([]Board, error) {
	refs := make([]*apipb.BoardRef, len(brefs))
	for i, ref := range brefs {
		refs[i] = ref.boardRef()
	}
	rep, err := p.service.Board(context.TODO(), &apipb.BoardRequest{
		Ref: refs,
	}, grpcCallOpts...)
	if err != nil {
		return nil, err
	}
	boards := make([]Board, len(rep.Boards))
	for i, b := range rep.Boards {
		boards[i] = toBoard(b)
	}
	return boards, nil
}

func toBoard(b *apipb.Board) Board {
	return Board{
		Bid:      BoardID(b.Bid),
		IsBoard:  !hasFlag(b.Attributes, BoardGroup),
		Over18:   hasFlag(b.Attributes, BoardOver18),
		Hidden:   false, // All returned boards are public.
		BrdName:  b.Name,
		Title:    b.Title,
		Class:    b.Bclass,
		BM:       b.RawModerators,
		Parent:   int(b.Parent),
		Nuser:    int(b.NumUsers),
		NumPosts: int(b.NumPosts),
		Children: toBoardIDs(b.Children),
	}
}

func toBoardIDs(bids []uint32) []BoardID {
	out := make([]BoardID, len(bids))
	for i := range bids {
		out[i] = BoardID(bids[i])
	}
	return out
}

func hasFlag(bits, mask uint32) bool {
	return (bits & mask) == mask
}

func (p *GrpcRemotePtt) GetArticleList(ref BoardRef, offset int) ([]Article, error) {
	rep, err := p.service.List(context.TODO(), &apipb.ListRequest{
		Ref:          ref.boardRef(),
		IncludePosts: true,
		Offset:       int32(offset),
		Length:       20,
	}, grpcCallOpts...)
	if err != nil {
		return nil, err
	}
	var articles []Article
	for _, a := range rep.Posts {
		articles = append(articles, toArticle(a))
	}
	return articles, nil
}

func (p *GrpcRemotePtt) GetBottomList(ref BoardRef) ([]Article, error) {
	rep, err := p.service.List(context.TODO(), &apipb.ListRequest{
		Ref:            ref.boardRef(),
		IncludeBottoms: true,
	}, grpcCallOpts...)
	if err != nil {
		return nil, err
	}
	var articles []Article
	for _, a := range rep.Bottoms {
		articles = append(articles, toArticle(a))
	}
	return articles, nil
}

func toArticle(p *apipb.Post) Article {
	return Article{
		Offset:    int(p.Index),
		FileName:  p.Filename,
		Date:      p.RawDate,
		Recommend: int(p.NumRecommends),
		FileMode:  int(p.Filemode),
		Owner:     p.Owner,
		Title:     p.Title,
		Modified:  time.Unix(0, p.ModifiedNsec),
	}
}

func (p *GrpcRemotePtt) GetArticleSelect(ref BoardRef, meth SelectMethod, filename, cacheKey string, offset, maxlen int) (*ArticlePart, error) {
	rep, err := p.service.Content(context.TODO(), &apipb.ContentRequest{
		BoardRef:         ref.boardRef(),
		Filename:         filename,
		ConsistencyToken: cacheKey,
		PartialOptions: &apipb.PartialOptions{
			SelectType: toSelectType(meth),
			Offset:     int64(offset),
			MaxLength:  int64(maxlen),
		},
	}, grpcCallOpts...)
	if err != nil {
		return nil, err
	}
	return toArticlePart(rep.Content), nil
}

func toSelectType(m SelectMethod) apipb.PartialOptions_SelectType {
	switch m {
	case SelectPart:
		return apipb.PartialOptions_SELECT_PART
	case SelectHead:
		return apipb.PartialOptions_SELECT_HEAD
	case SelectTail:
		return apipb.PartialOptions_SELECT_TAIL
	default:
		panic("unhandled select type")
	}
}

func toArticlePart(c *apipb.Content) *ArticlePart {
	return &ArticlePart{
		CacheKey: c.ConsistencyToken,
		FileSize: int(c.TotalLength),
		Offset:   int(c.Offset),
		Length:   int(c.Length),
		Content:  c.Content,
	}
}

func (p *GrpcRemotePtt) Hotboards() ([]Board, error) {
	rep, err := p.service.Hotboard(context.TODO(), &apipb.HotboardRequest{}, grpcCallOpts...)
	if err != nil {
		return nil, err
	}
	var boards []Board
	for _, b := range rep.Boards {
		boards = append(boards, toBoard(b))
	}
	return boards, nil
}
