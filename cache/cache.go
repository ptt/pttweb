package cache

import (
	"errors"
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

type CacheManager struct {
	server string
	mc     *memcache.Client
	gate   *gate.Gate
}

func NewCacheManager(server string, maxOpen int) *CacheManager {
	mc := memcache.New(server)
	mc.Timeout = DefaultTimeout
	mc.MaxIdleConns = maxOpen

	return &CacheManager{
		server: server,
		mc:     mc,
	}
}

func (m *CacheManager) Fetch(key string) ([]byte, error) {
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

func (m *CacheManager) Store(key string, data []byte, expire time.Duration) error {
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
