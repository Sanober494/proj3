// Package bsp provides the barrier used between BSP supersteps.
package bsp

import "sync"

// Barrier is a reusable barrier built from a mutex and a condition variable.
type Barrier struct {
	mu         sync.Mutex
	cond       *sync.Cond
	parties    int
	waiting    int
	generation int
}

// NewBarrier creates a barrier for the given number of goroutines.
func NewBarrier(parties int) *Barrier {
	b := &Barrier{parties: parties}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Wait blocks until all parties have called Wait, then releases them all.
func (b *Barrier) Wait() {
	b.mu.Lock()
	defer b.mu.Unlock()
	gen := b.generation
	b.waiting++
	if b.waiting == b.parties {
		b.waiting = 0
		b.generation++
		b.cond.Broadcast()
		return
	}
	for gen == b.generation {
		b.cond.Wait()
	}
}
