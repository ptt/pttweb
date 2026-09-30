package cache

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/ptt/pttweb/gate"
	"golang.org/x/sync/singleflight"
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

type CacheManager struct {
	server string
	mc     *memcache.Client
	gate   *gate.Gate
	group  singleflight.Group
}

func NewCacheManager(server string, maxOpen int) *CacheManager {
	mc := memcache.New(server)
	mc.Timeout = DefaultTimeout
	mc.MaxIdleConns = maxOpen

	return &CacheManager{
		server: server,
		mc:     mc,
		gate:   gate.New(maxOpen, maxOpen),
	}
}

func (m *CacheManager) Get(key Key, tp NewableFromBytes, expire time.Duration, generate GenerateFunc) (Cacheable, error) {
	keyString := key.String()

	// Check if can be served from cache
	if data, err := m.getFromCache(keyString); err != nil {
		if err != memcache.ErrCacheMiss {
			log.Printf("getFromCache: key: %q, err: %v", keyString, err)
		}
	} else if data != nil {
		if obj, err := tp.NewFromBytes(data); err == nil {
			return obj, nil
		} else {
			log.Printf("tp.NewFromBytes: key: %q, err: %v", keyString, err)
		}
	}

	val, err, _ := m.group.Do(keyString, func() (retVal interface{}, retErr error) {
		defer func() {
			if r := recover(); r != nil {
				retErr = fmt.Errorf("panic in cache generator for %q: %v", keyString, r)
				log.Println(retErr)
			}
		}()

		// Double-check cache in case another flight just finished
		if data, err := m.getFromCache(keyString); err == nil && data != nil {
			if obj, err := tp.NewFromBytes(data); err == nil {
				return obj, nil
			}
		}

		obj, err := generate(key)
		if err != nil {
			return nil, err
		}

		// Store result in cache if generated successfully
		if data, err := obj.EncodeToBytes(); err != nil {
			log.Printf("obj.EncodeToBytes: key: %q, err: %v", keyString, err)
		} else if err := m.storeResultCache(keyString, data, expire); err != nil {
			log.Printf("storeResultCache: key: %q, err: %v", keyString, err)
		}

		return obj, nil
	})

	if err != nil {
		return nil, err
	}
	return val.(Cacheable), nil
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
