// Command proj3 renders a Mandelbrot image with a sequential, a static BSP,
// or a work-stealing BSP implementation.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"time"

	"proj3-redesigned/engine"
	"proj3-redesigned/fractal"
)

type preset struct {
	params fractal.Params
	about  string
}

var presets = map[string]preset{
	"full": {fractal.Params{Width: 3000, Height: 2000, MaxIter: 3000, CenterX: -0.5, CenterY: 0, Scale: 3},
		"whole set; cost concentrated in middle rows (imbalanced)"},
	"seahorse": {fractal.Params{Width: 2400, Height: 1600, MaxIter: 6000, CenterX: -0.743643887037151, CenterY: 0.131825904205330, Scale: 0.004},
		"seahorse valley zoom; irregular cost"},
	"uniform": {fractal.Params{Width: 2400, Height: 1600, MaxIter: 1500, CenterX: -0.1, CenterY: 0, Scale: 0.2},
		"view inside the main cardioid; every pixel costs the same"},
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: proj3 [flags]\n\nFlags:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nModes:\n  seq     sequential\n  static  BSP, tiles split in equal contiguous blocks\n  steal   BSP, lock-free deques with work stealing\n\nPresets:\n")
	for name, p := range presets {
		fmt.Fprintf(os.Stderr, "  %-9s %s\n", name, p.about)
	}
	fmt.Fprintf(os.Stderr, "\nExample: proj3 -mode steal -t 8 -preset full -o out.png\n")
}

func main() {
	flag.Usage = usage
	modeName := flag.String("mode", "seq", "implementation: seq, static, steal")
	threads := flag.Int("t", 4, "number of worker threads (ignored for seq)")
	presetName := flag.String("preset", "full", "named input region (see below)")
	tile := flag.Int("tile", 4, "rows per task")
	out := flag.String("o", "", "output PNG path (empty: do not write)")
	stats := flag.Bool("stats", false, "print per-worker statistics")
	var p fractal.Params
	flag.IntVar(&p.Width, "w", 0, "image width (overrides preset)")
	flag.IntVar(&p.Height, "h", 0, "image height (overrides preset)")
	flag.IntVar(&p.MaxIter, "iter", 0, "max iterations (overrides preset)")
	flag.Float64Var(&p.CenterX, "cx", 0, "centre real part (overrides preset)")
	flag.Float64Var(&p.CenterY, "cy", 0, "centre imaginary part (overrides preset)")
	flag.Float64Var(&p.Scale, "scale", 0, "view width (overrides preset)")
	flag.Parse()

	mode, err := engine.ParseMode(*modeName)
	pre, ok := presets[*presetName]
	if err != nil || !ok || *threads < 1 || *tile < 1 {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else if !ok {
			fmt.Fprintf(os.Stderr, "unknown preset %q\n", *presetName)
		}
		usage()
		os.Exit(2)
	}
	params := pre.params
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "w":
			params.Width = p.Width
		case "h":
			params.Height = p.Height
		case "iter":
			params.MaxIter = p.MaxIter
		case "cx":
			params.CenterX = p.CenterX
		case "cy":
			params.CenterY = p.CenterY
		case "scale":
			params.Scale = p.Scale
		}
	})

	res := engine.Run(engine.Config{Mode: mode, Threads: *threads, TileRows: *tile, Params: params})

	fmt.Printf("mode=%s threads=%d preset=%s size=%dx%d iter=%d\n", mode, len(res.Workers), *presetName, params.Width, params.Height, params.MaxIter)
	fmt.Printf("time_ms=%.2f\n", ms(res.Elapsed))
	fmt.Printf("phases_ms compute=%.2f merge=%.2f prefix=%.2f color=%.2f\n",
		ms(res.Phases.Compute), ms(res.Phases.Merge), ms(res.Phases.Prefix), ms(res.Phases.Color))
	fmt.Printf("checksum=%x\n", sha256.Sum256(res.Image.Pix))
	if *stats {
		printStats(res)
	}
	if *out != "" {
		if err := fractal.WritePNG(*out, res.Image); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func printStats(res engine.Result) {
	var total, max int64
	for _, w := range res.Workers {
		total += w.Iterations
		if w.Iterations > max {
			max = w.Iterations
		}
	}
	for i, w := range res.Workers {
		fmt.Printf("worker %2d: iterations=%d tiles=%d stolen=%d\n", i, w.Iterations, w.Tiles, w.Stolen)
	}
	fmt.Printf("balance_bound=%.2f (total work / busiest worker)\n", float64(total)/float64(max))
}
