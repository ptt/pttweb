package article

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

type RichContent struct {
	URLString   string
	ContentHtml string
}

type UrlPatternHandler func(urlBytes []byte, match [][]byte) ([]*RichContent, error)

type UrlPattern struct {
	Pattern *regexp.Regexp
	Handler UrlPatternHandler
}

var defaultPatterns = []*UrlPattern{
	NewUrlPattern(`^https?://(?:www\.youtube\.com/watch\?(?:.+&)*v=|youtu\.be/)([\w\-]+)`, handleYoutube),
	NewUrlPattern(`^https?://imgur\.com/([,\w]+)(?:\#(\d+))?[^/]*$`, handleImgur),
	NewUrlPattern(`^http://picmoe\.net/d\.php\?id=(\d+)`, handlePicmoe),
	NewUrlPattern(`\.(?i:png|jpg|gif)$`, handleGenericImage),
}

func NewUrlPattern(pattern string, handler UrlPatternHandler) *UrlPattern {
	return &UrlPattern{
		Pattern: regexp.MustCompile(pattern),
		Handler: handler,
	}
}

func FindRichContents(input []byte) ([]*RichContent, error) {
	rcs := make([]*RichContent, 0, 4)
	for _, u := range FindAllUrls(input) {
		for _, p := range defaultPatterns {
			if match := p.Pattern.FindSubmatch(u); match != nil {
				if rc, err := p.Handler(u, match); err == nil {
					rcs = append(rcs, rc...)
				}
				break
			}
		}
	}
	return rcs, nil
}

func imageHtmlTag(urlString string) string {
	return fmt.Sprintf(`<img src="%s" alt="" />`, html.EscapeString(urlString))
}

// Handlers

func handleYoutube(urlBytes []byte, match [][]byte) ([]*RichContent, error) {
	return []*RichContent{
		&RichContent{
			URLString: string(match[0]),
			ContentHtml: fmt.Sprintf(
				`<iframe class="youtube-player" type="text/html" width="640" height="385" src="//www.youtube.com/embed/%s" frameborder="0"></iframe>`,
				string(match[1])),
		},
	}, nil
}

func handleImgur(urlBytes []byte, match [][]byte) ([]*RichContent, error) {
	rcs := make([]*RichContent, 0, 4)
	for _, id := range strings.Split(string(match[1]), ",") {
		link := fmt.Sprintf(`//i.imgur.com/%s.jpg`, id)
		rcs = append(rcs, &RichContent{
			URLString:   link,
			ContentHtml: imageHtmlTag(link),
		})
	}
	return rcs, nil
}

func handlePicmoe(urlBytes []byte, match [][]byte) ([]*RichContent, error) {
	link := fmt.Sprintf(`http://picmoe.net/src/%ss.jpg`, string(match[1]))
	return []*RichContent{
		&RichContent{
			URLString:   link,
			ContentHtml: imageHtmlTag(link),
		},
	}, nil
}

func handleGenericImage(urlBytes []byte, match [][]byte) ([]*RichContent, error) {
	return []*RichContent{
		&RichContent{
			URLString:   string(urlBytes),
			ContentHtml: imageHtmlTag(string(urlBytes)),
		},
	}, nil
}
