package pttbbs

import (
	"context"
	"fmt"
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

func (p *GrpcRemotePtt) getBoard(ref *apipb.BoardRef) (*apipb.Board, error) {
	rep, err := p.service.Board(context.TODO(), &apipb.BoardRequest{
		Ref: []*apipb.BoardRef{ref},
	}, grpcCallOpts...)
	if err != nil {
		return nil, err
	}
	if len(rep.Boards) != 1 {
		return nil, fmt.Errorf("%v boards received, expected 1", len(rep.Boards))
	}
	return rep.Boards[0], nil
}

func boardRefByBid(bid int) *apipb.BoardRef {
	return &apipb.BoardRef{Ref: &apipb.BoardRef_Bid{int32(bid)}}
}

func boardRefByName(name string) *apipb.BoardRef {
	return &apipb.BoardRef{Ref: &apipb.BoardRef_Name{name}}
}

func (p *GrpcRemotePtt) BrdName2Bid(brdname string) (int, error) {
	b, err := p.getBoard(boardRefByName(brdname))
	if err != nil {
		return 0, err
	}
	return int(b.Bid), nil
}

func (p *GrpcRemotePtt) GetBoard(bid int) (Board, error) {
	b, err := p.getBoard(boardRefByBid(bid))
	if err != nil {
		return Board{}, err
	}
	return toBoard(b), nil
}

func (p *GrpcRemotePtt) GetBoards(bids []int) ([]Board, error) {
	refs := make([]*apipb.BoardRef, len(bids))
	for i, bid := range bids {
		refs[i] = boardRefByBid(bid)
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
		Bid:     int(b.Bid),
		IsBoard: !hasFlag(b.Attributes, BoardGroup),
		Over18:  hasFlag(b.Attributes, BoardOver18),
		Hidden:  false, // All returned boards are public.
		BrdName: b.Name,
		Title:   b.Title,
		Class:   b.Bclass,
		BM:      b.RawModerators,
		Parent:  int(b.Parent),
		Nuser:   int(b.NumUsers),
	}
}

func hasFlag(bits, mask uint32) bool {
	return (bits & mask) == mask
}

func (p *GrpcRemotePtt) GetBoardChildren(bid int) ([]int, error) {
	b, err := p.getBoard(boardRefByBid(bid))
	if err != nil {
		return nil, err
	}
	children := make([]int, len(b.Children))
	for i, c := range b.Children {
		children[i] = int(c)
	}
	return children, nil
}

func (p *GrpcRemotePtt) GetArticleCount(bid int) (int, error) {
	b, err := p.getBoard(boardRefByBid(bid))
	if err != nil {
		return 0, err
	}
	return int(b.NumPosts), nil
}

func (p *GrpcRemotePtt) GetArticleList(bid, offset int) ([]Article, error) {
	rep, err := p.service.List(context.TODO(), &apipb.ListRequest{
		Ref:          boardRefByBid(bid),
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

func (p *GrpcRemotePtt) GetBottomList(bid int) ([]Article, error) {
	rep, err := p.service.List(context.TODO(), &apipb.ListRequest{
		Ref:            boardRefByBid(bid),
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

func (p *GrpcRemotePtt) GetArticleContent(bid int, filename string) (content []byte, err error) {
	a, err := p.GetArticleSelect(bid, SelectPart, filename, "", 0, -1)
	if err != nil {
		return nil, err
	}
	return a.Content, nil
}

func (p *GrpcRemotePtt) GetArticleSelect(bid int, meth SelectMethod, filename, cacheKey string, offset, maxlen int) (*ArticlePart, error) {
	rep, err := p.service.Content(context.TODO(), &apipb.ContentRequest{
		BoardRef:         boardRefByBid(bid),
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
