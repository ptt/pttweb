package article

import "testing"

func TestRender(t *testing.T) {
	tests := []struct {
		desc     string
		input    string
		wantErr  error
		wantHTML string
	}{
		{
			desc:     "link crossing segments",
			input:    "\033[31mhttp://exam\033[32mple.com/ bar\033[m",
			wantHTML: `<a href="http://example.com/" target="_blank" rel="nofollow"><span class="f1">http://exam</span><span class="f2">ple.com/</span></a><span class="f2"> bar</span>`,
		},
		{
			desc:     "link spans 2 segments",
			input:    "\033[31mhttp://exam\033[32mple.com/",
			wantHTML: `<a href="http://example.com/" target="_blank" rel="nofollow"><span class="f1">http://exam</span><span class="f2">ple.com/</span></a>`,
		},
		{
			desc:     "link at beginning of a segment",
			input:    "\033[31mhttp://example.com/ bar\033[m",
			wantHTML: `<span class="f1"><a href="http://example.com/" target="_blank" rel="nofollow">http://example.com/</a> bar</span>`,
		},
	}
	for _, test := range tests {
		ra, err := Render(WithContent([]byte(test.input)), WithDisableArticleHeader())
		if err != test.wantErr {
			t.Errorf("%v: Render(test.input) = _, %v; want _, %v", test.desc, err, test.wantErr)
			continue
		} else if err != nil {
			continue
		}
		if got, want := string(ra.HTML()), test.wantHTML; got != want {
			t.Errorf("%v: ra.HTML():\ngot  = %v\nwant = %v", test.desc, got, want)
		}
	}
}
