package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
)

// iconSet is the upstream definitions: the base icons in name order, the
// badges by name and the slash, as the build loads them.
type iconSet struct {
	bases   []string
	badges  []string
	scripts string
}

// bundle builds one script that runs in goja and exposes the upstream
// renderer and every definition as globalThis.kubai.
func bundle(upstream string) (iconSet, error) {
	list := func(dir string) ([]string, error) {
		entries, err := os.ReadDir(filepath.Join(upstream, dir))
		if err != nil {
			return nil, err
		}
		var names []string
		for _, e := range entries {
			if n := e.Name(); strings.HasSuffix(n, ".js") && !strings.HasPrefix(n, "_") {
				names = append(names, strings.TrimSuffix(n, ".js"))
			}
		}
		sort.Strings(names)
		return names, nil
	}
	var set iconSet
	var err error
	if set.bases, err = list("icons"); err != nil {
		return set, err
	}
	if set.badges, err = list("badges"); err != nil {
		return set, err
	}
	var src strings.Builder
	src.WriteString("import { withBadge, withSlash } from './packages/core/src/compose.js';\n")
	src.WriteString("import { renderInner } from './packages/core/src/render.js';\n")
	src.WriteString("import { EASE, PHASE } from './packages/core/src/motion.js';\n")
	src.WriteString("import slash from './badges/_slash.js';\n")
	for i, n := range set.bases {
		fmt.Fprintf(&src, "import i%d from './icons/%s.js';\n", i, n)
	}
	for i, n := range set.badges {
		fmt.Fprintf(&src, "import b%d from './badges/%s.js';\n", i, n)
	}
	src.WriteString("const bases = [")
	for i := range set.bases {
		fmt.Fprintf(&src, "i%d,", i)
	}
	src.WriteString("];\nconst badges = {")
	for i, n := range set.badges {
		fmt.Fprintf(&src, "%q: b%d,", n, i)
	}
	src.WriteString("};\n")
	src.WriteString(sampler)
	result := api.Build(api.BuildOptions{
		Stdin:    &api.StdinOptions{Contents: src.String(), ResolveDir: upstream, Sourcefile: "entry.js"},
		Bundle:   true,
		Format:   api.FormatIIFE,
		Target:   api.ES2017,
		Write:    false,
		LogLevel: api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		return set, fmt.Errorf("bundle: %s", result.Errors[0].Text)
	}
	set.scripts = string(result.OutputFiles[0].Contents)
	return set, nil
}

// sampler renders an icon's markup at given moments of its motion. The
// way out mirrors the runtime: from the hover pose back to rest, with the
// phase draw() reads to settle what swings instead of replaying it.
const sampler = `
const defs = new Map();
for (const base of bases) {
  defs.set(base.name, base);
  for (const v of base.variants || []) {
    const badge = badges[v];
    if (!badge) throw new Error(base.name + ': unknown badge ' + v);
    defs.set(base.name + '-' + v, withBadge(base, badge));
  }
  if (base.off) defs.set(base.name + '-off', withSlash(base, slash));
}
const back = new Map();
const swapped = (def) => {
  let w = back.get(def);
  if (!w) {
    w = Object.assign({}, def, { target: (h, s) => def.target(!h, s) });
    back.set(def, w);
  }
  return w;
};
// A badge alone, at the origin, for composites the runtime draws as their
// base with the badge over its corner.
const glyphs = new Map();
for (const [name, badge] of Object.entries(badges)) {
  glyphs.set(name, {
    name: '#' + name,
    svg: (u) => '<g data-badge="#33363D">' + badge.svg(u) + '</g>',
    target: badge.target,
    draw: badge.draw,
  });
}
globalThis.kubai = {
  badgeNames: () => [...glyphs.keys()],
  badgeTone: (name) => badges[name].tone || '',
  badgeAt: (name) => defs.get(name).badgeAt || [22.6, 18.4],
  // badgeScale is the badge's hover pop at moments ts in, then out, which
  // the gap round it follows.
  badgeScale: (name, ts) => {
    const b = badges[name], rest = b.target(false, {}), hover = b.target(true, {}), out = [];
    for (const dir of [false, true]) {
      for (const t of ts) {
        const e = EASE(t), from = dir ? hover : rest, to = dir ? rest : hover;
        out.push(((from.s || 1) + ((to.s || 1) - (from.s || 1)) * e));
      }
    }
    return out;
  },
  names: () => [...defs.keys()],
  meta: (name) => {
    const d = defs.get(name);
    return { base: d.base || '', badge: d.badge || '', tone: d.tone || '', tap: Boolean(d.tap) };
  },
  baseMeta: () => bases.map((b) => ({ name: b.name, category: b.category || '', tags: (b.tags || []).join(','), variants: (b.variants || []).join(','), off: Boolean(b.off) })),
  // frames renders the icon at moments ts (0 to 1) of its motion in, then
  // of its motion out.
  frames: (name, ts) => {
    const def = defs.get(name) || glyphs.get(name.slice(1)), out = [];
    for (const dir of [false, true]) {
      const d = dir ? swapped(def) : def;
      for (const t of ts) {
        const e = EASE(t);
        globalThis[PHASE] = { out: dir, e };
        try {
          out.push(renderInner(d, { k: 'K', paint: 'template', names: false, progress: e }));
        } finally {
          globalThis[PHASE] = undefined;
        }
      }
    }
    return out;
  },
};
`

// runtime is one goja runtime with the bundle loaded.
type jsRuntime struct {
	vm     *goja.Runtime
	frames func(string, []float64) []string
}

func newRuntime(script string) (*jsRuntime, error) {
	vm := goja.New()
	if _, err := vm.RunString(script); err != nil {
		return nil, err
	}
	k := vm.Get("kubai").ToObject(vm)
	r := &jsRuntime{vm: vm}
	if err := vm.ExportTo(k.Get("frames"), &r.frames); err != nil {
		return nil, err
	}
	return r, nil
}
