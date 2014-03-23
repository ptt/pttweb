package main

import (
	"fmt"
	"net/http"
	"net/url"
)

type RedirectErrorPage struct {
	To string
}

func (e *RedirectErrorPage) Error() string {
	return fmt.Sprintf("redirect error page to: %v", e.To)
}

func (e *RedirectErrorPage) EmitErrorPage(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Location", e.To)
	w.WriteHeader(http.StatusFound)
	return nil
}

type NotFoundErrorPage struct {
	Err error
}

func (e *NotFoundErrorPage) Error() string {
	return fmt.Sprintf("not found error page: %v", e.Err)
}

func (e *NotFoundErrorPage) EmitErrorPage(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusNotFound)
	return tmpl["notfound.html"].Execute(w, nil)
}

func NewNotFoundErrorPage(err error) *NotFoundErrorPage {
	return &NotFoundErrorPage{
		Err: err,
	}
}

func isSafeRedirectURI(uri string) bool {
	if len(uri) < 1 || uri[0] != '/' {
		return false
	}
	u, err := url.Parse(uri)
	return err == nil && u.Scheme == "" && u.User == nil && u.Host == ""
}
