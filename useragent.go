package main

import (
	"net/http"
	"strings"
)

var (
	// Whitelist crawlers here
	crawlerPatterns = [...]string{
		"Google (+https://developers.google.com/+/web/snippet/)",
		"Googlebot",
		"bingbot",
		"MSNbot",
		"facebookexternalhit",
		"PlurkBot",
		"Twitterbot",
		"TelegramBot",
		"CloudFlare-AlwaysOnline",
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
