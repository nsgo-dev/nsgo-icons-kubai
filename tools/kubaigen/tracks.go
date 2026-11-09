package main

import (
	"math"
	"strconv"
	"strings"
)

// The motion: 460 ms on cubic-bezier(.22, 1, .36, 1), as upstream's.
func bezier(x1, y1, x2, y2 float64) func(float64) float64 {
	cx := 3 * x1
	bx := 3*(x2-x1) - cx
	ax := 1 - cx - bx
	cy := 3 * y1
	by := 3*(y2-y1) - cy
	ay := 1 - cy - by
	sx := func(t float64) float64 { return ((ax*t+bx)*t + cx) * t }
	sy := func(t float64) float64 { return ((ay*t+by)*t + cy) * t }
	dx := func(t float64) float64 { return (3*ax*t+2*bx)*t + cx }
	return func(x float64) float64 {
		if x <= 0 {
			return 0
		}
		if x >= 1 {
			return 1
		}
		t := x
		for range 8 {
			e := sx(t) - x
			if math.Abs(e) < 1e-5 {
				break
			}
			d := dx(t)
			if math.Abs(d) < 1e-6 {
				break
			}
			t -= e / d
		}
		return sy(t)
	}
}

var ease = bezier(0.22, 1, 0.36, 1)

// samples is the moments, from 0 to 1, each direction of a motion is
// rendered at.
func samples(n int) []float64 {
	ts := make([]float64, n+1)
	for i := range ts {
		ts[i] = float64(i) / float64(n)
	}
	return ts
}

// program is an icon compiled across the frames of its motion: letters
// and numbers, with "@k" for a number that follows track k, and the
// tracks after a bar.
type program struct {
	text   string
	tracks int
	// variants is how many forms the icon takes across its motion.
	variants int
}

func skeleton(ts []tok) string {
	var b strings.Builder
	for _, t := range ts {
		if t.kind == slotSym {
			b.WriteString(t.s)
		} else {
			b.WriteByte('#')
		}
		b.WriteByte(' ')
	}
	return b.String()
}

// link compiles the frames of a motion: the way in at moments ts, then the
// way out. Frames whose letters differ are told apart as variants, each
// drawn only while the motion is in one of its frames.
func link(frames []string, ts []float64) (program, map[string]bool, error) {
	warn := map[string]bool{}
	roots := make([]*element, len(frames))
	union := map[string]map[string]bool{}
	for i, f := range frames {
		r, err := parseMarkup(f)
		if err != nil {
			return program{}, nil, err
		}
		roots[i] = r
		collectUnion(r, union)
	}
	compiled := make([][]tok, len(frames))
	for i, r := range roots {
		t, err := compileFrame(r, union, warn)
		if err != nil {
			return program{}, nil, err
		}
		compiled[i] = t
	}
	n := len(ts)
	xs := frameXs(ts)
	groups := map[string][]int{}
	var order []string
	for i, t := range compiled {
		k := skeleton(t)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}
	tr := &trackSet{index: map[string]int{}}
	var text strings.Builder
	for _, k := range order {
		members := groups[k]
		in := map[int]bool{}
		for _, i := range members {
			in[i] = true
		}
		tmpl := compiled[members[0]]
		for j, t := range tmpl {
			if j > 0 {
				text.WriteByte(' ')
			}
			if t.kind == slotSym {
				text.WriteString(t.s)
				continue
			}
			if t.kind == slotRest {
				text.WriteString(fmtNum(t.v, 3))
				continue
			}
			ys := make([]float64, len(frames))
			for i := range frames {
				if in[i] {
					ys[i] = compiled[i][j].v
					continue
				}
				if t.kind == slotAlpha {
					ys[i] = 0
					continue
				}
				ys[i] = compiled[nearest(members, i)][j].v
			}
			prec := 3
			if t.kind == slotLinear {
				prec = 4
			}
			text.WriteString(tr.arg(xs, ys, n, prec))
		}
		text.WriteByte(' ')
	}
	out := strings.TrimSpace(optimizeAlpha(text.String()))
	if len(tr.list) > 0 {
		out += " | " + strings.Join(tr.list, " ; ")
	}
	return program{text: out, tracks: len(tr.list), variants: len(order)}, warn, nil
}

// frameXs is where each frame is on the motion value: 0 to 1 on the way
// in, 1 to 2 on the way out.
func frameXs(ts []float64) []float64 {
	xs := make([]float64, 2*len(ts))
	for i := range xs {
		e := ease(ts[i%len(ts)])
		if i >= len(ts) {
			e++
		}
		xs[i] = e
	}
	return xs
}

func nearest(members []int, i int) int {
	best, d := members[0], math.MaxInt
	for _, m := range members {
		if dd := abs(m - i); dd < d {
			best, d = m, dd
		}
	}
	return best
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func fmtNum(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

// trackSet holds an icon's tracks, each written once.
type trackSet struct {
	list  []string
	index map[string]int
}

// arg writes a number that is the same in every frame, or the track it
// follows. Tracks that differ by a constant, as the points of a shape that
// moves as one do, share their points: "@k+c" is track k moved by c.
func (tr *trackSet) arg(xs, ys []float64, n, prec int) string {
	tol := 0.008
	if prec == 4 {
		tol = 0.001
	}
	lo, hi := ys[0], ys[0]
	for _, y := range ys {
		lo, hi = min(lo, y), max(hi, y)
	}
	if hi-lo < tol/4 {
		return fmtNum(ys[0], prec)
	}
	base := ys[0]
	moved := make([]float64, len(ys))
	for i, y := range ys {
		moved[i] = y - base
	}
	enc := encodeTrack(xs, moved, n, tol, prec)
	k, ok := tr.index[enc]
	if !ok {
		k = len(tr.list)
		tr.index[enc] = k
		tr.list = append(tr.list, enc)
	}
	ref := "@" + strconv.Itoa(k)
	if off := fmtNum(base, prec); off != "0" {
		if !strings.HasPrefix(off, "-") {
			off = "+" + off
		}
		ref += off
	}
	return ref
}

type pt struct {
	i    int
	x, y float64
}

// encodeTrack writes the points of a track simplified to within tol, each
// as the frame it was sampled at and its value. A track whose way out
// retraces its way in is written as the way in after "~", and one that
// also runs straight from rest to hover as "=" and its two ends.
func encodeTrack(xs, ys []float64, n int, tol float64, prec int) string {
	in := dedupe(xs[:n], ys[:n], 0)
	out := dedupe(xs[n:], ys[n:], n)
	simpleIn := simplify(in, tol)
	mirrored := true
	for _, p := range out {
		if math.Abs(p.y-at(simpleIn, 2-p.x)) > tol {
			mirrored = false
			break
		}
	}
	var b strings.Builder
	write := func(ps []pt) {
		for i, p := range ps {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strconv.Itoa(p.i))
			b.WriteByte(' ')
			b.WriteString(fmtNum(p.y, prec))
		}
	}
	if mirrored {
		if len(simpleIn) == 2 && simpleIn[0].i == 0 && simpleIn[1].i == n-1 {
			return "=" + fmtNum(simpleIn[0].y, prec) + " " + fmtNum(simpleIn[1].y, prec)
		}
		b.WriteString("~")
		write(simpleIn)
		return b.String()
	}
	all := append(append([]pt{}, in...), out[1:]...)
	write(simplify(all, tol))
	return b.String()
}

// dedupe drops samples at a moment the one before already holds, where the
// curve has settled closer to its end than floating point tells apart.
func dedupe(xs, ys []float64, first int) []pt {
	var ps []pt
	for i := range xs {
		if len(ps) > 0 && xs[i] <= ps[len(ps)-1].x {
			continue
		}
		ps = append(ps, pt{first + i, xs[i], ys[i]})
	}
	return ps
}

// at reads a polyline at x.
func at(ps []pt, x float64) float64 {
	if x <= ps[0].x {
		return ps[0].y
	}
	for i := 1; i < len(ps); i++ {
		if x <= ps[i].x {
			a, b := ps[i-1], ps[i]
			return a.y + (b.y-a.y)*(x-a.x)/(b.x-a.x)
		}
	}
	return ps[len(ps)-1].y
}

// simplify keeps the fewest points that stay within tol of every sample,
// measured along y (Ramer–Douglas–Peucker).
func simplify(ps []pt, tol float64) []pt {
	if len(ps) <= 2 {
		return ps
	}
	keep := make([]bool, len(ps))
	keep[0], keep[len(ps)-1] = true, true
	var rec func(a, b int)
	rec = func(a, b int) {
		worst, at := 0.0, -1
		for i := a + 1; i < b; i++ {
			y := ps[a].y + (ps[b].y-ps[a].y)*(ps[i].x-ps[a].x)/(ps[b].x-ps[a].x)
			if d := math.Abs(ps[i].y - y); d > worst {
				worst, at = d, i
			}
		}
		if worst > tol {
			keep[at] = true
			rec(a, at)
			rec(at, b)
		}
	}
	rec(0, len(ps)-1)
	var out []pt
	for i, p := range ps {
		if keep[i] {
			out = append(out, p)
		}
	}
	return out
}

// optimizeAlpha drops opacities that set what is already set.
func optimizeAlpha(text string) string {
	fields := strings.Fields(text)
	var out []string
	stack := []string{"1"}
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch f {
		case "S":
			stack = append(stack, stack[len(stack)-1])
		case "R":
			stack = stack[:len(stack)-1]
		case "A":
			v := fields[i+1]
			if !strings.HasPrefix(v, "@") && v == stack[len(stack)-1] {
				i++
				continue
			}
			stack[len(stack)-1] = v
			if strings.HasPrefix(v, "@") {
				// A track may hold any value: the next opacity is written.
				stack[len(stack)-1] = "?"
			}
		case "T":
			// A text's quoted string may hold spaces.
			out = append(out, f)
			j := i + 1
			for ; j < len(fields); j++ {
				out = append(out, fields[j])
				if strings.HasPrefix(fields[j], "\"") && len(fields[j]) > 1 && strings.HasSuffix(fields[j], "\"") {
					break
				}
			}
			i = j
			continue
		}
		out = append(out, f)
	}
	return strings.Join(out, " ")
}

