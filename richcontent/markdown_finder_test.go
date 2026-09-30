package richcontent

import (
	"testing"

	"golang.org/x/net/context"
)

func TestFindMarkdownUrl(t *testing.T) {
	ctx := context.TODO()

	// 1. Basic match
	rcs, err := FindMarkdownUrl(ctx, []byte("Visit [PTT](https://term.ptt.cc) now!"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rcs) != 1 {
		t.Fatalf("expected 1 match, got %d", len(rcs))
	}
	rc := rcs[0]
	if b, e := rc.Pos(); b != 6 || e != 32 {
		t.Errorf("Pos() = (%d, %d), want (6, 32)", b, e)
	}
	if tr, ok := rc.(TextPosRichContent); !ok {
		t.Errorf("expected TextPosRichContent")
	} else if tb, te := tr.TextPos(); tb != 7 || te != 10 {
		t.Errorf("TextPos() = (%d, %d), want (7, 10)", tb, te)
	}
	if u := rc.URLString(); u != "https://term.ptt.cc" {
		t.Errorf("URLString() = %q, want %q", u, "https://term.ptt.cc")
	}

	// 2. Chinese text
	rcs, err = FindMarkdownUrl(ctx, []byte("看 [說明文件](https://ptt.cc/doc)"))
	if err != nil || len(rcs) != 1 {
		t.Fatalf("expected 1 match, got %d (err: %v)", len(rcs), err)
	}
	if tr, ok := rcs[0].(TextPosRichContent); ok {
		tb, te := tr.TextPos()
		if string([]byte("看 [說明文件](https://ptt.cc/doc)")[tb:te]) != "說明文件" {
			t.Errorf("text = %q, want %q", string([]byte("看 [說明文件](https://ptt.cc/doc)")[tb:te]), "說明文件")
		}
	}

	// 3. Negative tests
	negatives := []string{
		"[]()",
		"[text]()",
		"[](https://ptt.cc)",
		"[   ](https://ptt.cc)",
		"[text] (https://ptt.cc)",
		"[evil](javascript:alert(1))",
		"[evil](file:///etc/passwd)",
		"[問卦] 今日天氣真好",
		"plain text without links",
	}
	for _, neg := range negatives {
		rcs, err := FindMarkdownUrl(ctx, []byte(neg))
		if err != nil {
			t.Errorf("unexpected error on %q: %v", neg, err)
		}
		if len(rcs) != 0 {
			t.Errorf("expected 0 matches for %q, got %d", neg, len(rcs))
		}
	}

	// 4. Overlap with FindUrl in richcontent.Find
	allRcs, err := Find(ctx, []byte("Check [PTT](https://term.ptt.cc) and http://example.com"))
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(allRcs) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(allRcs))
	}
	if allRcs[0].URLString() != "https://term.ptt.cc" {
		t.Errorf("allRcs[0] URL = %q, want %q", allRcs[0].URLString(), "https://term.ptt.cc")
	}
	if _, ok := allRcs[0].(TextPosRichContent); !ok {
		t.Errorf("allRcs[0] should be TextPosRichContent")
	}
	if allRcs[1].URLString() != "http://example.com" {
		t.Errorf("allRcs[1] URL = %q, want %q", allRcs[1].URLString(), "http://example.com")
	}
}
