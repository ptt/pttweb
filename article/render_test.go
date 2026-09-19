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
		{
			desc:     "SGR 66 dual-color character transition and reset",
			input:    "\033[31m前\033[66;32m雙後續\033[66;m終",
			wantHTML: `<span class="f1">前</span><span class="o f1 b0 rf2 rb0" data-text="雙">雙</span><span class="f2">後續</span><span class="o f2 b0 rf7 rb0" data-text="終">終</span>`,
		},
		{
			desc:     "SGR 66 single-sequence left and right colors",
			input:    "\033[1;31;66;0;32m雙\033[m",
			wantHTML: `<span class="o f1 b0 hl rf2 rb0" data-text="雙">雙</span>`,
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
