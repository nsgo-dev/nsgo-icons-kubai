package kubai

import (
	"math"
	"slices"
	"sync"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestAllIcons parses every icon once, shares it, and draws its rest pose
// with balanced states and finite numbers.
func TestAllIcons(t *testing.T) {
	if len(allIcons) == 0 {
		t.Fatal("empty icon collection")
	}
	for _, icon := range allIcons {
		t.Run(icon.name, func(t *testing.T) {
			t.Parallel()
			results := make([]*Symbol, 4)
			var wg sync.WaitGroup
			for i := range results {
				wg.Go(func() { results[i] = icon.get() })
			}
			wg.Wait()
			for _, r := range results {
				if r != results[0] {
					t.Fatal("accessor did not reuse its symbol")
				}
			}
			s := results[0]
			if s.Name() != icon.name {
				t.Fatalf("name = %q", s.Name())
			}
			d := &core.Drawing{}
			r := &renderer{d: d, palette: palette{fill: Reference, edge: Reference}, scale: 1}
			r.symbol(s)
			depth := 0
			for _, op := range d.Ops {
				switch op.Kind {
				case core.DrawSave:
					depth++
				case core.DrawRestore:
					depth--
				}
				for _, v := range []float32{op.X, op.Y, op.X1, op.Y1, op.X2, op.Y2, op.W, op.H, op.R, op.A} {
					if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
						t.Fatalf("op %v holds %v", op.Kind, v)
					}
				}
			}
			if depth != 0 || len(d.Ops) == 0 {
				t.Fatalf("%d ops, states left open %d", len(d.Ops), depth)
			}
		})
	}
}

func TestBadges(t *testing.T) {
	for _, b := range allBadges {
		g := b()
		if g.tone == 0 || len(g.prog.ops) == 0 || g.gap.out[0] != 6.4 {
			t.Fatalf("badge %+v", g)
		}
	}
	if FolderPlus().Tone() != core.RGB(0x16A34A) || Folder().Tone() != 0 {
		t.Fatal("tones")
	}
}

func TestTracks(t *testing.T) {
	lin := parseTrack("=1 3")
	if lin.at(0) != 1 || lin.at(0.5) != 2 || lin.at(1) != 3 || lin.at(1.5) != 2 || lin.at(2) != 1 {
		t.Fatalf("linear track %+v", lin)
	}
	m := parseTrack("~0 0 48 5")
	if m.at(1) != 5 || m.at(2) != 0 || math.Abs(m.at(2-frameX(24))-m.at(frameX(24))) > 1e-9 {
		t.Fatalf("mirrored track %+v", m)
	}
	for i := 1; i < 2*(motionSamples+1); i++ {
		if i != motionSamples+1 && frameX(i) <= frameX(i-1) {
			t.Fatalf("frame %d at %v, before %v", i, frameX(i), frameX(i-1))
		}
	}
}

func TestTint(t *testing.T) {
	accent := core.RGB(0xEC4899)
	if got := tint(Reference, accent); got != accent {
		t.Fatalf("reference tints to %08x", got)
	}
	if got := tint(core.RGB(0xFFFFFF), accent); got != core.RGB(0xFFFFFF) {
		t.Fatalf("white tints to %08x", got)
	}
}

// TestHover binds the drawing's moving numbers only once the icon is first
// hovered, and plays the motion on the platform.
func TestHover(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(20)}, Children: []core.Node{
			core.View{TestID: "box", Children: []core.Node{Icon{Symbol: FolderPlus(), Size: 48, TestID: "icon"}}},
		}}
	})
	icon := app.Find("icon")
	if icon.Drawing == nil || len(icon.Drawing.Slots) != 0 {
		t.Fatal("an icon at rest binds numbers")
	}
	rest := slices.Clone(icon.Drawing.Ops)
	app.Hover("icon")
	icon = app.Find("icon")
	if len(icon.Drawing.Slots) == 0 {
		t.Fatal("a hovered icon binds no numbers")
	}
	if slices.Equal(icon.Ops, rest) {
		t.Fatal("the hover pose is the rest pose")
	}
	app.HoverOut()
	if got := app.Find("icon").Ops; !near(got, rest) {
		t.Fatal("the icon did not come back to rest")
	}
}

// TestClick plays the motion once on a press and returns to rest.
func TestClick(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(20)}, Children: []core.Node{
			Icon{Symbol: FolderPlus(), Size: 48, Click: true, TestID: "icon"},
		}}
	}, nsgotest.ManualFrames())
	icon := app.Find("icon")
	rest := slices.Clone(icon.Drawing.Ops)
	app.Press("icon")
	icon = app.Find("icon")
	if len(icon.Drawing.Slots) == 0 {
		t.Fatal("a clicked icon binds no numbers")
	}
	app.Advance(duration)
	icon = app.Find("icon")
	if slices.Equal(icon.Ops, rest) {
		t.Fatal("the click pose is the rest pose")
	}
	app.Advance(duration)
	if got := app.Find("icon").Ops; !near(got, rest) {
		t.Fatal("the icon did not come back to rest")
	}
}

// near reports whether two drawings hold the same ops, their numbers
// within rounding.
func near(a, b []core.DrawOp) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		for _, d := range []float32{x.X - y.X, x.Y - y.Y, x.X1 - y.X1, x.Y1 - y.Y1, x.X2 - y.X2, x.Y2 - y.Y2, x.W - y.W, x.R - y.R, x.A - y.A} {
			if math.Abs(float64(d)) > 1e-3 {
				return false
			}
		}
	}
	return true
}
