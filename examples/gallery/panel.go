package main

import (
	"fmt"

	"github.com/nsgo-dev/nsgo-icons-kubai/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	kubai "github.com/nsgo-dev/nsgo-icons-kubai"
)

// accents are the accents the panel offers as one-click wells: Kubai's
// blue first.
var accents = []struct {
	color core.Color
	id    string
	label func(*core.Scope) string
}{
	{kubai.Reference, "blue", locales.ColorBlue},
	{core.RGB(0x6366F1), "indigo", locales.ColorIndigo},
	{core.RGB(0xEC4899), "pink", locales.ColorPink},
	{core.RGB(0x10B981), "green", locales.ColorGreen},
	{core.RGB(0xF59E0B), "amber", locales.ColorAmber},
}

// rgb24 is c without alpha, for comparing accents and writing usage.
func rgb24(c core.Color) uint32 { return uint32(c) >> 8 }

// panelWidth is the panel's width beside the grid.
const panelWidth = 320

// wellSize is a color well's diameter.
const wellSize core.Points = 22

// tabs is a small equal tab list for a settings knob.
func tabs[T comparable](label, testID string, value func() T, change func(T), options []ui.Option[T]) core.Node {
	return ui.Tabs[T]{Size: ui.SizeSmall, Equal: true, Label: label, TestID: testID, Value: value, OnChange: change,
		Style: core.Style{FlexGrow: 1}, Options: options}
}

// onOff is Off and On tabs for a boolean setting.
func onOff(cx *core.Scope, get func() bool, set func(bool), label, testID string) core.Node {
	return tabs(label, testID, get, set, []ui.Option[bool]{
		{Value: false, Label: locales.Off(cx), TestID: testID + "-off"},
		{Value: true, Label: locales.On(cx), TestID: testID + "-on"},
	})
}

// knob is a setting's name beside its control.
func knob(name string, muted func() core.Style, control core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(ui.ButtonHeight)},
		Children: []core.Node{core.Text{Content: name, Style: ui.TypeBody.Merge(core.Style{Width: core.Pt(72)}), StyleFunc: muted}, control}}
}

// colorWell is a one-click accent: a disc that rings while chosen.
func colorWell(cx *core.Scope, color core.Color, on func() bool, label, testID string, press func()) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{
		Style: core.Style{Width: core.Pt(wellSize), Height: core.Pt(wellSize), CornerRadius: wellSize / 2, BorderWidth: 1},
		StyleFunc: func() core.Style {
			s := core.Style{Background: color, BorderColor: colors.Border()}
			if on() {
				s.OutlineWidth, s.OutlineColor, s.OutlineOffset = 2, colors.Text(), 2
			}
			return s
		},
		Accessible: true, Role: core.RoleButton, Label: label, TestID: testID,
		OnPress: func(core.PressEvent) { press() },
	}
}

// colorPickerWell is a disc of the current accent that opens a color
// picker, and rings when the accent is not one of the presets.
func colorPickerWell(cx *core.Scope, s *state) core.Node {
	colors := ui.ColorsOf(cx)
	preset := func() bool {
		cur := rgb24(s.accent.Get())
		for _, a := range accents {
			if cur == rgb24(a.color) {
				return true
			}
		}
		return false
	}
	return base.Popover{Offset: 4, Margin: 8, Label: locales.ColorCustom(cx), TestID: "color-custom",
		Style: core.Style{Width: core.Pt(wellSize), Height: core.Pt(wellSize), CornerRadius: wellSize / 2, BorderWidth: 1},
		StyleFunc: func() core.Style {
			look := core.Style{Background: s.accent.Get(), BorderColor: colors.Border()}
			if !preset() {
				look.OutlineWidth, look.OutlineColor, look.OutlineOffset = 2, colors.Text(), 2
			}
			return look
		},
		PopupStyle: core.Style{Padding: core.All(12), MaxWidth: core.Pct(100)},
		PopupStyleFunc: func() core.Style {
			return ui.FinOf(cx).Menu().Over(core.Style{CornerRadius: ui.RadiusCard, BorderWidth: 1})
		},
		Children: []core.Node{ui.ColorPicker{Value: s.accent.Get, NoAlpha: true, TestID: "color-picker",
			OnChange: func(c core.Color, _ string) { s.accent.Set(core.RGB(rgb24(c))) }}}}
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
	stats := core.NewSignal(cx, statsOn(cx))
	return core.View{Style: core.Style{Width: core.Pt(panelWidth), Height: core.Pct(100), FlexShrink: 0, Borders: core.Edges{Left: 1}},
		StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Line(), Background: colors.Surface()} },
		Children: []core.Node{
			ui.ScrollView{Style: ui.FillRest, TestID: "panel", Children: []core.Node{
				core.Dynamic(func(cx *core.Scope) core.Node {
					_ = locales.For(cx)
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
					wells := make([]core.Node, 0, len(accents)+1)
					for _, a := range accents {
						a := a
						wells = append(wells, colorWell(cx, a.color, func() bool { return rgb24(s.accent.Get()) == rgb24(a.color) }, a.label(cx), "color-"+a.id, func() {
							s.accent.Set(a.color)
						}))
					}
					wells = append(wells, colorPickerWell(cx, s))
					return core.View{Style: core.Style{Padding: core.All(16), Gap: 20}, Children: []core.Node{
						preview,
						core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
							core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween},
								Children: []core.Node{
									heading(locales.Usage(cx)),
									core.Dynamic(func(*core.Scope) core.Node {
										text := locales.Copy(cx)
										if copied.Get() == code() {
											text = locales.Copied(cx)
										}
										return ui.Button{Text: text, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, TestID: "copy",
											OnPress: func() { c := code(); cx.SetClipboard(c); copied.Set(c) }}
									}),
								}},
							core.View{Style: core.Style{Padding: core.All(12), CornerRadius: ui.RadiusControl}, Selectable: true,
								StyleFunc: func() core.Style { return core.Style{Background: colors.Inset()} },
								Children:  []core.Node{core.Text{ContentFunc: code, Style: ui.TypeCode, TestID: "usage"}}},
						}},
						core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
							heading(locales.Settings(cx)),
							knob(locales.Size(cx), muted, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexGrow: 1}, Children: []core.Node{
								core.Text{ContentFunc: func() string { return fmt.Sprint(s.size.Get()) }, Style: ui.TypeBody.Merge(core.Style{Width: core.Pt(24)})},
								ui.Slider{Value: s.size.Get, OnChange: s.size.Set, Min: 16, Max: 96, Step: 4, Label: locales.Size(cx), TestID: "size",
									Style: core.Style{FlexGrow: 1}},
							}}),
							knob(locales.Color(cx), muted, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter,
								Gap: 8, FlexGrow: 1, Wrap: core.Wrap, Padding: core.All(2)}, Children: wells}),
							knob(locales.Variant(cx), muted, tabs(locales.Variant(cx), "variant", s.variant.Get, s.variant.Set, []ui.Option[kubai.Variant]{
								{Value: kubai.VariantColor, Label: locales.VariantColor(cx), TestID: "variant-color"},
								{Value: kubai.VariantSoft, Label: locales.VariantSoft(cx), TestID: "variant-soft"},
							})),
							knob(locales.Tone(cx), muted, onOff(cx, s.tone.Get, s.tone.Set, locales.Tone(cx), "tone")),
							knob(locales.Click(cx), muted, onOff(cx, s.click.Get, s.click.Set, locales.Click(cx), "click")),
						}},
					}}
				}),
			}},
			core.Dynamic(func(cx *core.Scope) core.Node {
				_ = locales.For(cx)
				langs := make([]ui.Option[string], 0, 3)
				for _, l := range languageChoices(cx) {
					langs = append(langs, ui.Option[string]{Value: l.id, Label: l.name, TestID: l.testID})
				}
				return core.View{Style: core.Style{Padding: core.All(16), Gap: 12, Borders: core.Edges{Top: 1}},
					StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Line()} },
					Children: []core.Node{
						knob(locales.Language(cx), muted, tabs(locales.Language(cx), "language", s.language.Get, func(v string) { s.setLanguage(cx, v) }, langs)),
						knob(locales.StatsSend(cx), muted, onOff(cx, stats.Get, func(v bool) { stats.Set(v); setStatsOn(cx, v) }, locales.StatsSend(cx), "stats")),
					}}
			}),
		}}
}
