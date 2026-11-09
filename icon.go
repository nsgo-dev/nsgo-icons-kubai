package kubai

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// Variant is an icon's look.
type Variant uint8

const (
	// VariantColor is the default look: fills in the color, white marks.
	VariantColor Variant = iota
	// VariantSoft is the clear look: fills lifted toward white, outlines a
	// step deeper and marks in the deep tone.
	VariantSoft
)

// Reference is the blue every icon is drawn in by default, and every color
// of an icon is written against.
var Reference = core.RGB(0x5393FF)

// Icon draws a Kubai icon, moving on hover: a folder opens, a bell rings.
// It moves when the mouse is over it, or as Hovered says, so a button can
// play its icon when the button is hovered.
//
//	kubai.Icon{Symbol: kubai.Folder(), Size: 32, Color: core.RGB(0x6366F1)}
//
// The motion runs on the platform's frame clock: a hover costs one commit
// and none per frame. An icon that has not moved yet draws its rest pose
// alone and holds no animated nodes. Under reduced motion it jumps to its
// pose.
type Icon struct {
	Symbol *Symbol
	// Size is the icon's width and height; 0 is 24.
	Size core.Points
	// Color is the icon's accent, the color of its body; 0 is Reference.
	Color core.Color
	// BadgeColor is the badge's disc; 0 is the neutral ink. Symbol.Tone is
	// the badge's own color.
	BadgeColor core.Color
	Variant    Variant
	// Hovered, when set, plays the motion while it reports true, instead
	// of the mouse over the icon.
	Hovered func() bool
	// Label names the icon for assistive technology; without it the icon
	// is a decoration.
	Label  string
	TestID string
}

// Motion, as upstream's: 460 ms on cubic-bezier(.22, 1, .36, 1).
const duration = 460 * time.Millisecond

var easeCurve = core.Curve{X1: 0.22, Y1: 1, X2: 0.36, Y2: 1}

// ease is the motion's curve.
func ease(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	const x1, y1, x2, y2 = 0.22, 1, 0.36, 1
	cx := 3.0 * x1
	bx := 3*(x2-x1) - cx
	ax := 1 - cx - bx
	cy := 3.0 * y1
	by := 3*(y2-y1) - cy
	ay := 1 - cy - by
	t := x
	for range 8 {
		e := ((ax*t+bx)*t+cx)*t - x
		if math.Abs(e) < 1e-5 {
			break
		}
		d := (3*ax*t+2*bx)*t + cx
		if math.Abs(d) < 1e-6 {
			break
		}
		t -= e / d
	}
	return ((ay*t+by)*t + cy) * t
}

// The canvas reaches past the icon's box by pad units on every side, for
// the shadow, over upstream's 31-unit view box from (0, -3).
const (
	viewBox = 31
	pad     = 2.5
)

// motion is an icon's place on its motion value: 0 at rest, 1 hovered and
// 2 at rest again after the way out. It runs on the platform; the engine
// knows where it is from when and where it started.
type motion struct {
	value    *core.AnimatedValue
	from, to float64
	start    time.Time
}

func (m *motion) at(now time.Time) float64 {
	k := float64(now.Sub(m.start)) / float64(duration)
	return m.from + (m.to-m.from)*ease(min(1, k))
}

// hover moves the icon toward its hover pose or back to rest, from the
// pose it shows. live turns on with the first move, and the drawing binds
// its numbers from then on.
func (m *motion) hover(cx *core.Scope, on bool, live *core.Signal[bool]) {
	now := cx.Now()
	cur := m.at(now)
	target := 2.0
	if on {
		target = 1
	}
	if cur == target || cur == 0 && !on {
		return
	}
	live.Set(true)
	// The way in and the way out are two halves of the value: a pose on
	// one half is the same pose at the mirrored place on the other.
	switch {
	case on && cur >= 2:
		cur = 0
	case on && cur > 1, !on && cur < 1:
		cur = 2 - cur
	}
	if cx.Env().ReduceMotion() {
		m.value.Set(target)
		m.from, m.to, m.start = target, target, now
		return
	}
	m.value.Set(cur)
	m.from, m.to, m.start = cur, target, now
	cx.StartAnimation(core.Timing(m.value, target, core.TimingConfig{Duration: duration, Easing: core.BezierEasing(easeCurve)}), nil)
}

// Render implements core.Node.
func (i Icon) Render(cx *core.Scope) core.Node {
	size := i.Size
	if size == 0 {
		size = 24
	}
	accent := i.Color
	if accent == 0 {
		accent = Reference
	}
	pl := palette{fill: accent, edge: accent, badge: i.BadgeColor}
	if i.Variant == VariantSoft {
		pl = palette{fill: soften(accent, 0.1), edge: darken(accent, 0.16), badge: i.BadgeColor, deep: true}
	}
	sym := i.Symbol
	live := core.NewSignal(cx, false)
	m := &motion{value: core.NewAnimatedValue(cx, 0)}
	hover := func(on bool) { cx.Untrack(func() { m.hover(cx, on, live) }) }
	if i.Hovered != nil {
		cx.Effect(func() { hover(i.Hovered()) })
	}
	full := size * (viewBox + 2*pad) / viewBox
	canvas := core.Canvas{
		Label:  i.Label,
		TestID: i.TestID,
		Style: core.Style{Position: core.Absolute, Left: core.Pt(-(full - size) / 2), Top: core.Pt(-(full - size) / 2),
			Width: core.Pt(full), Height: core.Pt(full)},
		Draw: func(d *core.Drawing, box core.Size) {
			if sym == nil {
				return
			}
			k := float32(box.Width) / (viewBox + 2*pad)
			r := &renderer{d: d, palette: pl, scale: k}
			if live.Get() {
				r.value, r.nodes = m.value, map[nodeKey]core.AnimatedNode{}
			}
			d.Save()
			d.Scale(k, k)
			d.Translate(pad, 3+pad)
			r.symbol(sym)
			d.Restore()
		},
	}
	v := core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)}, Children: []core.Node{canvas}}
	if i.Hovered == nil {
		v.OnMouseEnter = func(core.MouseEvent) { hover(true) }
		v.OnMouseLeave = func(core.MouseEvent) { hover(false) }
	}
	return v
}
