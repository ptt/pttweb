// Package gate provides building blocks to limit concurrency.
package gate

import (
	"sync"
)

// Gate provides concurrency limitation.
type Gate struct {
	maxInflight int
	maxWait     int

	mu    sync.Mutex
	num   int
	queue chan *Reservation
}

// New creates a Gate.
func New(maxInflight, maxWait int) *Gate {
	return &Gate{
		maxInflight: maxInflight,
		maxWait:     maxWait,
		queue:       make(chan *Reservation, maxWait),
	}
}

// Reserve attempts to obtain a reservation. If the wait queue is too long, it
// returns false.
func (g *Gate) Reserve() (*Reservation, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.num >= g.maxInflight+g.maxWait {
		return nil, false
	}
	g.num++

	// Grant immediately.
	if g.num <= g.maxInflight {
		return &Reservation{
			g: g,
		}, true
	}

	// Grant later.
	r := &Reservation{
		g:       g,
		granted: make(chan struct{}),
	}
	g.queue <- r
	return r, true
}

func (g *Gate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.num--

	select {
	case r := <-g.queue:
		r.grant()
	default:
	}
}

// Reservation represents a reservation.
type Reservation struct {
	g       *Gate
	granted chan struct{}
}

// Wait blocks until number of inflight requests is lower than the maximum.
func (r *Reservation) Wait() {
	if r.granted != nil {
		<-r.granted
	}
}

// Release returns the reservation. It must be called when the reservation is
// no longer needed. It doesn't matter if Wait() was called or not.
func (r *Reservation) Release() {
	r.g.release()
}

func (r *Reservation) grant() {
	close(r.granted)
}

func (r *Reservation) isGranted() bool {
	select {
	case <-r.granted:
		return true
	default:
		return r.granted == nil
	}
}
