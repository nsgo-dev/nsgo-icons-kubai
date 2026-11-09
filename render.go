package kubai

import (
	"math"

	"github.com/oneo-me/nsgo/core"
)

// renderer draws programs into a drawing. Without a motion value it draws
// the rest pose; with one, each number that moves is bound to a node
// following its track, so the platform plays the motion on its own frame
// clock.
type renderer struct {
	d       *core.Drawing
	palette palette
	// scale is canvas points per icon unit, for the shadow, which is in
	// the canvas's own coordinates.
	scale float32
	value *core.AnimatedValue
	nodes map[nodeKey]core.AnimatedNode
}

type nodeKey struct {
	p     *program
	track int32
	off   float32
}

// Shadow and highlight, as upstream's filters: a soft drop shadow 2 units
// down, and a light rim along a layer's top edge.
const (
	shadowOffset = 2
	shadowBlur   = 4
	shadowAlpha  = 0.21
	lightDepth   = 0.7
	lightAlpha   = 0.5
)

// node is the node a number follows, made once a drawing.
func (r *renderer) node(p *program, n num) core.AnimatedNode {
	k := nodeKey{p, n.track, n.v}
	if a, ok := r.nodes[k]; ok {
		return a
	}
	t := p.tracks[n.track]
	out := make([]float64, len(t.out))
	for i, v := range t.out {
		out[i] = v + float64(n.v)
	}
	a := core.Interpolate(r.value, core.Interpolation{Input: t.in, Output: out, Extrapolate: core.ExtrapolateClamp})
	r.nodes[k] = a
	return a
}

// bind binds argument arg of the operation just recorded to n's node, if
// n moves and the drawing plays the motion.
func (r *renderer) bind(p *program, arg core.DrawArg, n num) {
	if n.track >= 0 && r.value != nil {
		r.d.Bind(arg, r.node(p, n))
	}
}

// shifted is n moved by dy, for the highlight's copy of a path.
func shifted(n num, dy float32) num { return num{v: n.v + dy, track: n.track} }

// path adds path i to the current path, moved down by dy.
func (r *renderer) path(p *program, i int, dy float32) {
	d := r.d
	for _, s := range p.paths[i] {
		a := s.nums
		switch s.cmd {
		case 'M', 'L':
			x, y := a[0], shifted(a[1], dy)
			if s.cmd == 'M' {
				d.MoveTo(x.v, y.v)
			} else {
				d.LineTo(x.v, y.v)
			}
			r.bind(p, core.ArgX, x)
			r.bind(p, core.ArgY, y)
		case 'Q':
			cy, y := shifted(a[1], dy), shifted(a[3], dy)
			d.QuadTo(a[0].v, cy.v, a[2].v, y.v)
			r.bind(p, core.ArgX1, a[0])
			r.bind(p, core.ArgY1, cy)
			r.bind(p, core.ArgX, a[2])
			r.bind(p, core.ArgY, y)
		case 'C':
			y1, y2, y := shifted(a[1], dy), shifted(a[3], dy), shifted(a[5], dy)
			d.CubicTo(a[0].v, y1.v, a[2].v, y2.v, a[4].v, y.v)
			r.bind(p, core.ArgX1, a[0])
			r.bind(p, core.ArgY1, y1)
			r.bind(p, core.ArgX2, a[2])
			r.bind(p, core.ArgY2, y2)
			r.bind(p, core.ArgX, a[4])
			r.bind(p, core.ArgY, y)
		case 'Z':
			d.ClosePath()
		}
	}
}

func (r *renderer) gradient(p *program, pt paint) *core.CanvasGradient {
	g := p.gradients[pt.gradient]
	stops := make([]core.GradientStop, len(g.offsets))
	for i := range stops {
		stops[i] = core.GradientStop{Offset: g.offsets[i], Color: r.palette.color(g.colors[i])}
	}
	return core.CanvasLinear(pt.x0, pt.y0, pt.x1, pt.y1, stops...)
}

func (r *renderer) fillPaint(p *program, pt paint) {
	if pt.gradient >= 0 {
		r.d.SetFillGradient(r.gradient(p, pt))
		return
	}
	r.d.SetFillColor(r.palette.color(pt.color))
}

func (r *renderer) strokePaint(p *program, pt paint) {
	if pt.gradient >= 0 {
		r.d.SetStrokeGradient(r.gradient(p, pt))
		return
	}
	r.d.SetStrokeColor(r.palette.color(pt.color))
}

func (r *renderer) fill(rule core.FillRule) {
	if rule == core.EvenOdd {
		r.d.FillEvenOdd()
	} else {
		r.d.Fill()
	}
}

func (r *renderer) clip(rule core.FillRule) {
	if rule == core.EvenOdd {
		r.d.ClipEvenOdd()
	} else {
		r.d.Clip()
	}
}

// Outlines and badges clip to all but a shape: this rectangle, well past
// the canvas, with the shape cut out of it.
const outside = 40

func (r *renderer) draw(p *program) {
	d := r.d
	for _, o := range p.ops {
		switch o.kind {
		case 'S':
			d.Save()
		case 'R':
			d.Restore()
		case 'X':
			a := o.nums
			d.Transform(a[0].v, a[1].v, a[2].v, a[3].v, a[4].v, a[5].v)
			for i, arg := range []core.DrawArg{core.ArgX1, core.ArgY1, core.ArgX2, core.ArgY2, core.ArgX, core.ArgY} {
				r.bind(p, arg, a[i])
			}
		case 'A':
			d.SetAlpha(o.nums[0].v)
			r.bind(p, core.ArgA, o.nums[0])
		case 'W':
			sc := r.palette.color(colorRef{kind: 't', src: core.RGB(0x8AB6FF), mode: 'c'})
			d.SetShadow(sc.Faded(shadowAlpha), 0, shadowOffset*r.scale, shadowBlur*r.scale)
		case 'w':
			d.SetShadow(0, 0, 0, 0)
		case 'F':
			r.fillPaint(p, o.paint)
			d.BeginPath()
			r.path(p, o.path, 0)
			r.fill(o.rule)
		case 'K':
			r.strokePaint(p, o.paint)
			d.SetLineWidth(o.nums[0].v)
			r.bind(p, core.ArgW, o.nums[0])
			d.SetLineCap(o.cap)
			d.SetLineJoin(o.join)
			d.SetLineDash(o.dash...)
			if len(o.dash) > 0 {
				d.SetLineDashOffset(o.nums[1].v)
				r.bind(p, core.ArgW, o.nums[1])
			}
			d.BeginPath()
			r.path(p, o.path, 0)
			d.Stroke()
		case 'c':
			d.BeginPath()
			r.path(p, o.path, 0)
			r.clip(o.rule)
		case 'o':
			d.BeginPath()
			d.Rect(-outside, -outside, 31+2*outside, 31+2*outside)
			r.path(p, o.path, 0)
			d.ClipEvenOdd()
		case 'H':
			// The highlight: the layer less its copy moved down, lightened.
			d.Save()
			d.BeginPath()
			r.path(p, o.path, 0)
			r.clip(o.rule)
			d.BeginPath()
			r.path(p, o.path, 0)
			r.path(p, o.path, lightDepth)
			d.SetFillColor(core.RGB(0xFFFFFF).Faded(lightAlpha))
			d.SetComposite(core.CompositeOverlay)
			d.FillEvenOdd()
			d.Restore()
		case 'T':
			r.fillPaint(p, o.paint)
			d.SetTextAlign(o.align)
			d.FillText(o.text, o.nums[0].v, o.nums[1].v, o.font)
			r.bind(p, core.ArgX, o.nums[0])
			r.bind(p, core.ArgY, o.nums[1])
		}
	}
}

// gap clips to all but the circle of radius g round the origin, which
// follows track g of the badge's pop.
func (r *renderer) gap(b *badge, x, y float32) {
	d := r.d
	d.BeginPath()
	d.Rect(-outside-x, -outside-y, 31+2*outside, 31+2*outside)
	var a core.AnimatedNode
	if r.value != nil {
		k := nodeKey{b.prog, -1, 0}
		var ok bool
		if a, ok = r.nodes[k]; !ok {
			a = core.Interpolate(r.value, core.Interpolation{Input: b.gap.in, Output: b.gap.out, Extrapolate: core.ExtrapolateClamp})
			r.nodes[k] = a
		}
	}
	rest := float32(b.gap.out[0])
	d.MoveTo(rest, 0)
	if a != nil {
		d.Bind(core.ArgX, a)
	}
	d.Arc(0, 0, rest, 0, 2*math.Pi, false)
	if a != nil {
		d.Bind(core.ArgR, a)
	}
	d.ClipEvenOdd()
}

// symbol draws s: its program, or its base with the badge over the corner
// and a gap cut round it.
func (r *renderer) symbol(s *Symbol) {
	if s.prog != nil {
		r.draw(s.prog)
		return
	}
	d := r.d
	x, y := s.at[0], s.at[1]
	d.Save()
	d.Translate(x, y)
	r.gap(s.badge, x, y)
	d.Translate(-x, -y)
	r.symbol(s.base)
	d.Restore()
	d.Save()
	d.Translate(x, y)
	r.draw(s.badge.prog)
	d.Restore()
}
