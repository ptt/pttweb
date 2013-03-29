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

var urlPattern = regexp.MustCompile(`https?://[^\s]+`)
var defaultPatterns = []*UrlPattern{
	NewUrlPattern(`^https?://(?:www\.youtube\.com/watch\?(?:.+&)*v=|youtu\.be/)([\w\-]+)`, handleYoutube),
	NewUrlPattern(`^http://v\.youku\.com/v_show/id_(\w+)\.html`, handleYouku),
	NewUrlPattern(`^https?://imgur\.com/([,\w]+)(?:\#(\d+))?[^/]*$`, handleImgur),
	NewUrlPattern(`^http://picmoe\.net/d\.php\?id=(\d+)`, handlePicmoe),
	NewUrlPattern(`\.(?:png|jpg|gif)$`, handleGenericImage),
}

func NewUrlPattern(pattern string, handler UrlPatternHandler) *UrlPattern {
	return &UrlPattern{
		Pattern: regexp.MustCompile(pattern),
		Handler: handler,
	}
}

func FindUrls(input []byte) [][]byte {
	urls := urlPattern.FindAll(input, -1)
	if urls == nil {
		return [][]byte{}
	}
	return urls
}

func FindRichContents(input []byte) ([]*RichContent, error) {
	rcs := make([]*RichContent, 0, 4)
	for _, u := range FindUrls(input) {
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
				`<iframe class="youtube-player" type="text/html" width="640" height="385" src="http://www.youtube.com/embed/%s" frameborder="0"></iframe>`,
				string(match[1])),
		},
	}, nil
}

func handleYouku(urlBytes []byte, match [][]byte) ([]*RichContent, error) {
	return []*RichContent{
		&RichContent{
			URLString: string(match[0]),
			ContentHtml: fmt.Sprintf(
				`<embed src="http://player.youku.com/player.php/sid/%s/v.swf" quality="high" width="480" height="400" align="middle" allowScriptAccess="sameDomain" allowFullscreen="true" type="application/x-shockwave-flash"></embed>`,
				string(match[1])),
		},
	}, nil
}

func handleImgur(urlBytes []byte, match [][]byte) ([]*RichContent, error) {
	rcs := make([]*RichContent, 0, 4)
	for _, id := range strings.Split(string(match[1]), ",") {
		link := fmt.Sprintf(`http://i.imgur.com/%s.jpg`, id)
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
