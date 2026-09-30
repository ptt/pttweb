package richcontent

import (
	"bytes"
	"regexp"

	"golang.org/x/net/context"
)

var (
	markdownUrlPattern = regexp.MustCompile(`\[([^\[\r\n]+)\]\(((?i:https?)://[^\s\(\)]+)\)`)
)

func FindMarkdownUrl(ctx context.Context, input []byte) ([]RichContent, error) {
	// Fast path.
	if bytes.IndexByte(input, '[') < 0 || bytes.IndexByte(input, ']') < 0 || bytes.IndexByte(input, '(') < 0 {
		return nil, nil
	}

	matches := markdownUrlPattern.FindAllSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	rcs := make([]RichContent, 0, len(matches))
	for _, m := range matches {
		textBytes := input[m[2]:m[3]]
		if len(bytes.TrimSpace(textBytes)) == 0 {
			continue
		}
		urlBytes := input[m[4]:m[5]]
		var components []Component
		for _, p := range defaultUrlPatterns {
			if match := p.Pattern.FindSubmatchIndex(urlBytes); match != nil {
				if c, err := p.Handler(ctx, urlBytes, MatchIndices(match)); err == nil {
					components = c
				}
				break
			}
		}
		rcs = append(rcs, MakeMarkdownRichContent(m[0], m[1], m[2], m[3], string(urlBytes), components))
	}
	return rcs, nil
}
