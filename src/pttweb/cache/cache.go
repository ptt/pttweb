package cache

import (
	"code.google.com/p/vitess/go/memcache"
	"pttbbs"
	"sync"
	"time"
)

type Result struct {
	Output []byte
	Err    error
	Expire time.Duration
}

type Request struct {
	Key      string
	Output   chan<- Result
	Generate GenerateFunc
}

type GenerateFunc func(key string) Result

type CacheManager struct {
	server   string
	connPool *pttbbs.MemcacheConnPool

	mu      sync.Mutex
	pending map[string][]Request
}

func NewCacheManager(server string) *CacheManager {
	return &CacheManager{
		server:   server,
		connPool: pttbbs.NewMemcacheConnPool(server, 8),
		pending:  make(map[string][]Request),
	}
}

func (m *CacheManager) Get(key string, generate GenerateFunc) <-chan Result {
	resultChan := make(chan Result)

	go func() {
		// Check if can be served from cache
		if result, err := m.getFromCache(key); err == nil && result.Output != nil {
			resultChan <- result
			return
		}

		// No luck. Check if anyone is generating
		cascade := true
		m.mu.Lock()
		if _, ok := m.pending[key]; !ok {
			cascade = false
			m.pending[key] = make([]Request, 0, 1)
		}
		m.pending[key] = append(m.pending[key], Request{
			Key:      key,
			Output:   resultChan,
			Generate: generate,
		})
		m.mu.Unlock()

		if cascade {
			// Someone is generating, we wait
			return
		}

		// We are the one responsible for generating the result
		result := generate(key)
		if result.Err == nil {
			// If there is no errors during generating, store result in cache
			m.storeResultCache(key, result)
		}

		// Respond to all audience
		m.mu.Lock()
		audience := m.pending[key]
		delete(m.pending, key)
		m.mu.Unlock()

		for _, c := range audience {
			c.Output <- result
		}
	}()

	return resultChan
}

func (m *CacheManager) getFromCache(key string) (result Result, err error) {
	var memd *memcache.Connection
	if memd, err = m.connPool.GetConn(); err == nil {
		defer func() {
			m.connPool.ReleaseConn(memd, err)
		}()

		result.Output, _, err = memd.Get(key)
		result.Err = err
	}
	return
}

func (m *CacheManager) storeResultCache(key string, result Result) (err error) {
	var memd *memcache.Connection
	if memd, err = m.connPool.GetConn(); err == nil {
		defer func() {
			m.connPool.ReleaseConn(memd, err)
		}()

		_, err = memd.Set(key, 0, uint64(result.Expire.Seconds()), result.Output)
	}
	return
}
