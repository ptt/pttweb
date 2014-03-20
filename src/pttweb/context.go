package main

import (
	"net/http"
)

type Context struct {
	R *http.Request

	hasOver18Cookie bool
	isCrawler       bool
}

func (c *Context) MergeFromRequest(r *http.Request) error {
	c.R = r
	c.hasOver18Cookie = checkOver18Cookie(r)
	c.isCrawler = isCrawlerUserAgent(r)
	return nil
}

func (c *Context) IsOver18() bool {
	return c.hasOver18Cookie
}

func (c *Context) IsCrawler() bool {
	return c.isCrawler
}
