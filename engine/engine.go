// Package engine runs the Mandelbrot pipeline sequentially or in parallel.
//
// The parallel versions follow the BSP pattern with four supersteps separated
// by condition-variable barriers:
//
//  1. compute  - escape iterations per pixel + thread-local histograms
//  2. merge    - histograms reduced into a global one (bins split by thread)
//  3. prefix   - thread 0 builds the cumulative histogram
//  4. colour   - rows coloured in parallel
//
// The two parallel modes differ only in how superstep 1 is scheduled.
package engine

import (
	"fmt"
	"image"
	"time"

	"proj3-redesigned/bsp"
	"proj3-redesigned/fractal"
)

// Mode selects the implementation.
type Mode string

const (
	Sequential Mode = "seq"
	Static     Mode = "static"
	Steal      Mode = "steal"
)

// ParseMode validates a mode name.
func ParseMode(s string) (Mode, error) {
	switch m := Mode(s); m {
	case Sequential, Static, Steal:
		return m, nil
	}
	return "", fmt.Errorf("unknown mode %q (want seq, static or steal)", s)
}

// WorkerStats records what one worker did during the compute superstep.
type WorkerStats struct {
	Iterations int64 // escape-time iterations performed
	Tiles      int   // tiles processed
	Stolen     int   // tiles obtained by stealing
	_          [40]byte
}

// Phases holds the wall time of each superstep.
type Phases struct{ Compute, Merge, Prefix, Color time.Duration }

// Result is the output of a run.
type Result struct {
	Image   *image.RGBA
	Workers []WorkerStats
	Phases  Phases
	Elapsed time.Duration
}

// Config bundles the run settings.
type Config struct {
	Mode     Mode
	Threads  int
	TileRows int
	Params   fractal.Params
}

// run-wide shared state for the BSP supersteps.
type state struct {
	cfg     Config
	canvas  *fractal.Canvas
	img     *image.RGBA
	locals  [][]int64
	global  []int64
	cum     []int64
	total   int64
	workers []WorkerStats
	tiles   int
}

func newState(cfg Config) *state {
	p := cfg.Params
	s := &state{
		cfg:     cfg,
		canvas:  fractal.NewCanvas(p),
		img:     image.NewRGBA(image.Rect(0, 0, p.Width, p.Height)),
		locals:  make([][]int64, cfg.Threads),
		global:  p.NewHistogram(),
		workers: make([]WorkerStats, cfg.Threads),
		tiles:   (p.Height + cfg.TileRows - 1) / cfg.TileRows,
	}
	for i := range s.locals {
		s.locals[i] = p.NewHistogram()
	}
	return s
}

// block returns the half-open range of n items owned by worker tid of t.
func block(n, t, tid int) (int, int) { return tid * n / t, (tid + 1) * n / t }

// runTile computes one tile of rows on behalf of worker tid.
func (s *state) runTile(tid, tile int) {
	y0 := tile * s.cfg.TileRows
	y1 := y0 + s.cfg.TileRows
	if y1 > s.cfg.Params.Height {
		y1 = s.cfg.Params.Height
	}
	w := &s.workers[tid]
	w.Iterations += s.canvas.ComputeRows(y0, y1, s.locals[tid])
	w.Tiles++
}

func (s *state) computeStatic(tid int) {
	lo, hi := block(s.tiles, s.cfg.Threads, tid)
	for tile := lo; tile < hi; tile++ {
		s.runTile(tid, tile)
	}
}

func (s *state) merge(tid int) {
	lo, hi := block(len(s.global), s.cfg.Threads, tid)
	fractal.MergeHist(s.global, s.locals, lo, hi)
}

func (s *state) prefix() {
	s.cum, s.total = fractal.Cumulative(s.global, s.cfg.Params.MaxIter)
}

func (s *state) color(tid int) {
	lo, hi := block(s.cfg.Params.Height, s.cfg.Threads, tid)
	s.canvas.ColorRows(lo, hi, s.cum, s.total, s.img)
}

// Run executes the configured implementation.
func Run(cfg Config) Result {
	if cfg.Mode == Sequential {
		cfg.Threads = 1
	}
	s := newState(cfg)
	start := time.Now()
	var ph Phases
	if cfg.Mode == Sequential {
		ph = s.runSequential()
	} else {
		ph = s.runBSP()
	}
	return Result{Image: s.img, Workers: s.workers, Phases: ph, Elapsed: time.Since(start)}
}

func (s *state) runSequential() Phases {
	var ph Phases
	t := time.Now()
	lap := func(d *time.Duration) { now := time.Now(); *d = now.Sub(t); t = now }
	for tile := 0; tile < s.tiles; tile++ {
		s.runTile(0, tile)
	}
	lap(&ph.Compute)
	s.merge(0)
	lap(&ph.Merge)
	s.prefix()
	lap(&ph.Prefix)
	s.color(0)
	lap(&ph.Color)
	return ph
}

func (s *state) runBSP() Phases {
	threads := s.cfg.Threads
	barrier := bsp.NewBarrier(threads)
	compute := s.computeStatic
	if s.cfg.Mode == Steal {
		compute = s.newStealer().compute
	}
	var ph Phases
	done := make(chan struct{}, threads)
	start := time.Now()
	for tid := 0; tid < threads; tid++ {
		go func(tid int) {
			t := start
			lap := func(d *time.Duration) { // only worker 0 records phase times
				if tid == 0 {
					now := time.Now()
					*d = now.Sub(t)
					t = now
				}
			}
			compute(tid)
			barrier.Wait()
			lap(&ph.Compute)
			s.merge(tid)
			barrier.Wait()
			lap(&ph.Merge)
			if tid == 0 {
				s.prefix()
			}
			barrier.Wait()
			lap(&ph.Prefix)
			s.color(tid)
			barrier.Wait()
			lap(&ph.Color)
			done <- struct{}{}
		}(tid)
	}
	for i := 0; i < threads; i++ {
		<-done
	}
	return ph
}
