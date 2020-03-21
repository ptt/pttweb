package cache

import (
	"errors"
	"log"
	"sync"
	"time"
)

var errCacheMiss = errors.New("cache miss")

type Key interface {
	String() string
}

type NewableFromBytes interface {
	NewFromBytes(data []byte) (Cacheable, error)
}

type Cacheable interface {
	NewableFromBytes
	EncodeToBytes() ([]byte, error)
}

type GenerateFunc func(key Key) (Cacheable, error)

type result struct {
	Obj Cacheable
	Err error
}

type resultChan chan result

type CacheManager struct {
	server   string
	connPool *MemcacheConnPool

	mu      sync.Mutex
	pending map[string][]resultChan
}

func NewCacheManager(server string, maxOpen int) *CacheManager {
	return &CacheManager{
		server:   server,
		connPool: NewMemcacheConnPool(server, maxOpen),
		pending:  make(map[string][]resultChan),
	}
}

func (m *CacheManager) Get(key Key, tp NewableFromBytes, expire time.Duration, generate GenerateFunc) (Cacheable, error) {
	keyString := key.String()

	// Check if can be served from cache
	if data, err := m.getFromCache(keyString); err != nil {
		if err != errCacheMiss {
			log.Printf("getFromCache: key: %q, err: %v", keyString, err)
		}
	} else if data != nil {
		return tp.NewFromBytes(data)
	}

	ch := make(chan result)

	// No luck. Check if anyone is generating
	if first := m.putPendings(keyString, ch); first {
		// We are the one responsible for generating the result
		go m.doGenerate(key, keyString, expire, generate)
	}

	result := <-ch
	return result.Obj, result.Err
}

func (m *CacheManager) doGenerate(key Key, keyString string, expire time.Duration, generate GenerateFunc) {
	obj, err := generate(key)
	if err == nil {
		// There is no errors during generating, store result in cache
		if data, err := obj.EncodeToBytes(); err != nil {
			log.Printf("obj.EncodeToBytes: key: %q, err: %v", keyString, err)
		} else if err = m.storeResultCache(keyString, data, expire); err != nil {
			log.Printf("storeResultCache: key: %q, err: %v", keyString, err)
		}
	}

	// Respond to all audience
	result := result{
		Obj: obj,
		Err: err,
	}
	for _, c := range m.removePendings(keyString) {
		c <- result
	}
}

func (m *CacheManager) putPendings(key string, ch resultChan) (first bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.pending[key]; !ok {
		first = true
		m.pending[key] = make([]resultChan, 0, 1)
	}
	m.pending[key] = append(m.pending[key], ch)
	return
}

func (m *CacheManager) removePendings(key string) []resultChan {
	m.mu.Lock()
	defer m.mu.Unlock()

	pendings := m.pending[key]
	delete(m.pending, key)
	return pendings
}

func (m *CacheManager) getFromCache(key string) ([]byte, error) {
	memd, err := m.connPool.GetConn()
	if err != nil {
		return nil, err
	}

	res, err := memd.Get(key)
	defer m.connPool.ReleaseConn(memd, err)

	if err != nil {
		return nil, err
	} else if len(res) != 1 {
		return nil, errCacheMiss
	}
	return res[0].Value, nil
}

func (m *CacheManager) storeResultCache(key string, data []byte, expire time.Duration) error {
	memd, err := m.connPool.GetConn()
	if err != nil {
		return err
	}
	defer m.connPool.ReleaseConn(memd, err)

	_, err = memd.Set(key, 0, uint64(expire.Seconds()), data)
	return err
}
