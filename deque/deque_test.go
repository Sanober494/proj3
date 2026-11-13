package deque

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestOwnerLIFO(t *testing.T) {
	d := New(8)
	for i := 0; i < 5; i++ {
		d.PushBottom(i)
	}
	for i := 4; i >= 0; i-- {
		if v, ok := d.PopBottom(); !ok || v != i {
			t.Fatalf("got %d,%v want %d", v, ok, i)
		}
	}
	if _, ok := d.PopBottom(); ok {
		t.Fatal("expected empty")
	}
}

func TestStealFIFO(t *testing.T) {
	d := New(8)
	d.PushBottom(1)
	d.PushBottom(2)
	if v, r := d.Steal(); r != Success || v != 1 {
		t.Fatalf("got %d,%v", v, r)
	}
}

// Every task must be consumed exactly once under contention.
func TestConcurrentExactlyOnce(t *testing.T) {
	const n, thieves = 200000, 7
	d := New(1024)
	seen := make([]atomic.Int32, n)
	var done atomic.Bool
	var wg sync.WaitGroup
	for i := 0; i < thieves; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !done.Load() {
				if v, r := d.Steal(); r == Success {
					seen[v].Add(1)
				}
			}
			for {
				v, r := d.Steal()
				if r == Empty {
					return
				}
				if r == Success {
					seen[v].Add(1)
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		for !d.PushBottom(i) {
			if v, ok := d.PopBottom(); ok {
				seen[v].Add(1)
			}
		}
		if i%3 == 0 {
			if v, ok := d.PopBottom(); ok {
				seen[v].Add(1)
			}
		}
	}
	for {
		v, ok := d.PopBottom()
		if !ok {
			break
		}
		seen[v].Add(1)
	}
	done.Store(true)
	wg.Wait()
	for i := range seen {
		if c := seen[i].Load(); c != 1 {
			t.Fatalf("task %d consumed %d times", i, c)
		}
	}
}
