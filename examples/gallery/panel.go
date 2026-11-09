package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"

	kubai "github.com/oneo-me/nsgo-icons-kubai"
)

// accents are the accents the panel offers: Kubai's blue first.
var accents = []ui.Option[core.Color]{
	{Value: kubai.Reference, Label: "Blue"}, {Value: core.RGB(0x6366F1), Label: "Indigo"},
	{Value: core.RGB(0xEC4899), Label: "Pink"}, {Value: core.RGB(0x10B981), Label: "Green"},
	{Value: core.RGB(0xF59E0B), Label: "Amber"},
}

var variants = []ui.Option[kubai.Variant]{{Value: kubai.VariantColor, Label: "Color"}, {Value: kubai.VariantSoft, Label: "Soft"}}

// panelWidth is the panel's width beside the grid.
const panelWidth = 320

// statsSetting turns the usage statistics on or off.
func statsSetting(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	on := core.NewSignal(cx, statsOn(cx))
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
			core.Text{Content: "Send usage statistics", Style: ui.TypeBody},
			core.Text{Content: "Which icons are looked at and copied, with no personal data", Style: ui.TypeCaption,
				StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }},
		}},
		ui.Switch{Checked: on.Get, OnChange: func(v bool) { on.Set(v); setStatsOn(cx, v) }, Label: "Send usage statistics"},
	}}
}

// panel is the chosen icon, the code that draws it, and the settings.
func panel(cx *core.Scope, s *state) core.Node {
	colors := ui.ColorsOf(cx)
	muted := func() core.Style { return core.Style{Color: colors.Muted()} }
	heading := func(text string) core.Node {
		return core.Text{Content: text, Style: ui.TypeCaption.Merge(core.Style{FontWeight: ui.Semibold}), StyleFunc: muted}
	}
	code := func() string {
		return usage(s.selected(), s.size.Get(), s.accent.Get(), s.variant.Get(), s.tone.Get(), s.click.Get())
	}
	copied := core.NewSignal(cx, "")
	preview := core.Dynamic(func(*core.Scope) core.Node {
		e := s.selected()
		return core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
			core.View{Style: core.Style{AlignSelf: core.AlignStretch, Height: core.Pt(140), AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter, CornerRadius: ui.RadiusControl},
				StyleFunc: func() core.Style { return core.Style{Background: colors.Inset()} },
				Children:  []core.Node{s.icon(e, 72, "preview", s.click.Get())}},
			core.Text{Content: e.name, Style: ui.TypeTitle.Merge(core.Style{Margin: core.Edges{Top: 6}}), TestID: "preview-name"},
			core.Text{Content: fmt.Sprintf("kubai.%s()", e.ident), Style: ui.TypeCode, StyleFunc: muted},
		}}
	})
	knob := func(name string, control core.Node) core.Node {
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(ui.ButtonHeight)},
			Children: []core.Node{core.Text{Content: name, Style: ui.TypeBody.Merge(core.Style{Width: core.Pt(56)}), StyleFunc: muted}, control}}
	}
	return core.View{Style: core.Style{Width: core.Pt(panelWidth), Borders: core.Edges{Left: 1}},
		StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Line(), Background: colors.Surface()} },
		Children: []core.Node{ui.ScrollView{Style: ui.FillRest, TestID: "panel", Children: []core.Node{
			core.View{Style: core.Style{Padding: core.All(16), Gap: 20}, Children: []core.Node{
				preview,
				core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween},
						Children: []core.Node{
							heading("Usage"),
							core.Dynamic(func(*core.Scope) core.Node {
								text := "Copy"
								if copied.Get() == code() {
									text = "Copied"
								}
								return ui.Button{Text: text, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, TestID: "copy",
									OnPress: func() { c := code(); cx.SetClipboard(c); copied.Set(c); s.stats.copied(s.selected().name) }}
							}),
						}},
					core.View{Style: core.Style{Padding: core.All(12), CornerRadius: ui.RadiusControl}, Selectable: true,
						StyleFunc: func() core.Style { return core.Style{Background: colors.Inset()} },
						Children:  []core.Node{core.Text{ContentFunc: code, Style: ui.TypeCode, TestID: "usage"}}},
				}},
				core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
					heading("Settings"),
					knob("Size", core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexGrow: 1}, Children: []core.Node{
						core.Text{ContentFunc: func() string { return fmt.Sprint(s.size.Get()) }, Style: ui.TypeBody.Merge(core.Style{Width: core.Pt(24)})},
						ui.Slider{Value: s.size.Get, OnChange: s.size.Set, Min: 16, Max: 96, Step: 4, Label: "Size", TestID: "size",
							Style: core.Style{FlexGrow: 1}},
					}}),
					knob("Color", ui.Select[core.Color]{Options: accents, Value: s.accent.Get, OnChange: s.accent.Set, Label: "Color", TestID: "color",
						Style: core.Style{Width: core.Pt(panelWidth - 100)}}),
					knob("Variant", ui.Select[kubai.Variant]{Options: variants, Value: s.variant.Get, OnChange: s.variant.Set, Label: "Variant", TestID: "variant",
						Style: core.Style{Width: core.Pt(panelWidth - 100)}}),
					knob("Tone", ui.Switch{Checked: s.tone.Get, OnChange: s.tone.Set, Label: "Tone"}),
					knob("Click", ui.Switch{Checked: s.click.Get, OnChange: s.click.Set, Label: "Click"}),
				}},
				statsSetting(cx),
			}},
		}}},
	}
}
