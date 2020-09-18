package gate

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGate(t *testing.T) {
	g := New(4, 2)

	var rs [6]*Reservation
	for i := range rs {
		var ok bool
		rs[i], ok = g.Reserve()
		if !ok {
			t.Fatalf("#%v Reserve() = _, %v; want _, true", i, ok)
		}

		// First 4 reservation should be in already-granted state.
		granted := i < 4
		if got := rs[i].isGranted(); got != granted {
			t.Errorf("#%v isGranted() = %v, want %v", i, got, granted)
		}
	}

	// Reserved at maximum. Should start failing now.
	for i := len(rs); i < len(rs)*2; i++ {
		if _, ok := g.Reserve(); ok {
			t.Fatalf("#%v Reserve() = _, %v; want _, false", i, ok)
		}
	}

	// Wait should return immediately for the first 4 already granted
	// reservations.
	rs[0].Wait()
	rs[1].Wait()
	rs[2].Wait()
	rs[3].Wait()

	// Release 1 and Wait 1.
	rs[0].Release()
	rs[4].Wait()

	// Should be able to reserve another one now.
	if r, ok := g.Reserve(); !ok {
		t.Errorf("Reserve() = _, %v; want _, true", ok)
	} else {
		defer r.Release()
		if r.isGranted() {
			t.Errorf("isGranted() = true, want false")
		}
	}

	// Release 1 and Wait 1.
	rs[1].Release()
	rs[5].Wait()

	// Release the rest of reservations.
	for i := 2; i < len(rs); i++ {
		rs[i].Release()
	}
}

func concurrentWorker(t *testing.T, wg *sync.WaitGroup, g *Gate, num *int32, max int32) {
	defer wg.Done()
	for j := 0; j < 1000; j++ {
		us := rand.Intn(200)
		r, ok := g.Reserve()
		if !ok {
			time.Sleep(time.Duration(us) * time.Microsecond)
			j--
			continue
		}

		r.Wait()
		if atomic.AddInt32(num, 1) > max {
			t.Fatal("num > max")
		}
		time.Sleep(time.Duration(us) * time.Microsecond)
		atomic.AddInt32(num, -1)
		r.Release()
	}
}

func TestConcurrent(t *testing.T) {
	const max = 10
	var num int32
	var wg sync.WaitGroup
	defer wg.Wait()

	g := New(max, max)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go concurrentWorker(t, &wg, g, &num, max)
	}
}
