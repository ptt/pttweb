package main

import (
	"errors"
)

type PttwebConfig struct {
	Bind              []string
	BoarddAddress     string
	MemcachedAddress  string
	TemplateDirectory string
	StaticPrefix      string

	BoarddMaxConn    int
	MemcachedMaxConn int

	GAAccount string
	GADomain  string

	EnableOver18Cookie bool
}

const (
	DefaultBoarddMaxConn    = 16
	DefaultMemcachedMaxConn = 16
)

func (c *PttwebConfig) CheckAndFillDefaults() error {
	if c.BoarddAddress == "" {
		return errors.New("boardd address not specified")
	}

	if c.MemcachedAddress == "" {
		return errors.New("memcached address not specified")
	}

	if c.BoarddMaxConn <= 0 {
		c.BoarddMaxConn = DefaultBoarddMaxConn
	}

	if c.MemcachedMaxConn <= 0 {
		c.MemcachedMaxConn = DefaultMemcachedMaxConn
	}

	return nil
}
