package cache

import "sync"

type singleflight[V any] struct {
	mu      sync.Mutex
	pending map[string][]chan V
}

func newSingleFlight[V any]() *singleflight[V] {
	return &singleflight[V]{
		pending: make(map[string][]chan V),
	}
}

func (s *singleflight[V]) Request(key string, ch chan V) (first bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.pending[key]; !ok {
		first = true
		s.pending[key] = make([]chan V, 0, 1)
	}
	s.pending[key] = append(s.pending[key], ch)
	return
}

func (s *singleflight[V]) Fulfill(key string, value V) {
	for _, c := range s.removePendings(key) {
		c <- value
	}
}

func (s *singleflight[V]) removePendings(key string) []chan V {
	s.mu.Lock()
	defer s.mu.Unlock()

	pendings := s.pending[key]
	delete(s.pending, key)
	return pendings
}
