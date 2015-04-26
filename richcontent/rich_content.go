package richcontent

import (
	"sort"

	"golang.org/x/net/context"
)

type Component interface {
	HTML() string
}

type RichContent interface {
	Pos() (int, int)
	URLString() string
	Components() []Component
}

type Finder func(ctx context.Context, input []byte) ([]RichContent, error)

var defaultFinders = []Finder{
	FindUrl,
}

func RegisterFinder(finder Finder) {
	defaultFinders = append(defaultFinders, finder)
}

func Find(ctx context.Context, input []byte) ([]RichContent, error) {
	var rcs []RichContent
	for _, f := range defaultFinders {
		found, err := f(ctx, input)
		if err != nil {
			return nil, err
		}
		rcs = append(rcs, found...)
	}
	sort.Sort(RichContentByBeginThenLongest(rcs))

	var filtered []RichContent
	left := 0
	for _, rc := range rcs {
		l, r := rc.Pos()
		if left <= l {
			left = r
			filtered = append(filtered, rc)
		}
	}
	return filtered, nil
}

type RichContentByBeginThenLongest []RichContent

func (c RichContentByBeginThenLongest) Len() int      { return len(c) }
func (c RichContentByBeginThenLongest) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c RichContentByBeginThenLongest) Less(i, j int) bool {
	ib, ie := c[i].Pos()
	jb, je := c[j].Pos()
	switch {
	case ib < jb:
		return true
	case ib > jb:
		return false
	default:
		return je < ie
	}
}

type simpleRichComponent struct {
	begin, end int
	urlString  string
	components []Component
}

func (c *simpleRichComponent) Pos() (int, int)         { return c.begin, c.end }
func (c *simpleRichComponent) URLString() string       { return c.urlString }
func (c *simpleRichComponent) Components() []Component { return c.components }

func MakeRichContent(begin, end int, urlString string, components []Component) RichContent {
	return &simpleRichComponent{
		begin:      begin,
		end:        end,
		urlString:  urlString,
		components: components,
	}
}

type simpleComponent struct {
	html string
}

func (c *simpleComponent) HTML() string { return c.html }

func MakeComponent(html string) Component {
	return &simpleComponent{html: html}
}

type MatchIndices []int

func (m MatchIndices) Len() int                           { return len(m) / 2 }
func (m MatchIndices) At(i int) (int, int)                { return m[2*i], m[2*i+1] }
func (m MatchIndices) ByteSliceOf(b []byte, i int) []byte { return b[m[2*i]:m[2*i+1]] }
