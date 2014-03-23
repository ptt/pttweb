package main

import (
	"net/http"
)

const (
	Over18CookieName = "over18"
)

func checkOver18Cookie(r *http.Request) bool {
	if cookie, err := r.Cookie(Over18CookieName); err == nil {
		return cookie.Value != ""
	}
	return false
}

func setOver18Cookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:  Over18CookieName,
		Value: "1",
		Path:  "/",
	})
}
