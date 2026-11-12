// Command kubaigen imports a pinned, clean Kubai Icons checkout and writes
// the kubai package: one accessor per icon, its layers and motion compiled
// for the canvas, its provenance and its license.
//
// The icons are programs: each one's draw() moves its layers. kubaigen
// runs upstream's renderer in goja at moments of the motion in and out,
// reads each frame's markup into canvas operations and keeps, for every
// number that moves, a track the platform animates.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// motionSamples is how many steps of time each direction is rendered at.
const motionSamples = 48

func main() {
	upstream := flag.String("upstream", "", "path to a clean Kubai Icons checkout")
	commit := flag.String("commit", "", "required full upstream commit hash")
	out := flag.String("out", ".", "output package directory")
	gallery := flag.String("gallery", "examples/gallery", "the gallery's directory, which lists every icon")
	check := flag.Bool("check", false, "verify the generated files without writing")
	flag.Parse()
	if err := run(*upstream, *commit, *out, *gallery, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// compiledIcon is one icon ready to write: a program, or a base and the
// badge the runtime draws over its corner.
type compiledIcon struct {
	name, base, badge string
	at                [2]float64
	program           program
}

// compiledBadge is a badge alone at the origin, and the track of the gap
// cut round it.
type compiledBadge struct {
	name, tone string
	program    program
	gap        string
}

func compileAll(set iconSet) ([]compiledIcon, []compiledBadge, map[string][]string, error) {
	r, err := newRuntime(set.scripts)
	if err != nil {
		return nil, nil, nil, err
	}
	var names, badgeNames func() []string
	var meta func(string) map[string]any
	var tone func(string) string
	var badgeAt func(string) []float64
	var badgeScale func(string, []float64) []float64
	k := r.vm.Get("kubai").ToObject(r.vm)
	for name, fn := range map[string]any{"names": &names, "badgeNames": &badgeNames, "meta": &meta, "badgeTone": &tone, "badgeAt": &badgeAt, "badgeScale": &badgeScale} {
		if err := r.vm.ExportTo(k.Get(name), fn); err != nil {
			return nil, nil, nil, err
		}
	}
	ts := samples(motionSamples)
	var icons []compiledIcon
	var jobs []*program
	var jobNames []string
	for _, n := range names() {
		m := meta(n)
		ic := compiledIcon{name: n, base: m["base"].(string), badge: m["badge"].(string)}
		if ic.base != "" && ic.badge != "off" {
			a := badgeAt(ic.base)
			ic.at = [2]float64{a[0], a[1]}
		}
		icons = append(icons, ic)
	}
	for i := range icons {
		if icons[i].base == "" || icons[i].badge == "off" {
			jobs = append(jobs, &icons[i].program)
			jobNames = append(jobNames, icons[i].name)
		}
	}
	var badges []compiledBadge
	for _, n := range badgeNames() {
		badges = append(badges, compiledBadge{name: n, tone: tone(n)})
	}
	xs := frameXs(ts)
	for i := range badges {
		jobs = append(jobs, &badges[i].program)
		jobNames = append(jobNames, "#"+badges[i].name)
		s := badgeScale(badges[i].name, ts)
		for j := range s {
			s[j] *= 6.4
		}
		badges[i].gap = encodeTrack(xs, s, len(ts), 0.008, 3)
	}
	warnings := map[string][]string{}
	var mu sync.Mutex
	var firstErr error
	queue := make(chan int)
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Go(func() {
			rt, err := newRuntime(set.scripts)
			if err != nil {
				mu.Lock()
				firstErr = err
				mu.Unlock()
			}
			for i := range queue {
				if err != nil {
					continue
				}
				p, warn, lerr := link(rt.frames(jobNames[i], ts), ts)
				mu.Lock()
				if lerr != nil && firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", jobNames[i], lerr)
				}
				for w := range warn {
					warnings[w] = append(warnings[w], jobNames[i])
				}
				mu.Unlock()
				*jobs[i] = p
			}
		})
	}
	for i := range jobs {
		queue <- i
	}
	close(queue)
	wg.Wait()
	return icons, badges, warnings, firstErr
}

func git(dir string, args ...string) (string, error) {
	b, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git: %w: %s", err, b)
	}
	return strings.TrimSpace(string(b)), nil
}

type provenance struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	SHA256     string `json:"inputs_sha256"`
	Count      int    `json:"icon_count"`
}

// inputs hashes what the icons are made from: the license, the renderer
// and every icon and badge definition.
func inputs(upstream string) (string, error) {
	hash := sha256.New()
	var files []string
	for _, dir := range []string{"icons", "badges", "packages/core/src"} {
		paths, err := filepath.Glob(filepath.Join(upstream, dir, "*.js"))
		if err != nil {
			return "", err
		}
		files = append(files, paths...)
	}
	sort.Strings(files)
	files = append([]string{filepath.Join(upstream, "LICENSE-ICONS")}, files...)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(upstream, f)
		fmt.Fprintf(hash, "%s\x00%s\x00", filepath.ToSlash(rel), data)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func run(upstream, commit, out, gallery string, check bool) error {
	if upstream == "" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) {
		return fmt.Errorf("provide -upstream and a full 40-character -commit")
	}
	head, err := git(upstream, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != commit {
		return fmt.Errorf("checkout is %s, requested %s", head, commit)
	}
	status, err := git(upstream, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("upstream checkout must be clean")
	}
	license, err := os.ReadFile(filepath.Join(upstream, "LICENSE-ICONS"))
	if err != nil {
		return err
	}
	digest, err := inputs(upstream)
	if err != nil {
		return err
	}
	set, err := bundle(upstream)
	if err != nil {
		return err
	}
	icons, badges, warnings, err := compileAll(set)
	if err != nil {
		return err
	}
	for _, w := range sortedKeys(warnings) {
		fmt.Printf("Left out %s in %s\n", w, strings.Join(warnings[w], ", "))
	}
	code, tests, list, err := render(icons, badges)
	if err != nil {
		return err
	}
	meta, err := json.MarshalIndent(provenance{"https://github.com/kubai087/kubai-icons", commit, digest, len(icons)}, "", "  ")
	if err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
	}{
		{filepath.Join(out, "icons_gen.go"), code},
		{filepath.Join(out, "icons_gen_test.go"), tests},
		{filepath.Join(out, "LICENSE-ICONS"), license},
		{filepath.Join(out, "upstream.json"), append(meta, '\n')},
		{filepath.Join(gallery, "icons_gen.go"), list},
	}
	old, err := os.ReadFile(filepath.Join(out, "icons_gen.go"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	reportChanges(old, code)
	for _, file := range files {
		if check {
			existing, err := os.ReadFile(file.name)
			if err != nil {
				return err
			}
			if !bytes.Equal(existing, file.data) {
				return fmt.Errorf("%s is not up to date", file.name)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file.name), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(file.name, file.data, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("Compiled %d icons and %d badges at %s\n", len(icons), len(badges), commit)
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// identifier turns a kebab-case icon name into an exported Go name, one
// that is not the package's own API.
func identifier(name string) (string, error) {
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(name) {
		return "", fmt.Errorf("invalid icon name %q", name)
	}
	parts := strings.Split(name, "-")
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	s := strings.Join(parts, "")
	if s[0] >= '0' && s[0] <= '9' || reserved[s] {
		s = "Icon" + s
	}
	if !token.IsIdentifier(s) {
		return "", fmt.Errorf("invalid identifier %q", s)
	}
	return s, nil
}

// reserved is the package's own API, which no accessor takes.
var reserved = map[string]bool{"Icon": true, "Symbol": true, "Variant": true, "VariantColor": true, "VariantSoft": true, "Reference": true}

// render writes the package's accessors, its tests' list of every icon
// and badge, and the gallery's list of every icon.
func render(icons []compiledIcon, badges []compiledBadge) ([]byte, []byte, []byte, error) {
	var code, tests, list bytes.Buffer
	code.WriteString("// Code generated by kubaigen; DO NOT EDIT.\n\npackage kubai\n\n")
	tests.WriteString("// Code generated by kubaigen; DO NOT EDIT.\n\npackage kubai\n\nvar allIcons = []struct{ name string; get func() *Symbol }{\n")
	list.WriteString("// Code generated by kubaigen; DO NOT EDIT.\n\npackage main\n\nimport kubai \"github.com/nsgo-dev/nsgo-icons-kubai\"\n\n// allIcons is every icon, in upstream's order.\nvar allIcons = []entry{\n")
	symbols := map[string]string{}
	seen := map[string]string{}
	for _, ic := range icons {
		s, err := identifier(ic.name)
		if err != nil {
			return nil, nil, nil, err
		}
		if prev, ok := seen[s]; ok {
			return nil, nil, nil, fmt.Errorf("identifier collision: %s and %s", prev, ic.name)
		}
		seen[s] = ic.name
		symbols[ic.name] = s
	}
	for _, ic := range icons {
		s := symbols[ic.name]
		switch {
		case ic.base != "" && ic.badge != "off":
			b, _ := identifier(ic.badge)
			fmt.Fprintf(&code, "// %s returns the Kubai %s icon: %s with the %s badge.\nfunc %s() *Symbol {\n\treturn sym%s.badged(%q, %s(), badge%s(), %s, %s)\n}\n\nvar sym%s lazySymbol\n\n",
				s, ic.name, symbols[ic.base], ic.badge, s, s, ic.name, symbols[ic.base], b, fmtNum(ic.at[0], 3), fmtNum(ic.at[1], 3), s)
		default:
			fmt.Fprintf(&code, "// %s returns the Kubai %s icon.\nfunc %s() *Symbol { return sym%s.get(%q, %q) }\n\nvar sym%s lazySymbol\n\n",
				s, ic.name, s, s, ic.name, ic.program.text, s)
		}
		fmt.Fprintf(&tests, "{%q, %s},\n", ic.name, s)
		fmt.Fprintf(&list, "{%q, %q, kubai.%s},\n", ic.name, s, s)
	}
	list.WriteString("}\n")
	tests.WriteString("}\n\nvar allBadges = []func() *badge{\n")
	for _, b := range badges {
		id, _ := identifier(b.name)
		tone := uint32(0)
		if b.tone != "" {
			v, err := strconv.ParseUint(strings.TrimPrefix(b.tone, "#"), 16, 32)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("badge %s tone %q", b.name, b.tone)
			}
			tone = uint32(v)
		}
		fmt.Fprintf(&code, "// badge%s is the %s badge.\nfunc badge%s() *badge { return badge%sCache.get(0x%06X, %q, %q) }\n\nvar badge%sCache lazyBadge\n\n",
			id, b.name, id, id, tone, b.gap, b.program.text, id)
		fmt.Fprintf(&tests, "badge%s,\n", id)
	}
	tests.WriteString("}\n")
	var out [3][]byte
	for i, b := range []*bytes.Buffer{&code, &tests, &list} {
		formatted, err := format.Source(b.Bytes())
		if err != nil {
			return nil, nil, nil, err
		}
		out[i] = formatted
	}
	return out[0], out[1], out[2], nil
}

func reportChanges(old, current []byte) {
	pattern := regexp.MustCompile(`(?m)^func ([A-Za-z0-9]+)\(\) \*Symbol`)
	names := func(data []byte) map[string]bool {
		m := map[string]bool{}
		for _, match := range pattern.FindAllSubmatch(data, -1) {
			m[string(match[1])] = true
		}
		return m
	}
	before, after := names(old), names(current)
	var added, removed []string
	for name := range after {
		if !before[name] {
			added = append(added, name)
		}
	}
	for name := range before {
		if !after[name] {
			removed = append(removed, name)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	fmt.Printf("API: %d added, %d removed\n", len(added), len(removed))
	if len(removed) > 0 {
		fmt.Printf("Review removals/renames before accepting: %s\n", strings.Join(removed, ", "))
	}
}
