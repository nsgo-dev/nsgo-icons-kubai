// Command gallery finds Kubai icons: a search over every icon, a grid of
// the ones that match, and beside it the chosen icon, the code that draws
// it and the size, accent and variant both draw in.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	desklocales "github.com/nsgo-dev/nsgo/desktop/locales"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/oneo-me/nsgo-icons-kubai/examples/gallery/locales"

	kubai "github.com/oneo-me/nsgo-icons-kubai"
)

// entry is one icon: its upstream name, its accessor's name and the
// accessor.
type entry struct {
	name, ident string
	get         func() *kubai.Symbol
}

// state is what the gallery's parts share.
type state struct {
	query    *core.Signal[string]
	size     *core.Signal[float64]
	accent   *core.Signal[core.Color]
	variant  *core.Signal[kubai.Variant]
	tone     *core.Signal[bool]
	click    *core.Signal[bool]
	language *core.Signal[string]
	chosen   *ui.Selection
	matching func() []entry
	stats    *stats
}

// byName finds an icon by its upstream name.
var byName = func() map[string]entry {
	m := make(map[string]entry, len(allIcons))
	for _, e := range allIcons {
		m[e.name] = e
	}
	return m
}()

// selected is the chosen icon, the first when none is.
func (s *state) selected() entry {
	if keys := s.chosen.Keys(); len(keys) > 0 {
		if e, ok := byName[keys[0].(string)]; ok {
			return e
		}
	}
	return allIcons[0]
}

// icon is e drawn as the settings say.
func (s *state) icon(e entry, size core.Points, testID string, click bool) core.Node {
	sym := e.get()
	badge := core.Color(0)
	if s.tone.Get() {
		badge = sym.Tone()
	}
	return kubai.Icon{Symbol: sym, Size: size, Color: s.accent.Get(), Variant: s.variant.Get(), BadgeColor: badge,
		Click: click, Label: label(e.name), TestID: testID}
}

// Root is the gallery's one screen: the search and the grid, and the panel
// beside them.
func Root(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	s := &state{
		query:    core.NewSignal(cx, ""),
		size:     core.NewSignal(cx, 40.0),
		accent:   core.NewSignal(cx, kubai.Reference),
		variant:  core.NewSignal(cx, kubai.VariantColor),
		tone:     core.NewSignal(cx, true),
		click:    core.NewSignal(cx, false),
		language: core.NewSignal(cx, ""),
		chosen:   ui.NewSelection(cx, false),
	}
	if len(s.chosen.Keys()) == 0 {
		s.chosen.Set("folder")
	}
	rememberLanguage(cx, s)
	_ = locales.For(cx)
	s.matching = core.NewMemoFunc(cx, func() []entry { return search(s.query.Get()) }, nil).Get
	s.stats = reportUsage(cx, s)
	desktop.SetTitle(cx, locales.AppName(cx))
	cx.Effect(func() { desktop.SetApplicationMenu(cx, menu(cx)) })
	return core.View{Style: ui.FillRest.Merge(core.Style{Direction: core.Row}),
		StyleFunc: func() core.Style { return core.Style{Background: colors.Background()} },
		Children:  []core.Node{browser(cx, s), panel(cx, s)}}
}

// menu is the application menu: the app menu and Edit of roles, and
// Window.
func menu(cx *core.Scope) []desktop.MenuItem {
	tr := desklocales.For(cx)
	return []desktop.MenuItem{
		{Label: locales.AppName(cx), Items: []desktop.MenuItem{
			{Role: desktop.MenuAbout}, {Separator: true},
			{Role: desktop.MenuHide}, {Role: desktop.MenuHideOthers}, {Role: desktop.MenuShowAll}, {Separator: true},
			{Role: desktop.MenuQuit},
		}},
		{Label: tr.Edit(), Items: []desktop.MenuItem{
			{Role: desktop.MenuUndo}, {Role: desktop.MenuRedo}, {Separator: true},
			{Role: desktop.MenuCut}, {Role: desktop.MenuCopy}, {Role: desktop.MenuPaste}, {Role: desktop.MenuSelectAll},
		}},
		{Label: tr.Window(), Items: []desktop.MenuItem{
			{Role: desktop.MenuMinimize}, {Role: desktop.MenuZoom}, {Role: desktop.MenuToggleFullScreen}, {Separator: true},
			{Role: desktop.MenuClose},
		}},
	}
}

func main() { nsgo.Run(Root, nsgo.WindowSize(1100, 760)) }
