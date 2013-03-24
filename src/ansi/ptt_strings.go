package ansi

import (
	"bytes"
)

var quotePrefixStrings []string
var sigPrefixStrings []string

func quotePrefixes() []string {
	if quotePrefixStrings == nil {
		quotePrefixStrings = []string{": ", "> "}
	}
	return quotePrefixStrings
}

func sigPrefixes() []string {
	if sigPrefixStrings == nil {
		sigPrefixStrings = []string{"※", "==>"}
	}
	return sigPrefixStrings
}

func matchPrefixBytesToStrings(str []byte, patts []string) bool {
	for _, s := range patts {
		if bytes.HasPrefix(str, []byte(s)) {
			return true
		}
	}
	return false
}
