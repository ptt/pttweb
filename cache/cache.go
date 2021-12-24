package cache

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/ptt/pttweb/gate"
)

const (
	// Request and connect timeout
	DefaultTimeout = time.Second * 30
)

var (
	ErrTooBusy = errors.New("conn pool too busy")
)

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
	server string
	mc     *memcache.Client
	gate   *gate.Gate

	mu      sync.Mutex
	pending map[string][]resultChan
}

func NewCacheManager(server string, maxOpen int) *CacheManager {
	mc := memcache.New(server)
	mc.Timeout = DefaultTimeout
	mc.MaxIdleConns = maxOpen

	return &CacheManager{
		server:  server,
		mc:      mc,
		gate:    gate.New(maxOpen, maxOpen),
		pending: make(map[string][]resultChan),
	}
}

func NewTyped[K Key, V any](m *CacheManager, gen Generator[K, V], ser Serializer[V], des Deserializer[V]) *TypedManager[*CacheManager, K, V] {
	return newTypedManager[*CacheManager, K, V](m, gen, ser, des)
}

func (m *CacheManager) Get(key Key, tp NewableFromBytes, expire time.Duration, generate GenerateFunc) (Cacheable, error) {
	keyString := key.String()

	// Check if can be served from cache
	if data, err := m.getFromCache(keyString); err != nil {
		if err != memcache.ErrCacheMiss {
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
	rsv, ok := m.gate.Reserve()
	if !ok {
		return nil, ErrTooBusy
	}
	rsv.Wait()
	defer rsv.Release()

	res, err := m.mc.Get(key)
	if err != nil {
		return nil, err
	}
	return res.Value, nil
}

func (m *CacheManager) Fetch(key string) ([]byte, error) {
	return m.getFromCache(key)
}

func (m *CacheManager) storeResultCache(key string, data []byte, expire time.Duration) error {
	rsv, ok := m.gate.Reserve()
	if !ok {
		return ErrTooBusy
	}
	rsv.Wait()
	defer rsv.Release()

	return m.mc.Set(&memcache.Item{
		Key:        key,
		Value:      data,
		Flags:      uint32(0),
		Expiration: int32(expire.Seconds()),
	})
}

func (m *CacheManager) Store(key string, data []byte, expire time.Duration) error {
	return m.storeResultCache(key, data, expire)
}
