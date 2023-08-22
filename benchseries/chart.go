// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchseries

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"

	//	"gonum.org/v1/plot/vg/vgpdf"
	//	"gonum.org/v1/plot/vg/vgsvg"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type ChartOptions int

type Point struct {
	numHash, denHash string
	values           plotter.Values
	changes          []float64
	changeBVID       []benchValID
}

// because there are holes in that data, the benchmark index can be larger than the valueIndex
type benchValID struct {
	benchIndex, valueIndex int
}

const pointRad = 5

func Chart(cs []*ComparisonSeries, pngDir, pdfDir, svgDir string, logScale bool, threshold float64, boring bool) {

	doDir := func(s string) {
		if s != "" {
			os.MkdirAll(s, 0777)
		}
	}
	doDir(pngDir)
	doDir(pdfDir)
	doDir(svgDir)

	for _, g := range cs {

		var allValues []float64

		// When boring runs are omitted, this is where the interesting parts go.
		selectedPoints := []*Point{}

		// select a subset of the points, inserting "..." whenever there is a gap.
		for i, s := range g.Series {

			values := make(plotter.Values, 0, len(g.Benchmarks))
			changes := make([]float64, 0, len(g.Benchmarks))
			changeBenches := make([]benchValID, 0, len(g.Benchmarks)) // which benchmarks changed?
			for j := range g.Benchmarks {
				sum := g.Summaries[i][j]
				if sum.Defined() {
					ch := math.NaN()
					v := sum.Center
					if math.IsInf(v, 0) {
						continue
					}
					if i > 0 {
						psum := g.Summaries[i-1][j]
						if psum.Defined() {
							ch = psum.HeurOverlap(sum, threshold)
						}
					}
					changes = append(changes, ch)
					changeBenches = append(changeBenches, benchValID{j, len(values)})
					values = append(values, v)
					allValues = append(allValues, v)
				}
			}
			hp := g.HashPairs[s]
			selectedPoints = append(selectedPoints, &Point{numHash: hp.NumHash, denHash: hp.DenHash, values: values, changes: changes, changeBVID: changeBenches})
		}

		if len(selectedPoints) == 0 {
			continue
		}

		// Want lines that grab most of the data; it's outliers we worry about
		sort.Float64s(allValues)
		lav := len(allValues)
		const Nth = 25
		lapNth := (lav + (Nth / 2)) / Nth
		minLine := allValues[lapNth]
		maxLine := allValues[lav-lapNth-1]

		pl := plot.New()

		pl.Title.Text = g.Unit
		pl.Title.TextStyle.Font.Size = 40
		pl.Y.Label.Text = "tip measure / baseline measure"

		if logScale {
			pl.Y.Scale = plot.LogScale{}
			// TODO perhaps these are not the best lines for a log scale.
			pl.Y.Tick.Marker = ratioLines(minLine, maxLine, allValues[0], allValues[lav-1])

		} else {
			pl.Y.Tick.Marker = ratioLines(minLine, maxLine, allValues[0], allValues[lav-1])
		}
		pl.Y.Tick.Label.Font.Size = 20

		grid := plotter.NewGrid()
		grid.Vertical.Color = nil
		pl.Add(grid)

		w := vg.Points(10)

		var nominalX []string
		var boxes []plot.Plotter
		for i, sp := range selectedPoints {

			perm := absSortedPermFor(sp.changes)
			rmsChange := norm(sp.changes, 2)
			sp.changes = permute(sp.changes, perm)
			l := len(sp.changes)
			movers := make(map[int]bool)
			moves := []directedColor{}
			for k := 1; k <= 5; k++ {
				if l-k < 0 {
					break
				}
				ch := sp.changes[l-k]
				c := math.Abs(ch)
				noteMove := func(clr color.Color) {
					index := sp.changeBVID[perm[l-k]].valueIndex
					bi := sp.changeBVID[perm[l-k]].benchIndex
					prevIndex := -1
					for _, psp := range selectedPoints[i-1].changeBVID {
						if psp.benchIndex == bi {
							prevIndex = psp.valueIndex
						}
					}
					// it should not be the case that a move was recorded, yet there was no match for prevIndex
					movers[index] = true
					moves = append(moves, directedColor{prev: selectedPoints[i-1].values.Value(prevIndex), index: index, change: ch, clr: clr})
				}
				if c >= 100 {
					noteMove(green(0xff))
				} else if c > 5 {
					noteMove(red(0xff))
				} else if c > 4 {
					noteMove(purple(0xff))
				} else if c > 3 {
					noteMove(blue(0xff))
				} else {
					break
				}

			}

			p := sp.values
			b, err := MyNewBoxPlot(w, float64(i), p, moves, movers)
			if err != nil {
				panic(err)
			}
			b.bp.BoxStyle.Color = color.Black
			b.bp.GlyphStyle.Radius = pointRad

			boxes = append(boxes, b)

			if rmsChange > 6 {
				b.bp.BoxStyle.Color = red(0xff)
				b.bp.FillColor = red(0x50)
			} else if rmsChange > 4 {
				b.bp.BoxStyle.Color = purple(0xFF)
				b.bp.FillColor = purple(0x50)
			} else if percentile(sp.changes, 1) > 4 {
				b.bp.BoxStyle.Color = blue(0xff)
				b.bp.FillColor = blue(0x50)
			}
			label := sp.numHash + "/" + sp.denHash

			nominalX = append(nominalX, label)
		}
		pl.Add(boxes...)
		pl.NominalX(nominalX...)

		pl.X.Tick.Width = vg.Points(0.5)
		pl.X.Tick.Length = vg.Points(8)

		pl.X.Tick.Label.Rotation = -math.Pi / 8
		pl.X.Tick.Label.YAlign = draw.YTop
		pl.X.Tick.Label.XAlign = draw.XLeft
		pl.X.Tick.Label.Font.Size = 15

		// Force the unit ratio onto the graph to ensure there is a scale.
		if pl.Y.Min > 1 {
			pl.Y.Min = 1
		}
		if pl.Y.Max < 1 {
			pl.Y.Max = 1
		}

		// Heuristic width and height
		width := 1.5 * float64(2+len(selectedPoints))
		height := width / 3
		if pl.Y.Max > 1 && pl.Y.Max-1 > 2*(math.Max(maxLine, minLine)-1) ||
			pl.Y.Min < 1 && 1-pl.Y.Min > 2*(1-math.Min(maxLine, minLine)) {
			height = height * 1.5
		}
		if height < 5 {
			height = 5
		}
		dpi := 300

		// // Override heuristics if demanded
		// if *flagWidth != 0 {
		// 	width = *flagWidth
		// }
		// if *flagHeight != 0 {
		// 	height = *flagHeight
		// }

		// Scale down dpi to conform to twitter limits
		initialWidth := float64(dpi) * width / 2.54
		if initialWidth > 8190 {
			dpi = int(math.Trunc(float64(dpi) * 8190 / initialWidth))
		}
		//fmt.Printf("%s: W=%f, H=%f, DPI=%d, PYM=%f, PYm=%f, Ml=%f, ml=%f\n", filename, width, height, dpi,
		//	p.Y.Max, p.Y.Min, maxLine, minLine)

		filename := strings.ReplaceAll(g.Unit, "/", "-per-")

		do := func(dir, sfx string, can vg.CanvasWriterTo) {
			file := filepath.Join(dir, filename) + "." + sfx
			f, err := os.Create(file)
			if err != nil {
				panic(err)
			}

			pl.Draw(draw.New(can))
			_, err = can.WriteTo(f)
			if err != nil {
				panic(err)
			}
			f.Close()
		}

		if pngDir != "" {
			do(pngDir, "png", vgimg.PngCanvas{Canvas: vgimg.NewWith(vgimg.UseWH(vg.Length(width)*vg.Centimeter, vg.Length(height)*vg.Centimeter),
				vgimg.UseDPI(dpi), vgimg.UseBackgroundColor(color.White))})
		}
		// if pdfDir != "" {
		// 	do(pdfDir, "pdf", vgpdf.Canvas{Canvas: vgimg.NewWith(vgimg.UseWH(vg.Length(width)*vg.Centimeter, vg.Length(height)*vg.Centimeter),
		// 		vgimg.UseDPI(dpi), vgimg.UseBackgroundColor(color.White))})
		// }
		// if svgDir != "" {
		// 	do(svgDir, "svg", vgsvg.Canvas{Canvas: vgimg.NewWith(vgimg.UseWH(vg.Length(width)*vg.Centimeter, vg.Length(height)*vg.Centimeter),
		// 		vgimg.UseDPI(dpi), vgimg.UseBackgroundColor(color.White))})
		// }
		// Other formats, including default PNG
		//if err := p.Save(20*vg.Inch, 5*vg.Inch, filepath.Join(dir, filename)+".png"); err != nil {
		//	panic(err)
		//}
		//if err := p.Save(20*vg.Inch, 5*vg.Inch, filepath.Join(dir, filename)+".svg"); err != nil {
		//	panic(err)
		//}
		//if err := p.Save(20*vg.Inch, 5*vg.Inch, filepath.Join(dir, filename)+".pdf"); err != nil {
		//	panic(err)
		//}
	}
}

func red(alpha uint8) color.Color {
	return color.NRGBA{0xFF, 0, 0, alpha}
}
func green(alpha uint8) color.Color {
	return color.NRGBA{0, 0xFF, 0, alpha}
}
func blue(alpha uint8) color.Color {
	return color.NRGBA{0, 0, 0xFF, alpha}
}
func purple(alpha uint8) color.Color {
	return color.NRGBA{0x99, 0, 0xFF, alpha}
}

type directedColor struct {
	prev   float64
	index  int
	change float64
	clr    color.Color
}

type MyBoxPlot struct {
	bp     *plotter.BoxPlot
	movers map[int]bool
	moves  []directedColor
}

func MyNewBoxPlot(w vg.Length, loc float64, values plotter.Valuer, moves []directedColor, movers map[int]bool) (*MyBoxPlot, error) {
	b, err := plotter.NewBoxPlot(w, loc, values)
	if err != nil {
		return nil, err
	}
	return &MyBoxPlot{bp: b, moves: moves, movers: movers}, nil
}

// Plot draws the BoxPlot on Canvas c and Plot plt.
func (p *MyBoxPlot) Plot(c draw.Canvas, plt *plot.Plot) {
	b := p.bp

	trX, trY := plt.Transforms(&c)
	x := trX(b.Location)
	px := trX(b.Location - 1)
	if !c.ContainsX(x) {
		return
	}
	x += b.Offset
	px += b.Offset

	med := trY(b.Median)
	q1 := trY(b.Quartile1)
	q3 := trY(b.Quartile3)
	aLow := trY(b.AdjLow)
	aHigh := trY(b.AdjHigh)

	pts := []vg.Point{
		{X: x - b.Width/2, Y: q1},
		{X: x - b.Width/2, Y: q3},
		{X: x + b.Width/2, Y: q3},
		{X: x + b.Width/2, Y: q1},
		{X: x - b.Width/2 - b.BoxStyle.Width/2, Y: q1},
	}
	box := c.ClipLinesY(pts)
	if b.FillColor != nil {
		c.FillPolygon(b.FillColor, c.ClipPolygonY(pts))
	}
	c.StrokeLines(b.BoxStyle, box...)

	medLine := c.ClipLinesY([]vg.Point{
		{X: x - b.Width/2, Y: med},
		{X: x + b.Width/2, Y: med},
	})
	c.StrokeLines(b.MedianStyle, medLine...)

	cap := b.CapWidth / 2
	whisks := c.ClipLinesY(
		[]vg.Point{{X: x, Y: q3}, {X: x, Y: aHigh}},
		[]vg.Point{{X: x - cap, Y: aHigh}, {X: x + cap, Y: aHigh}},
		[]vg.Point{{X: x, Y: q1}, {X: x, Y: aLow}},
		[]vg.Point{{X: x - cap, Y: aLow}, {X: x + cap, Y: aLow}},
	)
	c.StrokeLines(b.WhiskerStyle, whisks...)

	for _, out := range b.Outside {
		y := trY(b.Value(out))
		if c.ContainsY(y) {
			c.DrawGlyphNoClip(b.GlyphStyle, vg.Point{X: x, Y: y})
		}
	}

	for _, dc := range p.moves {
		clr := dc.clr
		y := trY(b.Value(dc.index))
		py := trY(dc.prev)
		if c.ContainsY(y) && c.ContainsY(py) {
			c.SetLineStyle(draw.LineStyle{Color: clr, Width: vg.Points(1)})
			p := make(vg.Path, 0, 3)
			p.Move(vg.Point{X: px, Y: py})
			p.Line(vg.Point{X: x, Y: y})
			c.Stroke(p)
		}
	}
}

func (b *MyBoxPlot) DataRange() (float64, float64, float64, float64) { return b.bp.DataRange() }
func (b *MyBoxPlot) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox       { return b.bp.GlyphBoxes(plt) }
func (b *MyBoxPlot) OutsideLabels(labels plotter.Labeller) (*plotter.Labels, error) {
	return b.bp.OutsideLabels(labels)
}

const (
	cosπover4 = vg.Length(.707106781202420)
	sinπover6 = vg.Length(.500000000025921)
	cosπover6 = vg.Length(.866025403769473)
)

// CrossGlyph is a glyph that draws a big X.
// this version draws a heavier X.
type CrossGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (CrossGlyph) DrawGlyph(c *draw.Canvas, sty draw.GlyphStyle, pt vg.Point) {
	c.SetLineStyle(draw.LineStyle{Color: sty.Color, Width: vg.Points(1)})
	r := sty.Radius * cosπover4
	p := make(vg.Path, 0, 2)
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y - r})
	p.Line(vg.Point{X: pt.X + r, Y: pt.Y + r})
	c.Stroke(p)
	p = p[:0]
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y + r})
	p.Line(vg.Point{X: pt.X + r, Y: pt.Y - r})
	c.Stroke(p)
}

type TriDown struct{}

// DrawGlyph implements the Glyph interface.
func (TriDown) DrawGlyph(c *draw.Canvas, sty draw.GlyphStyle, pt vg.Point) {
	c.SetLineStyle(draw.LineStyle{Color: sty.Color, Width: vg.Points(1)})
	r := sty.Radius * cosπover4
	p := make(vg.Path, 0, 3)
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y + r})
	p.Line(vg.Point{X: pt.X, Y: pt.Y - r})
	c.Stroke(p)
	p = p[:0]
	p.Move(vg.Point{X: pt.X + r, Y: pt.Y + r})
	p.Line(vg.Point{X: pt.X, Y: pt.Y - r})
	c.Stroke(p)
	p = p[:0]
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y})
	p.Line(vg.Point{X: pt.X + r, Y: pt.Y})
	c.Stroke(p)
}

type TriUp struct{}

// DrawGlyph implements the Glyph interface.
func (TriUp) DrawGlyph(c *draw.Canvas, sty draw.GlyphStyle, pt vg.Point) {
	c.SetLineStyle(draw.LineStyle{Color: sty.Color, Width: vg.Points(1)})
	r := sty.Radius * cosπover4
	p := make(vg.Path, 0, 3)
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y - r})
	p.Line(vg.Point{X: pt.X, Y: pt.Y + r})
	c.Stroke(p)
	p = p[:0]
	p.Move(vg.Point{X: pt.X + r, Y: pt.Y - r})
	p.Line(vg.Point{X: pt.X, Y: pt.Y + r})
	c.Stroke(p)
	p = p[:0]
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y})
	p.Line(vg.Point{X: pt.X + r, Y: pt.Y})
	c.Stroke(p)
}

type Lines struct {
	ticks []plot.Tick
}

// roundish finds a roundish fraction less than x, and the number of digits for formatting.
// x is distance from 1.0, so 1 +/- roundish(x) gives a good location for a grid line.
func roundish(x float64) (float64, int) {
	if !(x > 0) { // catch NaN also.
		panic(fmt.Sprintf("Roundish(%.9g <= 0)", x))
	}
	if x >= 1 {
		return math.Trunc(x), 0
	}
	if x >= 0.5 {
		return 0.5, 1
	}
	if x >= 0.25 {
		return 0.25, 2
	}
	if x >= 0.2 {
		return 0.2, 1
	}
	if x >= 0.1 {
		return 0.1, 1
	}
	x, n := roundish(x * 10)
	return x / 10, n + 1
}

func reverseTicks(ticks []plot.Tick) []plot.Tick {
	l := len(ticks)
	for i := 0; i < l/2; i++ {
		ticks[i], ticks[l-i-1] = ticks[l-i-1], ticks[i]
	}
	return ticks
}

func ratioLines(low, high, min, max float64) Lines {
	if high <= 1 {
		if low == 1 {
			// TODO this is a degenerate case
			return Lines{ticks: []plot.Tick{one}}
		}

		step, k := roundish(1 - low)
		var ticks []plot.Tick
		for t := 1.0; t > min; t -= step {
			ticks = append(ticks, tick(t, k))
		}

		return Lines{ticks: reverseTicks(ticks)}
	} else if low >= 1 {

		step, k := roundish(high - 1)
		k++ // for 1.frac
		var ticks []plot.Tick
		for t := 1.0; t < max; t += step {
			ticks = append(ticks, tick(t, k))
		}

		return Lines{ticks: ticks}
	}
	rmin, kmin := roundish(1 - low)
	rmax, k := roundish(high - 1)
	if rmax < rmin {
		rmax = rmin
		k = kmin
	}
	k++ // for 1.frac

	step := rmax
	var ticks []plot.Tick
	for t := 1.0; t > min; t -= step {
		ticks = append(ticks, tick(t, k))
	}
	ticks = reverseTicks(ticks)
	for t := 1.0 + step; t < max; t += step {
		ticks = append(ticks, tick(t, k))
	}

	return Lines{ticks: ticks}
}

func tick(x float64, k int) plot.Tick {
	return plot.Tick{Value: x, Label: fmt.Sprintf("%.[2]*[1]g", x, k)}
}

var one = plot.Tick{
	Value: 1.0, Label: "1.0",
}

func (u Lines) Ticks(min, max float64) []plot.Tick {
	return u.ticks
}
