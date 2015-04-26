package richcontent

import (
	"regexp"
)

var (
	urlPattern = regexp.MustCompile(`(?:^|[^a-zA-Z0-9\-_])(https?://[^\s]+)`)
)

const (
	urlPatternSubmatchLeadIndex    = 0
	urlPatternSubmatchContentIndex = 1
)

func isTail(tail byte) bool {
	return tail == ')' || tail == ']' || tail == '}'
}

func matchingTail(lead byte) (tail byte, ok bool) {
	switch lead {
	case '(':
		tail = ')'
	case '[':
		tail = ']'
	case '{':
		tail = '}'
	default:
		return 0, false
	}
	ok = true
	return
}

// Test if a sequence of bytes has matching paren-like (), [], {} pairs.
func isParensMatched(input []byte, start, end int) bool {
	stack := make([]byte, 16)
	level := -1
	for i := start; i < end; i++ {
		if isTail(input[i]) {
			if level >= 0 && input[i] == stack[level] {
				level--
			} else {
				return false
			}
		} else if tail, ok := matchingTail(input[i]); ok {
			level++
			if level >= cap(stack) {
				// Too many levels
				return false
			}
			stack[level] = tail
		}
	}
	return level < 0
}

// If there is a ), ], } at the end of input, the function returns the range without it
// unless it is matched in the input sequence. Otherwise, it returns the original range.
func removeMatchingParenlikePairInUrl(input []byte, start, end int) (int, int) {
	if isTail(input[end-1]) && !isParensMatched(input, start, end) && end-1 >= start {
		return start, end - 1
	}
	return start, end
}

func FindAllUrls(input []byte) [][]byte {
	matches := urlPattern.FindAllSubmatch(input, -1)
	if len(matches) == 0 {
		return nil
	}

	urls := make([][]byte, len(matches))
	for i := range matches {
		content := matches[i][urlPatternSubmatchContentIndex]
		start, end := removeMatchingParenlikePairInUrl(content, 0, len(content))
		urls[i] = content[start:end]
	}
	return urls
}

// [[start, end], [start, end], ...]
func FindAllUrlsIndex(input []byte) [][]int {
	matches := urlPattern.FindAllSubmatchIndex(input, -1)
	for i := range matches {
		rng := matches[i][2*urlPatternSubmatchContentIndex : 2*urlPatternSubmatchContentIndex+2]
		rng[0], rng[1] = removeMatchingParenlikePairInUrl(input, rng[0], rng[1])
		matches[i] = rng
	}
	return matches
}
