// Package fractal contains the Mandelbrot kernels: escape-time computation,
// histogram merging, and histogram-equalised colouring. It has no knowledge
// of threads; the engine package decides how to schedule these kernels.
package fractal

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// Params describes the region of the complex plane and the image to render.
type Params struct {
	Width, Height, MaxIter int
	CenterX, CenterY       float64
	Scale                  float64 // width of the view in the complex plane
}

// Canvas holds per-pixel escape iteration counts for one render.
type Canvas struct {
	P     Params
	Iters []int32
}

// NewCanvas allocates a canvas for p.
func NewCanvas(p Params) *Canvas {
	return &Canvas{P: p, Iters: make([]int32, p.Width*p.Height)}
}

// NewHistogram returns a zeroed histogram with one bin per iteration count.
func (p Params) NewHistogram() []int64 { return make([]int64, p.MaxIter+1) }

func escape(cr, ci float64, maxIter int) int {
	var zr, zi float64
	for n := 0; n < maxIter; n++ {
		zr2, zi2 := zr*zr, zi*zi
		if zr2+zi2 > 4 {
			return n
		}
		zi = 2*zr*zi + ci
		zr = zr2 - zi2 + cr
	}
	return maxIter
}

// ComputeRows fills rows [y0,y1) of the canvas, adds every pixel to hist, and
// returns the total number of iterations performed (a measure of work done).
func (c *Canvas) ComputeRows(y0, y1 int, hist []int64) int64 {
	p := c.P
	step := p.Scale / float64(p.Width)
	left := p.CenterX - p.Scale/2
	var work int64
	for y := y0; y < y1; y++ {
		ci := p.CenterY + (float64(y)-float64(p.Height)/2)*step
		row := c.Iters[y*p.Width : (y+1)*p.Width]
		for x := range row {
			n := escape(left+float64(x)*step, ci, p.MaxIter)
			row[x] = int32(n)
			hist[n]++
			work += int64(n)
		}
	}
	return work
}

// MergeHist sets dst[i] = sum of srcs[k][i] for bins i in [lo,hi).
func MergeHist(dst []int64, srcs [][]int64, lo, hi int) {
	for i := lo; i < hi; i++ {
		var sum int64
		for _, s := range srcs {
			sum += s[i]
		}
		dst[i] = sum
	}
}

// Cumulative returns cum, where cum[n] counts pixels escaping before
// iteration n, and total, the number of pixels outside the set.
func Cumulative(hist []int64, maxIter int) (cum []int64, total int64) {
	cum = make([]int64, maxIter)
	for n := 0; n < maxIter; n++ {
		cum[n] = total
		total += hist[n]
	}
	return cum, total
}

func palette(t float64) color.RGBA {
	u := 1 - t
	return color.RGBA{
		R: uint8(9 * u * t * t * t * 255),
		G: uint8(15 * u * u * t * t * 255),
		B: uint8(8.5 * u * u * u * t * 255),
		A: 255,
	}
}

// ColorRows colours rows [y0,y1) of img using histogram equalisation, so
// colour bands are spread evenly over the pixels that escaped.
func (c *Canvas) ColorRows(y0, y1 int, cum []int64, total int64, img *image.RGBA) {
	w := c.P.Width
	for y := y0; y < y1; y++ {
		for x := 0; x < w; x++ {
			n := int(c.Iters[y*w+x])
			col := color.RGBA{A: 255}
			if n < c.P.MaxIter && total > 0 {
				col = palette(math.Sqrt(float64(cum[n]) / float64(total)))
			}
			img.SetRGBA(x, y, col)
		}
	}
}

// WritePNG saves img to path.
func WritePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
