# NSGo Kubai Icons architecture

The repository is one Go module. Package `kubai` at its root draws [Kubai Icons](https://github.com/kubai087/kubai-icons) through NSGo's public `core` API. `tools/kubaigen` compiles the icons from upstream at build time, so apps run no JavaScript.

## Code map

| Path | Responsibility |
| --- | --- |
| `icon.go` | `Icon`: a `Symbol` at a size, in an accent, a variant and a badge color, and its hover motion |
| `symbol.go` | `Symbol` and the lazy accessors' shared parsing |
| `program.go`, `render.go` | The compiled program's format, parsed and played into canvas operations |
| `icons_gen.go`, `icons_gen_test.go` | Generated: an accessor and a test per icon |
| `upstream.json`, `LICENSE-ICONS` | Generated: the upstream commit and inputs, and upstream's license |
| `tools/kubaigen` | The generator: runs upstream and compiles its frames into programs and tracks |
| `examples/gallery` | The icon finder: search, a virtualized grid, and the chosen icon's code |
| `examples/gallery/icons_gen.go` | Generated: every icon's name and accessor, listed only by the gallery |
| `website/` | The static site GitHub Pages serves, with no build step |

## Boundaries

- Use only NSGo's public packages. What the pack needs that NSGo lacks goes into NSGo.
- Keep the package free of a runtime registry: an app links only the icons it calls. Only the gallery lists every icon.
- Never edit generated files; change `tools/kubaigen` and regenerate. Fidelity to upstream is the point.
- Keep `nsgo dev` builds and tests from sending usage statistics.

## Core flows

**Compile.** `kubaigen` runs upstream's renderer in goja at moments of each motion and turns the frames into canvas operations. Numbers that move become tracks over one motion value ([compiling](docs/architecture/icons.md#compiling)).

**Draw.** `Icon` draws the rest pose until the first hover or click. It then binds the moving numbers to an animated value, so the platform plays the motion on its frame clock ([drawing](docs/architecture/icons.md#drawing)).

**Count use.** The website and the gallery each report to a site of their own in Umami (`umami.oneo.me`), as NSGo's do. The website loads Umami's script on `oneo-me.github.io` only. The gallery posts its events itself.

## Data and state

The gallery keeps two values in its storage: a random install ID, which makes one install one visitor, and `stats`, `off` when its panel turned the statistics off.

## Verification

- `go tool nsgo test` parses and draws every icon and badge, plays a hover and a click, and checks the generator's paths, tracks and variants.
- It also runs the gallery headless: the grid stays virtualized within the window, search ranks and filters, and the usage is gofmt's.
- `go tool nsgo test -target macos examples/gallery` screenshots the gallery at rest, on hover and searched. `website/assets/gallery.png` is its `gallery.png`, 1600 pixels wide.
- Review motion in a desktop session; a screenshot shows one moment.

## Subjects

| Document | When to read |
| --- | --- |
| [Icons](docs/architecture/icons.md) | Changing how `kubaigen` compiles icons or how `Icon` draws and animates them |
