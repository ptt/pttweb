package main

import (
	"net/http"
	"strings"
)

var (
	// Whitelist crawlers here
	crawlerPatterns = [...]string{
		"Googlebot",
		"bingbot",
		"MSNbot",
		"facebookexternalhit",
		"PlurkBot",
	}
)

func isCrawlerUserAgent(r *http.Request) bool {
	ua := r.UserAgent()

	for _, pattern := range crawlerPatterns {
		if strings.Contains(ua, pattern) {
			return true
		}
	}

	return false
}
