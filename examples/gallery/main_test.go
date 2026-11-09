package main

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/nsgotest"

	kubai "github.com/oneo-me/nsgo-icons-kubai"
)

// TestMain gives the platform the main thread under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// TestBrowse lays out the grid within the window, mounting only the
// icons near the visible ones, and plays one on hover and back.
func TestBrowse(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(1100, 760))
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
	app := nsgotest.Start(t, Root, nsgotest.Size(1100, 760))
	app.Type("search", "folder plus")
	app.WaitFor("text:3 icons", 2*time.Second)
	app.Press("icon-folder-plus")
	if got := app.Find("preview-name").Text; got != "folder-plus" {
		t.Fatalf("preview shows %q, want folder-plus", got)
	}
	want := "kubai.Icon{\n\tSymbol: kubai.FolderPlus(),\n\tSize:   40,\n\tLabel:  \"Folder plus\",\n}"
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
	app := nsgotest.Start(t, Root, nsgotest.Size(1100, 760))
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
	got := usage(byName["folder-plus"], 32, core.RGB(0xEC4899), kubai.VariantSoft, true)
	want := "kubai.Icon{\n\tSymbol:     kubai.FolderPlus(),\n\tSize:       32,\n\tColor:      core.RGB(0xEC4899),\n\tVariant:    kubai.VariantSoft,\n\tBadgeColor: kubai.FolderPlus().Tone(),\n\tLabel:      \"Folder plus\",\n}"
	if got != want {
		t.Fatalf("usage is\n%s\nwant\n%s", got, want)
	}
	if got := usage(byName["folder"], 24, kubai.Reference, kubai.VariantColor, true); got != "kubai.Icon{\n\tSymbol: kubai.Folder(),\n\tLabel:  \"Folder\",\n}" {
		t.Fatalf("default usage is\n%s", got)
	}
}
