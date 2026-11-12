package kubai

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
)

// A program is an icon's layers as canvas operations, compiled by
// kubaigen from upstream's renderer. Its numbers are constants or follow
// tracks: what a number does over the motion value, 0 at rest, 1 at the
// end of the hover and 2 at rest again after the way out.

// num is a number of a program: v, plus track's value when track is not
// -1. Tracks are 0 at rest, so v is the number at rest.
type num struct {
	v     float32
	track int32
}

// colorRef is a color of the template paint, resolved against the
// accent when drawn.
type colorRef struct {
	// kind is '#' for src itself, 't' for src moved to the accent, 'w' for
	// the white of marks and 'p' for the top of a solid's gradient over
	// inner.
	kind byte
	src  core.Color
	// mode is 'c' for the fill accent, 'e' for the edge's and 'b' for the
	// badge's, ink when no badge color is set.
	mode  byte
	ink   core.Color
	inner *colorRef
}

// paint is a color or a linear gradient from (x0, y0) to (x1, y1).
type paint struct {
	color          colorRef
	gradient       int
	x0, y0, x1, y1 float32
}

type gradient struct {
	offsets []float32
	colors  []colorRef
}

type seg struct {
	cmd  byte
	nums []num
}

type op struct {
	kind  byte
	path  int
	paint paint
	rule  core.FillRule
	nums  []num
	cap   core.LineCap
	join  core.LineJoin
	dash  []float32
	text  string
	font  core.Font
	align core.TextAlign
}

type track struct{ in, out []float64 }

type program struct {
	gradients []gradient
	paths     [][]seg
	ops       []op
	tracks    []track
}

// motionSamples is how many steps of time kubaigen rendered each direction
// of the motion at; a track's points name those frames.
const motionSamples = 48

// frameX is where frame i is on the motion value.
func frameX(i int) float64 {
	n := motionSamples + 1
	if i < n {
		return ease(float64(i) / motionSamples)
	}
	return 1 + ease(float64(i-n)/motionSamples)
}

type parser struct {
	f []string
	i int
}

func (p *parser) next() string {
	if p.i >= len(p.f) {
		panic("kubai: program ends early")
	}
	p.i++
	return p.f[p.i-1]
}

func (p *parser) float() float32 {
	v, err := strconv.ParseFloat(p.next(), 32)
	if err != nil {
		panic(fmt.Sprintf("kubai: %v", err))
	}
	return float32(v)
}

func (p *parser) int() int {
	v, err := strconv.Atoi(p.next())
	if err != nil {
		panic(fmt.Sprintf("kubai: %v", err))
	}
	return v
}

func (p *parser) num() num {
	s := p.next()
	if !strings.HasPrefix(s, "@") {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			panic(fmt.Sprintf("kubai: %v", err))
		}
		return num{v: float32(v), track: -1}
	}
	s = s[1:]
	off := 0.0
	if i := strings.IndexAny(s, "+-"); i >= 0 {
		var err error
		if off, err = strconv.ParseFloat(s[i:], 32); err != nil {
			panic(fmt.Sprintf("kubai: %v", err))
		}
		s = s[:i]
	}
	k, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Sprintf("kubai: %v", err))
	}
	return num{v: float32(off), track: int32(k)}
}

func hexColor(s string) core.Color {
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil || len(s) != 6 {
		panic(fmt.Sprintf("kubai: color %q", s))
	}
	return core.RGB(uint32(v))
}

func parseColor(s string) colorRef {
	switch {
	case s == "w":
		return colorRef{kind: 'w'}
	case strings.HasPrefix(s, "p"):
		inner := parseColor(s[1:])
		return colorRef{kind: 'p', inner: &inner}
	case strings.HasPrefix(s, "#"):
		return colorRef{kind: '#', src: hexColor(s[1:])}
	case strings.HasPrefix(s, "t") && len(s) >= 8:
		c := colorRef{kind: 't', src: hexColor(s[1:7]), mode: s[7]}
		if c.mode == 'b' {
			c.ink = hexColor(s[8:])
		}
		return c
	}
	panic(fmt.Sprintf("kubai: color %q", s))
}

func (p *parser) paint() paint {
	s := p.next()
	if strings.HasPrefix(s, "g") {
		g, err := strconv.Atoi(s[1:])
		if err != nil {
			panic(fmt.Sprintf("kubai: %v", err))
		}
		return paint{gradient: g, x0: p.float(), y0: p.float(), x1: p.float(), y1: p.float()}
	}
	return paint{color: parseColor(s), gradient: -1}
}

func (p *parser) rule() core.FillRule {
	if p.next() == "e" {
		return core.EvenOdd
	}
	return core.NonZero
}

// parseProgram reads a program kubaigen wrote.
func parseProgram(src string) *program {
	body, tracks, _ := strings.Cut(src, " | ")
	pr := &program{}
	p := &parser{f: strings.Fields(body)}
	for p.i < len(p.f) {
		k := p.next()
		switch k {
		case "G":
			n := p.int()
			g := gradient{}
			for range n {
				g.offsets = append(g.offsets, p.float())
				g.colors = append(g.colors, parseColor(p.next()))
			}
			pr.gradients = append(pr.gradients, g)
		case "P":
			var path []seg
			for {
				c := p.next()
				if c == ";" {
					break
				}
				width := map[string]int{"M": 2, "L": 2, "Q": 4, "C": 6, "Z": 0}[c]
				s := seg{cmd: c[0]}
				for range width {
					s.nums = append(s.nums, p.num())
				}
				path = append(path, s)
			}
			pr.paths = append(pr.paths, path)
		case "S", "R", "W", "w":
			pr.ops = append(pr.ops, op{kind: k[0]})
		case "X":
			o := op{kind: 'X'}
			for range 6 {
				o.nums = append(o.nums, p.num())
			}
			pr.ops = append(pr.ops, o)
		case "A":
			pr.ops = append(pr.ops, op{kind: 'A', nums: []num{p.num()}})
		case "F":
			pr.ops = append(pr.ops, op{kind: 'F', path: p.int(), paint: p.paint(), rule: p.rule()})
		case "H", "c", "o":
			pr.ops = append(pr.ops, op{kind: k[0], path: p.int(), rule: p.rule()})
		case "K":
			o := op{kind: 'K', path: p.int(), paint: p.paint(), nums: []num{p.num()}}
			cj := p.next()
			o.cap = map[byte]core.LineCap{'b': core.CapButt, 'r': core.CapRound, 's': core.CapSquare}[cj[0]]
			o.join = map[byte]core.LineJoin{'m': core.JoinMiter, 'r': core.JoinRound, 'v': core.JoinBevel}[cj[1]]
			if n := p.int(); n > 0 {
				for range n {
					o.dash = append(o.dash, p.float())
				}
				o.nums = append(o.nums, p.num())
			}
			pr.ops = append(pr.ops, o)
		case "T":
			o := op{kind: 'T', nums: []num{p.num(), p.num()}}
			o.font = core.Font{Size: core.Points(p.float()), Weight: core.FontWeight(p.int())}
			o.align = map[string]core.TextAlign{"s": core.TextAlignStart, "m": core.TextAlignCenter, "e": core.TextAlignEnd}[p.next()]
			o.paint = p.paint()
			// The text is quoted and may hold spaces.
			q := p.next()
			for !(len(q) > 1 && strings.HasSuffix(q, `"`) && !strings.HasSuffix(q, `\"`)) {
				q += " " + p.next()
			}
			text, err := strconv.Unquote(q)
			if err != nil {
				panic(fmt.Sprintf("kubai: text %s", q))
			}
			o.text = text
			pr.ops = append(pr.ops, o)
		default:
			panic(fmt.Sprintf("kubai: operation %q", k))
		}
	}
	if tracks != "" {
		for _, t := range strings.Split(tracks, " ; ") {
			pr.tracks = append(pr.tracks, parseTrack(t))
		}
	}
	return pr
}

// parseTrack reads a track: frame and value pairs, the way in alone after
// "~" when the way out retraces it, or "=" and the values at rest and at
// the end of the hover when it also runs straight between them.
func parseTrack(s string) track {
	fl := func(s string) float64 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic(fmt.Sprintf("kubai: %v", err))
		}
		return v
	}
	if rest, ok := strings.CutPrefix(s, "="); ok {
		f := strings.Fields(rest)
		a, b := fl(f[0]), fl(f[1])
		return track{in: []float64{0, 1, 2}, out: []float64{a, b, a}}
	}
	mirror := strings.HasPrefix(s, "~")
	f := strings.Fields(strings.TrimPrefix(s, "~"))
	var xs, ys []float64
	for i := 0; i+1 < len(f); i += 2 {
		k, err := strconv.Atoi(f[i])
		if err != nil {
			panic(fmt.Sprintf("kubai: %v", err))
		}
		xs = append(xs, frameX(k))
		ys = append(ys, fl(f[i+1]))
	}
	if mirror {
		for i := len(xs) - 2; i >= 0; i-- {
			xs = append(xs, 2-xs[i])
			ys = append(ys, ys[i])
		}
	}
	return track{in: xs, out: ys}
}

// at is the track's value at motion value x.
func (t track) at(x float64) float64 {
	if x <= t.in[0] {
		return t.out[0]
	}
	for i := 1; i < len(t.in); i++ {
		if x <= t.in[i] {
			a, b := t.in[i-1], t.in[i]
			return t.out[i-1] + (t.out[i]-t.out[i-1])*(x-a)/(b-a)
		}
	}
	return t.out[len(t.out)-1]
}

// palette is the colors an icon resolves its template paint to.
type palette struct {
	fill, edge, badge core.Color
	deep              bool
}

func rgb(c core.Color) [3]float64 {
	r, g, b, _ := c.Components()
	return [3]float64{r * 255, g * 255, b * 255}
}

func fromRGB(v [3]float64) core.Color {
	var hex uint32
	for _, c := range v {
		hex = hex<<8 | uint32(math.Max(0, math.Min(255, math.Round(c))))
	}
	return core.RGB(hex)
}

// The reference blue every color of an icon is written against.
var ref = [3]float64{83, 147, 255}

// tint moves src, a color written against the reference blue, to sel by
// its relative lightness, as upstream's tint.
func tint(src, sel core.Color) core.Color {
	s, t := rgb(src), rgb(sel)
	var out [3]float64
	for i := range out {
		var amt float64
		if i == 2 {
			amt = (s[0] - ref[0]) / (255 - ref[0])
		} else {
			amt = (s[i] - ref[i]) / (255 - ref[i])
		}
		if amt >= 0 {
			out[i] = t[i] + (255-t[i])*amt
		} else {
			out[i] = t[i] * (1 + amt)
		}
	}
	return fromRGB(out)
}

// soften lifts a color toward white by k.
func soften(c core.Color, k float64) core.Color {
	v := rgb(c)
	for i := range v {
		v[i] += (255 - v[i]) * k
	}
	return fromRGB(v)
}

func darken(c core.Color, k float64) core.Color {
	v := rgb(c)
	for i := range v {
		v[i] *= 1 - k
	}
	return fromRGB(v)
}

func (pl palette) color(c colorRef) core.Color {
	switch c.kind {
	case 't':
		switch c.mode {
		case 'e':
			return tint(c.src, pl.edge)
		case 'b':
			if pl.badge != 0 {
				return tint(c.src, pl.badge)
			}
			return tint(c.src, c.ink)
		}
		return tint(c.src, pl.fill)
	case 'w':
		if pl.deep {
			return tint(core.RGB(0x2F6FE0), pl.edge)
		}
		return core.RGB(0xFFFFFF)
	case 'p':
		base := pl.color(*c.inner)
		if pl.deep {
			return soften(base, 0.72)
		}
		return base
	}
	return c.src
}
