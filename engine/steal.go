package engine

import (
	"math/rand"
	"runtime"
	"sync/atomic"

	"proj3-redesigned/deque"
)

// stealer schedules the compute superstep with per-worker lock-free deques.
// Tiles are initially dealt out in the same contiguous blocks as the static
// mode, so any difference in performance comes purely from stealing.
type stealer struct {
	s         *state
	deques    []*deque.Deque
	remaining atomic.Int64 // tiles not yet finished; workers exit at zero
}

func (s *state) newStealer() *stealer {
	st := &stealer{s: s, deques: make([]*deque.Deque, s.cfg.Threads)}
	st.remaining.Store(int64(s.tiles))
	for tid := range st.deques {
		lo, hi := block(s.tiles, s.cfg.Threads, tid)
		st.deques[tid] = deque.New(hi - lo + 1)
		// Push in reverse so the owner pops tiles in image order while
		// thieves take from the far end of the owner's block.
		for tile := hi - 1; tile >= lo; tile-- {
			st.deques[tid].PushBottom(tile)
		}
	}
	return st
}

func (st *stealer) compute(tid int) {
	rng := rand.New(rand.NewSource(int64(tid) + 1))
	own := st.deques[tid]
	for st.remaining.Load() > 0 {
		if tile, ok := own.PopBottom(); ok {
			st.finish(tid, tile)
		} else if tile, ok := st.stealFrom(tid, rng); ok {
			st.s.workers[tid].Stolen++
			st.finish(tid, tile)
		} else {
			runtime.Gosched()
		}
	}
}

func (st *stealer) finish(tid, tile int) {
	st.s.runTile(tid, tile)
	st.remaining.Add(-1)
}

// stealFrom tries every other worker once, starting at a random victim.
func (st *stealer) stealFrom(tid int, rng *rand.Rand) (int, bool) {
	n := len(st.deques)
	first := rng.Intn(n)
	for i := 0; i < n; i++ {
		victim := (first + i) % n
		if victim == tid {
			continue
		}
		for {
			tile, res := st.deques[victim].Steal()
			if res == deque.Success {
				return tile, true
			}
			if res == deque.Empty {
				break
			}
		}
	}
	return 0, false
}
