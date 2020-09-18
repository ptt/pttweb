package page

import (
	"net/http"
)

type Context interface {
	Request() *http.Request
}

type context struct {
	req *http.Request
}

func newContext(req *http.Request) (*context, error) {
	return &context{
		req: req,
	}, nil
}

func (c *context) Request() *http.Request {
	return c.req
}
