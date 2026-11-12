package main

import (
	"fmt"
	"go/format"
	"strings"

	"github.com/nsgo-dev/nsgo/core"

	kubai "github.com/oneo-me/nsgo-icons-kubai"
)

// label is a name as a person reads it: "folder-plus" is "Folder plus".
func label(name string) string {
	words := strings.ReplaceAll(name, "-", " ")
	if words == "" {
		return ""
	}
	return strings.ToUpper(words[:1]) + words[1:]
}

// usage is the Go that draws e as the settings say, leaving out what is
// already the default.
func usage(e entry, size float64, accent core.Color, variant kubai.Variant, tone, click bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "kubai.Icon{\nSymbol: kubai.%s(),\n", e.ident)
	if size != 24 {
		fmt.Fprintf(&b, "Size: %g,\n", size)
	}
	if accent != kubai.Reference {
		fmt.Fprintf(&b, "Color: core.RGB(0x%06X),\n", rgb24(accent))
	}
	if variant == kubai.VariantSoft {
		b.WriteString("Variant: kubai.VariantSoft,\n")
	}
	if tone && e.get().Tone() != 0 {
		fmt.Fprintf(&b, "BadgeColor: kubai.%s().Tone(),\n", e.ident)
	}
	if click {
		b.WriteString("Click: true,\n")
	}
	fmt.Fprintf(&b, "Label: %q,\n}", label(e.name))
	// gofmt aligns the fields as an editor would.
	src, err := format.Source([]byte("package p\n\nvar _ = " + b.String() + "\n"))
	if err != nil {
		return b.String()
	}
	return strings.TrimSuffix(strings.TrimPrefix(string(src), "package p\n\nvar _ = "), "\n")
}
