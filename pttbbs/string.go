package pttbbs

import (
	"bytes"
	"regexp"
)

var (
	ArticleFirstLineRegexp = regexp.MustCompile(`^(.+?): (.+) (.+?): (.+?)\n$`)
	ArticleMetaLineRegexp  = regexp.MustCompile(`^(.+?): (.+)\n$`)

	QuotePrefixStrings     = []string{": ", "> "}
	SignaturePrefixStrings = []string{"※", "==>"}

	ArticlePushPrefixStrings = []string{"推 ", "噓 ", "→ "}
)

const (
	ArticleAuthor = "作者"
	ArticleTitle  = "標題"
)

var (
	validBrdNameRegexp = regexp.MustCompile(`^[0-9a-zA-Z][0-9a-zA-Z_\.\-]+$`)
)

func IsValidBrdName(brdname string) bool {
	return validBrdNameRegexp.MatchString(brdname)
}

func ParseArticleFirstLine(line []byte) (tag1, val1, tag2, val2 []byte, ok bool) {
	m := ArticleFirstLineRegexp.FindSubmatch(line)
	if m == nil {
		ok = false
	} else {
		tag1, val1, tag2, val2 = m[1], m[2], m[3], m[4]
		ok = true
	}
	return
}

func ParseArticleMetaLine(line []byte) (tag, val []byte, ok bool) {
	m := ArticleMetaLineRegexp.FindSubmatch(line)
	if m == nil {
		ok = false
	} else {
		tag, val = m[1], m[2]
		ok = true
	}
	return
}

func MatchPrefixBytesToStrings(str []byte, patts []string) bool {
	for _, s := range patts {
		if bytes.HasPrefix(str, []byte(s)) {
			return true
		}
	}
	return false
}
