package cache

import (
	"log"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

type Cache interface {
	Fetch(key string) ([]byte, error)
	Store(key string, data []byte, expire time.Duration) error
}

type Serializer[V any] func(value V) ([]byte, error)

type Deserializer[V any] func(data []byte) (V, error)

type Generator[K Key, V any] func(key K) (V, time.Duration, error)

type res[V any] struct {
	value V
	err   error
}

type TypedManager[C Cache, K Key, V any] struct {
	c            Cache
	sf           *singleflight[*res[V]]
	generator    Generator[K, V]
	serializer   Serializer[V]
	deserializer Deserializer[V]
}

func newTypedManager[C Cache, K Key, V any](c Cache, gen Generator[K, V], ser Serializer[V], des Deserializer[V]) *TypedManager[C, K, V] {
	return &TypedManager[C, K, V]{
		c:            c,
		sf:           newSingleFlight[*res[V]](),
		generator:    gen,
		serializer:   ser,
		deserializer: des,
	}
}

func (tm *TypedManager[C, K, V]) Get(key K) (V, error) {
	keyString := key.String()

	// Check if can be served from cache
	if data, err := tm.c.Fetch(keyString); err != nil {
		if err != memcache.ErrCacheMiss {
			log.Printf("getFromCache: key: %q, err: %v", keyString, err)
		}
	} else if data != nil {
		return tm.deserializer(data)
	}

	ch := make(chan *res[V])

	// No luck. Check if anyone is generating
	if first := tm.sf.Request(keyString, ch); first {
		// We are the one responsible for generating the result
		go tm.doGenerate(key, keyString)
	}

	r := <-ch
	return r.value, r.err
}

func (tm *TypedManager[C, K, V]) doGenerate(key K, keyString string) {
	value, expire, err := tm.generator(key)
	if err == nil {
		// There is no errors during generating, store result in cache
		if data, err := tm.serializer(value); err != nil {
			log.Printf("tm.serializer(value): key: %q, err: %v", keyString, err)
		} else if err = tm.c.Store(keyString, data, expire); err != nil {
			log.Printf("storeResultCache: key: %q, err: %v", keyString, err)
		}
	}

	tm.sf.Fulfill(keyString, &res[V]{
		value: value,
		err:   err,
	})
}
