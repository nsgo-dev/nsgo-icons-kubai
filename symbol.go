package kubai

import (
	"sync"

	"github.com/nsgo-dev/nsgo/core"
)

// Symbol is one Kubai icon: its layers and how they move on hover. Icon
// draws it; accessors such as Folder return it.
type Symbol struct {
	name string
	prog *program
	// A badged icon is its base with the badge over the corner at at.
	base  *Symbol
	badge *badge
	at    [2]float32
}

// Name is the icon's upstream name, such as "folder-plus".
func (s *Symbol) Name() string { return s.name }

// Tone is the badge's semantic color, green for plus and red for x, which
// Icon.BadgeColor takes for a badge that says what it means by color. It
// is 0 for an icon without a badge.
func (s *Symbol) Tone() core.Color {
	if s.badge == nil {
		return 0
	}
	return s.badge.tone
}

// badge is a badge alone at the origin, and the track of the gap cut round
// it out of its base.
type badge struct {
	tone core.Color
	prog *program
	gap  track
}

// Zero-value caches keep unused icons out of package initializers, so a
// binary carries only the icons it uses.
type lazySymbol struct {
	once sync.Once
	s    *Symbol
}

func (l *lazySymbol) get(name, src string) *Symbol {
	l.once.Do(func() { l.s = &Symbol{name: name, prog: parseProgram(src)} })
	return l.s
}

func (l *lazySymbol) badged(name string, base *Symbol, b *badge, x, y float32) *Symbol {
	l.once.Do(func() { l.s = &Symbol{name: name, base: base, badge: b, at: [2]float32{x, y}} })
	return l.s
}

type lazyBadge struct {
	once sync.Once
	b    *badge
}

func (l *lazyBadge) get(tone uint32, gap, src string) *badge {
	l.once.Do(func() { l.b = &badge{tone: core.RGB(tone), prog: parseProgram(src), gap: parseTrack(gap)} })
	return l.b
}
