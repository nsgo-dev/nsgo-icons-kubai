package main

import (
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/oneo-me/nsgo-icons-kubai/examples/gallery/locales"
)

// search returns the icons whose names hold every word of query; spaces
// and hyphens both separate words. The name query spells comes first, then
// names that start with it, then the rest, each in upstream's order.
func search(query string) []entry {
	words := strings.Fields(strings.ToLower(strings.ReplaceAll(query, "-", " ")))
	if len(words) == 0 {
		return allIcons
	}
	spelled := strings.Join(words, "-")
	rank := func(e entry) int {
		switch {
		case e.name == spelled:
			return 0
		case strings.HasPrefix(e.name, spelled):
			return 1
		}
		return 2
	}
	var found []entry
	for _, e := range allIcons {
		match := true
		for _, w := range words {
			if !strings.Contains(e.name, w) && !strings.Contains(strings.ToLower(e.ident), w) {
				match = false
				break
			}
		}
		if match {
			found = append(found, e)
		}
	}
	slices.SortStableFunc(found, func(a, b entry) int { return rank(a) - rank(b) })
	return found
}

// at is the matching icon at i, which a list asks for as the matches
// change under it; ok is false past their end.
func (s *state) at(i int) (e entry, ok bool) {
	list := s.matching()
	if i < 0 || i >= len(list) {
		return entry{}, false
	}
	return list[i], true
}

// gutter is the grid's padding at its sides and end, inside the scroll
// view so the scroll bar keeps to the window's edge.
const gutter = 12

// cellWidth is a grid cell's width for icons of size: room for the icon
// and a name under it.
func cellWidth(size float64) core.Points { return core.Points(max(size+40, 104)) }

// browser is the search field over the grid of the icons that match.
func browser(cx *core.Scope, s *state) core.Node {
	colors := ui.ColorsOf(cx)
	width := core.NewSignal(cx, core.Points(0))
	columns := core.NewMemo(cx, func() int {
		return max(1, int((width.Get()+8)/(cellWidth(s.size.Get())+8)))
	})
	field := ui.Input{Value: s.query.Get, OnChange: s.query.Set, TestID: "search",
		PlaceholderFunc: func() string { return locales.SearchPlaceholder(cx, len(allIcons)) }, Style: core.Style{FlexGrow: 1}}
	count := core.Text{ContentFunc: func() string { return locales.IconCount(cx, len(s.matching())) },
		Style: ui.TypeCaption, StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }}
	grid := core.Dynamic(func(*core.Scope) core.Node {
		n, size := columns.Get(), s.size.Get()
		return ui.List{Count: func() int { return len(s.matching()) }, Columns: n, Gap: 8, TapSelects: true,
			Key:       func(i int) any { e, _ := s.at(i); return e.name },
			RowLabel:  func(i int) string { e, _ := s.at(i); return label(e.name) },
			Selection: s.chosen, EstimatedSize: core.Points(size + 48), Label: "Icons", TestID: "icons",
			Empty: core.Text{Content: locales.NoIconsMatch(cx), Style: ui.TypeBody.Merge(core.Style{AlignSelf: core.AlignCenter, Margin: core.All(32)}),
				StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }},
			Row: func(cx *core.Scope, row core.ListRow) core.Node { return cell(cx, s, row) },
			RowStyle: core.Style{Direction: core.Column, JustifyContent: core.JustifyCenter, Gap: 8,
				Width: core.Pt(cellWidth(size)), Height: core.Pt(core.Points(size + 48)), Padding: core.All(8)},
			Style: ui.FillRest, ListStyle: core.Style{Padding: core.Edges{Left: gutter, Right: gutter, Bottom: gutter}}}
	})
	return core.View{Style: ui.FillRest, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(12, gutter)},
			Children: []core.Node{field, count}},
		core.View{Style: ui.FillRest,
			OnLayout: func(b core.Bounds) { width.Set(b.Size.Width - 2*gutter) }, Children: []core.Node{grid}},
	}}
}

// cell is a grid cell: the icon over its name.
func cell(cx *core.Scope, s *state, row core.ListRow) core.Node {
	colors := ui.ColorsOf(cx)
	return core.Dynamic(func(*core.Scope) core.Node {
		e, ok := s.at(row.Index())
		if !ok {
			return core.View{}
		}
		return core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8, Width: core.Pct(100)}, Children: []core.Node{
			s.icon(e, core.Points(s.size.Get()), "icon-"+e.name, false),
			core.Text{Content: e.name, Style: ui.TypeCaption.Merge(core.Style{NumberOfLines: 1, MaxWidth: core.Pct(100)}),
				StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }},
		}}
	})
}
