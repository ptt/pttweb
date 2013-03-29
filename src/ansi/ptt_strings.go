package ansi

import (
	"bytes"
)

var (
	quotePrefixStrings = []string{": ", "> "}
	sigPrefixStrings   = []string{"※", "==>"}
)

func matchPrefixBytesToStrings(str []byte, patts []string) bool {
	for _, s := range patts {
		if bytes.HasPrefix(str, []byte(s)) {
			return true
		}
	}
	return false
}
