package main

import (
	"math"
	"strings"
	"testing"
)

func TestIdentifier(t *testing.T) {
	for name, want := range map[string]string{"folder-plus": "FolderPlus", "3d-box": "Icon3dBox", "icon": "IconIcon", "symbol": "IconSymbol"} {
		if got, err := identifier(name); err != nil || got != want {
			t.Errorf("%s: %q, %v; want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"", "-x", "a--b", "Foo", "../x"} {
		if _, err := identifier(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

// TestPathShape keeps a path's commands whatever its numbers: H and V are
// lines, relative points absolute and every arc four curves.
func TestPathShape(t *testing.T) {
	segs, err := parsePath("M1 2h3v4l-1 1A2 2 0 0 1 5 9a1 1 0 011 1Z")
	if err != nil {
		t.Fatal(err)
	}
	var cmds strings.Builder
	for _, s := range segs {
		cmds.WriteByte(s.cmd)
	}
	if got := cmds.String(); got != "MLLLCCCCCCCCZ" {
		t.Fatalf("commands %s", got)
	}
	if p := segs[3].pts; p[0] != 3 || p[1] != 7 {
		t.Fatalf("relative line ends at %v", p)
	}
	if end := segs[7].pts; math.Abs(end[4]-5) > 1e-9 || math.Abs(end[5]-9) > 1e-9 {
		t.Fatalf("arc ends at %v", end[4:])
	}
}

// TestTracks writes a straight track as its ends, a retraced one as its way
// in and anything else in full.
func TestTracks(t *testing.T) {
	ts := samples(motionSamples)
	xs := frameXs(ts)
	n := len(ts)
	lin, wave := make([]float64, 2*n), make([]float64, 2*n)
	for i, x := range xs {
		lin[i] = 2 + 3*min(x, 2-x)
		wave[i] = 0
		if i < n {
			wave[i] = math.Sin(math.Pi * ts[i])
		}
	}
	if got := encodeTrack(xs, lin, n, 0.008, 3); got != "=2 5" {
		t.Fatalf("straight track %q", got)
	}
	if got := encodeTrack(xs, wave, n, 0.008, 3); strings.HasPrefix(got, "=") || strings.HasPrefix(got, "~") {
		t.Fatalf("a swing that settles is written as %q", got)
	}
	tr := &trackSet{index: map[string]int{}}
	moved := make([]float64, len(lin))
	for i, v := range lin {
		moved[i] = v + 10
	}
	if a, b := tr.arg(xs, lin, n, 3), tr.arg(xs, moved, n, 3); a != "@0+2" || b != "@0+12" {
		t.Fatalf("tracks a constant apart: %q and %q", a, b)
	}
}

// TestLink compiles frames whose numbers move into tracks, and keeps those
// whose letters differ as variants.
func TestLink(t *testing.T) {
	ts := []float64{0, 1}
	frame := func(x string) string {
		return `<defs></defs><g transform="translate(` + x + ` 0)"><path d="M0 0H4V4Z" fill="#fff"/></g>`
	}
	p, _, err := link([]string{frame("0"), frame("2"), frame("2"), frame("0")}, ts)
	if err != nil {
		t.Fatal(err)
	}
	if p.tracks != 1 || p.variants != 1 || !strings.Contains(p.text, "X 1 0 0 1 @0 0") {
		t.Fatalf("program %q", p.text)
	}
	p, _, err = link([]string{`<path d="M0 0H4Z" fill="#fff"/>`, `<path d="M0 0H4ZM5 5H6Z" fill="#fff"/>`, `<path d="M0 0H4Z" fill="#fff"/>`, `<path d="M0 0H4Z" fill="#fff"/>`}, ts)
	if err != nil {
		t.Fatal(err)
	}
	if p.variants != 2 {
		t.Fatalf("%d variants in %q", p.variants, p.text)
	}
}
