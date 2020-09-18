package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

var routesSkipOver18 = map[string]bool{
	"atomfeed": true,
}

type Context struct {
	R *http.Request

	isOver18CheckSkipped bool
	hasOver18Cookie      bool
	isCrawler            bool
}

func (c *Context) Request() *http.Request {
	return c.R
}

func (c *Context) MergeFromRequest(r *http.Request) error {
	c.R = r
	_, c.isOver18CheckSkipped = routesSkipOver18[mux.CurrentRoute(r).GetName()]
	c.hasOver18Cookie = checkOver18Cookie(r)
	c.isCrawler = isCrawlerUserAgent(r)
	return nil
}

func (c *Context) IsOver18CheckSkipped() bool {
	return c.isOver18CheckSkipped
}

func (c *Context) IsOver18() bool {
	return c.hasOver18Cookie
}

func (c *Context) IsCrawler() bool {
	return c.isCrawler
}
