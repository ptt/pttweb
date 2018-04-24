package pttbbs

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ArticleFirstLineRegexp = regexp.MustCompile(`^(.+?): (.+) (.+?): (.+?)\n$`)
	ArticleMetaLineRegexp  = regexp.MustCompile(`^(.+?): (.+)\n$`)

	QuotePrefixStrings     = []string{": ", "> "}
	SignaturePrefixStrings = []string{"※", "==>"}

	ArticlePushPrefixStrings = []string{"推 ", "噓 ", "→ "}

	subjectPrefixStrings = []string{"re:", "fw:", "[轉錄]"}
)

const (
	ArticleAuthor = "作者"
	ArticleTitle  = "標題"
)

var (
	validBrdNameRegexp  = regexp.MustCompile(`^[0-9a-zA-Z][0-9a-zA-Z_\.\-]+$`)
	validFileNameRegexp = regexp.MustCompile(`^[MG]\.\d+\.A(\.[0-9A-F]+)?$`)
	validUserIDRegexp   = regexp.MustCompile(`^[a-zA-Z][0-9a-zA-Z]{1,11}$`)
	fileNameTimeRegexp  = regexp.MustCompile(`^[MG]\.(\d+)\.A(\.[0-9A-F]+)?$`)
)

func IsValidBrdName(brdname string) bool {
	return validBrdNameRegexp.MatchString(brdname)
}

func IsValidArticleFileName(filename string) bool {
	return validFileNameRegexp.MatchString(filename)
}

func IsValidUserID(userID string) bool {
	return validUserIDRegexp.MatchString(userID)
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

func ParseFileNameTime(filename string) (time.Time, error) {
	m := fileNameTimeRegexp.FindStringSubmatch(filename)
	if len(m) == 0 {
		return time.Time{}, errors.New("invalid filename pattern")
	}
	unix, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(unix), 0), nil
}

func Subject(subject string) string {
	lower := strings.ToLower(subject)
	off := 0
	for _, p := range subjectPrefixStrings {
		for strings.HasPrefix(lower[off:], p) {
			off += len(p)
			off += countPrefixSpaces(lower[off:])
		}
		off += countPrefixSpaces(lower[off:])
	}
	return subject[off:]
}

func countPrefixSpaces(s string) int {
	for i, c := range s {
		if c != ' ' {
			return i
		}
	}
	return 0
}
