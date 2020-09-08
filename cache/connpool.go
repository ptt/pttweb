package cache

import (
	"errors"
	"log"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

const (
	// Request and connect timeout
	DefaultTimeout = time.Second * 30
)

var (
	ErrTooBusy = errors.New("conn pool too busy")
)

type MemcacheConnPool struct {
	idle            chan connectResult
	drop            chan error
	req             chan int
	nrOpen, maxOpen int
	nrWait          int
	server          string
}

type connectResult struct {
	conn *memcache.Client
	err  error
}

func NewMemcacheConnPool(server string, maxOpen int) *MemcacheConnPool {
	m := &MemcacheConnPool{
		idle:    make(chan connectResult),
		drop:    make(chan error),
		req:     make(chan int),
		nrOpen:  0,
		maxOpen: maxOpen,
		nrWait:  0,
		server:  server,
	}
	go m.manager()
	return m
}

func (m *MemcacheConnPool) GetConn() (*memcache.Client, error) {
	if m.nrWait > 2*m.maxOpen {
		return nil, ErrTooBusy
	}
	var r connectResult
	select {
	case r = <-m.idle:
	default:
		m.req <- 1
		r = <-m.idle
		m.req <- -1
	}
	if r.err != nil {
		m.DropConn(r.conn)
	}
	return r.conn, r.err
}

func (m *MemcacheConnPool) ReleaseConn(c *memcache.Client, err error) {
	if err != nil {
		log.Printf("MemcacheConnPool: dropping bad connection to %s due to error: %s\n",
			m.server, err.Error())
		m.DropConn(c)
		return
	}
	go func(c *memcache.Client) {
		select {
		case m.idle <- connectResult{conn: c, err: nil}:
			// Somebody got it
		case <-time.After(time.Second * 10):
			// Timeout, close it
			m.DropConn(c)
		}
	}(c)
}

func (m *MemcacheConnPool) DropConn(c *memcache.Client) {
	m.drop <- nil

	// c will be GCed later
	// https://github.com/bradfitz/gomemcache/issues/51
	c = nil
}

func (m *MemcacheConnPool) manager() {
	for {
		select {
		case <-m.drop:
			m.nrOpen--
		case i := <-m.req:
			m.nrWait += i
		}
		for i := m.nrWait; i > 0 && m.nrOpen < m.maxOpen; i-- {
			m.nrOpen++
			go m.connect()
		}
	}
}

func (m *MemcacheConnPool) connect() {
	c := memcache.New(m.server)
	c.Timeout = DefaultTimeout
	if err := c.Ping(); err != nil {
		m.idle <- connectResult{conn: c, err: err}
	} else {
		m.ReleaseConn(c, nil)
	}
}
