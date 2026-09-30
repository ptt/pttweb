package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testKey string

func (k testKey) String() string {
	return string(k)
}

type testObject struct {
	data string
}

func (o *testObject) NewFromBytes(b []byte) (Cacheable, error) {
	return &testObject{data: string(b)}, nil
}

func (o *testObject) EncodeToBytes() ([]byte, error) {
	return []byte(o.data), nil
}

var zeroTestObject = &testObject{}

func TestCacheManager_Basic(t *testing.T) {
	// Point to dummy address so memcached fails fast / cache miss
	cm := NewCacheManager("127.0.0.1:1", 1)

	calls := int32(0)
	obj, err := cm.Get(testKey("key1"), zeroTestObject, time.Minute, func(k Key) (Cacheable, error) {
		atomic.AddInt32(&calls, 1)
		return &testObject{data: "val1"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if to, ok := obj.(*testObject); !ok || to.data != "val1" {
		t.Fatalf("got %v, want val1", obj)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestCacheManager_Singleflight(t *testing.T) {
	cm := NewCacheManager("127.0.0.1:1", 10)

	calls := int32(0)
	start := make(chan struct{})
	const concurrency = 20

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			obj, err := cm.Get(testKey("same_key"), zeroTestObject, time.Minute, func(k Key) (Cacheable, error) {
				atomic.AddInt32(&calls, 1)
				time.Sleep(50 * time.Millisecond) // simulate slow backend
				return &testObject{data: "coalesced"}, nil
			})
			if err != nil {
				t.Errorf("worker %d: unexpected error: %v", id, err)
				return
			}
			if to, ok := obj.(*testObject); !ok || to.data != "coalesced" {
				t.Errorf("worker %d: got %v, want coalesced", id, obj)
			}
		}(i)
	}

	close(start)
	wg.Wait()

	// All 20 concurrent requests must have coalesced into exactly 1 generation call
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (singleflight failed to coalesce)", calls)
	}
}

func TestCacheManager_ErrorPropagation(t *testing.T) {
	cm := NewCacheManager("127.0.0.1:1", 1)

	wantErr := errors.New("backend failed")
	_, err := cm.Get(testKey("err_key"), zeroTestObject, time.Minute, func(k Key) (Cacheable, error) {
		return nil, wantErr
	})
	if err != wantErr {
		t.Fatalf("got err = %v, want %v", err, wantErr)
	}
}

func TestCacheManager_PanicRecovery(t *testing.T) {
	cm := NewCacheManager("127.0.0.1:1", 1)

	// First call panics in generator, should return error without crashing
	_, err := cm.Get(testKey("panic_key"), zeroTestObject, time.Minute, func(k Key) (Cacheable, error) {
		panic("something went terribly wrong")
	})
	if err == nil {
		t.Errorf("expected error from panicked generator, got nil")
	}

	// Subsequent call to same key MUST NOT deadlock or hang
	done := make(chan struct{})
	go func() {
		defer close(done)
		obj, err := cm.Get(testKey("panic_key"), zeroTestObject, time.Minute, func(k Key) (Cacheable, error) {
			return &testObject{data: "recovered"}, nil
		})
		if err != nil {
			t.Errorf("subsequent Get failed: %v", err)
			return
		}
		if to, ok := obj.(*testObject); !ok || to.data != "recovered" {
			t.Errorf("got %v, want recovered", obj)
		}
	}()

	select {
	case <-done:
		// Success! Did not deadlock!
	case <-time.After(2 * time.Second):
		t.Fatal("DEADLOCK: subsequent call hanged after panic!")
	}
}
