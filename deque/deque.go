// Package deque implements a lock-free, fixed-capacity work-stealing deque
// (Chase-Lev). The owner thread pushes and pops at the bottom; any other
// thread may steal from the top. Go's sync/atomic operations are sequentially
// consistent, which is what the algorithm requires.
package deque

import "sync/atomic"

// StealResult describes the outcome of a Steal attempt.
type StealResult int

const (
	// Empty means the deque had no tasks.
	Empty StealResult = iota
	// Abort means the thief lost a race and may retry.
	Abort
	// Success means a task was stolen.
	Success
)

const cacheLine = 64

// Deque holds int task identifiers. Only the owner may call PushBottom and
// PopBottom; any goroutine may call Steal.
type Deque struct {
	top    atomic.Int64
	_      [cacheLine - 8]byte // keep top and bottom on separate cache lines
	bottom atomic.Int64
	_      [cacheLine - 8]byte
	buf    []atomic.Int64
	mask   int64
}

// New creates a deque able to hold at least capacity tasks.
func New(capacity int) *Deque {
	size := 1
	for size < capacity {
		size <<= 1
	}
	return &Deque{buf: make([]atomic.Int64, size), mask: int64(size - 1)}
}

// PushBottom adds a task at the owner's end. It returns false if full.
func (d *Deque) PushBottom(task int) bool {
	b := d.bottom.Load()
	t := d.top.Load()
	if b-t >= int64(len(d.buf)) {
		return false
	}
	d.buf[b&d.mask].Store(int64(task))
	d.bottom.Store(b + 1)
	return true
}

// PopBottom removes a task from the owner's end.
func (d *Deque) PopBottom() (int, bool) {
	b := d.bottom.Load() - 1
	d.bottom.Store(b)
	t := d.top.Load()
	if t > b { // already empty
		d.bottom.Store(b + 1)
		return 0, false
	}
	task := d.buf[b&d.mask].Load()
	if t == b { // last element: race against thieves
		won := d.top.CompareAndSwap(t, t+1)
		d.bottom.Store(b + 1)
		if !won {
			return 0, false
		}
	}
	return int(task), true
}

// Steal removes a task from the thieves' end.
func (d *Deque) Steal() (int, StealResult) {
	t := d.top.Load()
	b := d.bottom.Load()
	if t >= b {
		return 0, Empty
	}
	task := d.buf[t&d.mask].Load()
	if !d.top.CompareAndSwap(t, t+1) {
		return 0, Abort
	}
	return int(task), Success
}
