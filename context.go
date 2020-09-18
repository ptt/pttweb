package main

import (
	"net/http"
)

type Context struct {
	R *http.Request

	skipOver18      bool
	hasOver18Cookie bool
	isCrawler       bool
}

func (c *Context) Request() *http.Request {
	return c.R
}

func (c *Context) MergeFromRequest(r *http.Request) error {
	c.R = r
	c.hasOver18Cookie = checkOver18Cookie(r)
	c.isCrawler = isCrawlerUserAgent(r)
	return nil
}

func (c *Context) SetSkipOver18() {
	c.skipOver18 = true
}

func (c *Context) IsOver18() bool {
	return c.hasOver18Cookie || c.skipOver18
}

func (c *Context) IsCrawler() bool {
	return c.isCrawler
}
