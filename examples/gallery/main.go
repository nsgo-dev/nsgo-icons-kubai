// Command gallery finds Kubai icons: a search over every icon, a grid of
// the ones that match, and beside it the chosen icon, the code that draws
// it and the size, accent and variant both draw in.
package main

import (
	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/desktop/locales"
	"github.com/oneo-me/nsgo/ui"

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
func (s *state) icon(e entry, size core.Points, testID string) core.Node {
	sym := e.get()
	badge := core.Color(0)
	if s.tone.Get() {
		badge = sym.Tone()
	}
	return kubai.Icon{Symbol: sym, Size: size, Color: s.accent.Get(), Variant: s.variant.Get(), BadgeColor: badge,
		Label: label(e.name), TestID: testID}
}

// Root is the gallery's one screen: the search and the grid, and the panel
// beside them.
func Root(cx *core.Scope) core.Node {
	cx.SetTheme(ui.DefaultTheme())
	colors := ui.ColorsOf(cx)
	s := &state{
		query:   core.NewSignal(cx, ""),
		size:    core.NewSignal(cx, 40.0),
		accent:  core.NewSignal(cx, kubai.Reference),
		variant: core.NewSignal(cx, kubai.VariantColor),
		tone:    core.NewSignal(cx, false),
		chosen:  ui.NewSelection(cx, false),
	}
	s.chosen.Set("folder")
	s.matching = core.NewMemoFunc(cx, func() []entry { return search(s.query.Get()) }, nil).Get
	s.stats = reportUsage(cx, s)
	desktop.SetTitle(cx, "Kubai Icons")
	cx.Effect(func() { desktop.SetApplicationMenu(cx, menu(locales.For(cx))) })
	return core.View{Style: ui.FillRest.Merge(core.Style{Direction: core.Row}),
		StyleFunc: func() core.Style { return core.Style{Background: colors.Background()} },
		Children:  []core.Node{browser(cx, s), panel(cx, s)}}
}

// menu is the application menu: the app menu and Edit of roles, and
// Window.
func menu(tr locales.Messages) []desktop.MenuItem {
	return []desktop.MenuItem{
		{Label: "Kubai Icons", Items: []desktop.MenuItem{
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
