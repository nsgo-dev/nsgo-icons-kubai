package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"

	kubai "github.com/nsgo-dev/nsgo-icons-kubai"
)

// TestMain gives the platform the main thread under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func start(t testing.TB, opts ...nsgotest.Option) *nsgotest.App {
	return nsgotest.Start(t, Root, append([]nsgotest.Option{nsgotest.Size(1100, 760),
		nsgotest.Environment(core.Environment{Locale: "en"})}, opts...)...)
}

// TestBrowse lays out the grid within the window, mounting only the
// icons near the visible ones, and plays one on hover and back.
func TestBrowse(t *testing.T) {
	app := start(t)
	grid := app.Find("icons").Frame
	if grid.Size.Height > 760 {
		t.Fatalf("grid is %v tall, past the window", grid.Size.Height)
	}
	if grid.Origin.X != 0 || grid.Origin.X+grid.Size.Width != 1100-panelWidth {
		t.Fatalf("grid spans %v, not edge to edge beside the panel", grid)
	}
	mounted := 0
	for _, e := range allIcons {
		if app.Exists("icon-" + e.name) {
			mounted++
		}
	}
	if mounted >= len(allIcons)/4 {
		t.Fatalf("%d of %d icons mounted", mounted, len(allIcons))
	}
	if got := app.Find("preview-name").Text; got != "folder" {
		t.Fatalf("preview shows %q, want folder", got)
	}
	app.Screenshot("gallery")
	app.Hover("icon-abacus")
	app.Screenshot("gallery-hover")
	app.HoverOut()
}

// TestSearchAndCopy narrows the grid, chooses an icon and copies its code.
func TestSearchAndCopy(t *testing.T) {
	app := start(t)
	app.Type("search", "folder plus")
	app.WaitFor("text:3 icons", 2*time.Second)
	app.Press("icon-folder-plus")
	if got := app.Find("preview-name").Text; got != "folder-plus" {
		t.Fatalf("preview shows %q, want folder-plus", got)
	}
	want := "kubai.Icon{\n\tSymbol:     kubai.FolderPlus(),\n\tSize:       40,\n\tBadgeColor: kubai.FolderPlus().Tone(),\n\tLabel:      \"Folder plus\",\n}"
	if got := app.Find("usage").Text; got != want {
		t.Fatalf("usage is\n%s\nwant\n%s", got, want)
	}
	app.Press("copy")
	app.Find("text:Copied")
	app.Screenshot("gallery-search")
	app.Clear("search")
	app.Type("search", "no such icon")
	app.Find("text:No icons match")
}

// TestApplicationMenu sets the app menu of roles, Edit, and Window.
func TestApplicationMenu(t *testing.T) {
	app := start(t)
	if len(app.Menu) == 0 {
		t.Fatal("no application menu")
	}
	if app.Menu[0].Items[0].Role != desktop.MenuAbout {
		t.Fatalf("app menu starts with %+v", app.Menu[0].Items[0])
	}
	var roles []desktop.MenuRole
	for _, it := range app.Menu[len(app.Menu)-1].Items {
		if !it.Separator {
			roles = append(roles, it.Role)
		}
	}
	want := []desktop.MenuRole{desktop.MenuMinimize, desktop.MenuZoom, desktop.MenuToggleFullScreen, desktop.MenuClose}
	if !slices.Equal(roles, want) {
		t.Fatalf("Window roles %v, want %v", roles, want)
	}
}

func TestSearch(t *testing.T) {
	for query, want := range map[string][]string{"folder-plus": {"folder-plus", "folder-open-plus", "folders-plus"},
		"FOLDER plus": {"folder-plus", "folder-open-plus", "folders-plus"}, "zzz": nil} {
		var got []string
		for _, e := range search(query) {
			got = append(got, e.name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("search(%q) found %v, want %v", query, got, want)
		}
	}
	if got := len(search(" ")); got != len(allIcons) {
		t.Errorf("an empty search found %d", got)
	}
}

func TestUsage(t *testing.T) {
	got := usage(byName["folder-plus"], 32, core.RGB(0xEC4899), kubai.VariantSoft, true, true)
	want := "kubai.Icon{\n\tSymbol:     kubai.FolderPlus(),\n\tSize:       32,\n\tColor:      core.RGB(0xEC4899),\n\tVariant:    kubai.VariantSoft,\n\tBadgeColor: kubai.FolderPlus().Tone(),\n\tClick:      true,\n\tLabel:      \"Folder plus\",\n}"
	if got != want {
		t.Fatalf("usage is\n%s\nwant\n%s", got, want)
	}
	if got := usage(byName["folder"], 24, kubai.Reference, kubai.VariantColor, true, false); got != "kubai.Icon{\n\tSymbol: kubai.Folder(),\n\tLabel:  \"Folder\",\n}" {
		t.Fatalf("default usage is\n%s", got)
	}
}

// TestColorVariantAndLanguage changes the accent and variant with one
// press, opens the color well's picker, and switches the UI to Chinese.
func TestColorVariantAndLanguage(t *testing.T) {
	app := start(t)
	app.Press("color-pink")
	want := "kubai.Icon{\n\tSymbol: kubai.Folder(),\n\tSize:   40,\n\tColor:  core.RGB(0xEC4899),\n\tLabel:  \"Folder\",\n}"
	if got := app.Find("usage").Text; got != want {
		t.Fatalf("pink usage is\n%s\nwant\n%s", got, want)
	}
	app.Press("variant-soft")
	if !strings.Contains(app.Find("usage").Text, "VariantSoft") {
		t.Fatal("soft variant missing from usage")
	}
	app.Press("language-zh-Hans")
	app.Find("text:用法")
	app.Press("language-en")
	app.Find("text:Usage")
	app.Press("color-custom")
	app.Find("color-picker")
}

func TestLanguagePersists(t *testing.T) {
	app := start(t, nsgotest.Storage(map[string]string{languageKey: "zh-Hans"}))
	app.Find("text:用法")
}
