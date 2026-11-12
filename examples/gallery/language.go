package main

import (
	"github.com/nsgo-dev/nsgo-icons-kubai/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/core"
)

// languageKey keeps the language chosen in the settings; without it the
// gallery follows the system's.
const languageKey = "language"

// rememberLanguage shows the language chosen last time.
func rememberLanguage(cx *core.Scope, s *state) {
	if lang, ok := cx.Storage().GetItem(languageKey); ok && lang != "" {
		s.language.Set(lang)
		cx.SetLocale(lang)
	}
}

// setLanguage shows lang, "" the system's, and keeps the choice.
func (s *state) setLanguage(cx *core.Scope, lang string) {
	s.language.Set(lang)
	cx.SetLocale(lang)
	if lang == "" {
		cx.Storage().RemoveItem(languageKey)
	} else {
		cx.Storage().SetItem(languageKey, lang)
	}
}

// languageChoice is one language the settings offer: id empty for the
// system, name in the language itself.
type languageChoice struct{ id, name, testID string }

func languageChoices(cx *core.Scope) []languageChoice {
	return []languageChoice{
		{"", locales.LanguageSystem(cx), "language-system"},
		{"en", "English", "language-en"},
		{"zh-Hans", "中文", "language-zh-Hans"},
	}
}
