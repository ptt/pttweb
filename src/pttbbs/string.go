package pttbbs

import (
	"regexp"
)

const (
	ArticleTitlePrefix = "標題: "
)

var validBrdNameRegexp *regexp.Regexp

func IsValidBrdName(brdname string) bool {
	if validBrdNameRegexp == nil {
		validBrdNameRegexp = regexp.MustCompile("^[a-zA-Z0-9_\\-]+$")
	}
	return validBrdNameRegexp.MatchString(brdname)
}
