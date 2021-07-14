package main

import (
	"bytes"
	"regexp"

	"github.com/ptt/pttweb/pttbbs"
	"github.com/ptt/pttweb/richcontent"

	"golang.org/x/net/context"
)

var aidPatterns = []struct {
	Pattern *regexp.Regexp
	Handler func(ctx context.Context, input []byte, m richcontent.MatchIndices) (link string, err error)
}{
	{
		Pattern: regexp.MustCompile(`([0-9A-Za-z\-_]{1,12}) 看板 #([0-9A-Za-z\-_\@]{8,10})`),
		Handler: handleBoardAidText,
	},
	{
		Pattern: regexp.MustCompile(`#([0-9A-Za-z\-_\@]{8,10}) \(([0-9A-Za-z\-_]{1,12})\)`),
		Handler: handleAidBoardText,
	},
	{
		Pattern: regexp.MustCompile(`#([0-9A-Za-z\-_\@]{8,10})`),
		Handler: handleAidText,
	},
}

func init() {
	richcontent.RegisterFinder(findAidText)
}

func findAidText(ctx context.Context, input []byte) (rcs []richcontent.RichContent, err error) {
	// Fast path.
	if bytes.IndexByte(input, '#') < 0 {
		return nil, nil
	}

	for _, p := range aidPatterns {
		all := p.Pattern.FindAllSubmatchIndex(input, -1)
		for _, m := range all {
			link, err := p.Handler(ctx, input, richcontent.MatchIndices(m))
			if err != nil {
				return nil, err
			}
			if link != "" {
				rcs = append(rcs, richcontent.MakeRichContent(m[0], m[1], link, nil))
			}
		}
	}
	return rcs, nil
}

type boardname interface {
	Boardname() string
}

func handleAidText(ctx context.Context, input []byte, m richcontent.MatchIndices) (string, error) {
	bn, ok := ctx.Value(CtxKeyBoardname).(boardname)
	if !ok {
		return "", nil // Silently fail
	}

	aidString := string(m.ByteSliceOf(input, 1))
	return aidAndBrdnameToArticle(bn.Boardname(), aidString)
}

func handleAidBoardText(ctx context.Context, input []byte, m richcontent.MatchIndices) (string, error) {
	aidString := string(m.ByteSliceOf(input, 1))
	brdname := string(m.ByteSliceOf(input, 2))
	return aidAndBrdnameToArticle(brdname, aidString)
}

func handleBoardAidText(ctx context.Context, input []byte, m richcontent.MatchIndices) (string, error) {
	brdname := string(m.ByteSliceOf(input, 1))
	aidString := string(m.ByteSliceOf(input, 2))
	return aidAndBrdnameToArticle(brdname, aidString)
}

func aidAndBrdnameToArticle(brdname, aidString string) (string, error) {
	aid, err := pttbbs.ParseAid(aidString)
	if err != nil {
		return "", nil // Silently fail
	}

	u, err := router.Get("bbsarticle").URLPath("brdname", brdname, "filename", aid.Filename())
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
