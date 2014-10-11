package main

import (
	"fmt"
	"net/url"

	"github.com/ptt/pttweb/page"
)

type ShouldAskOver18Error struct {
	page.Redirect
}

func (e *ShouldAskOver18Error) Error() string {
	return fmt.Sprintf("should ask over18, redirect: %v", e.To)
}

type NotFoundError struct {
	page.NotFound
	UnderlyingErr error
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found error page: %v", e.UnderlyingErr)
}

func NewNotFoundError(err error) *NotFoundError {
	return &NotFoundError{
		UnderlyingErr: err,
	}
}

func isSafeRedirectURI(uri string) bool {
	if len(uri) < 1 || uri[0] != '/' {
		return false
	}
	u, err := url.Parse(uri)
	return err == nil && u.Scheme == "" && u.User == nil && u.Host == ""
}
