# Icons

Upstream draws an icon with JavaScript: `target()` gives the numbers at rest and hovered, and `draw()` writes their mix into the layers' attributes. This library runs that code once, at build time, and draws what it produced with NSGo's canvas.

## Compiling

`tools/kubaigen` compiles every icon and badge:

1. It bundles upstream's renderer and definitions with esbuild and runs them in goja.
2. It renders each icon at 49 moments of the way in and 49 of the way out, with upstream's phase, so what swings settles instead of replaying.
3. It reads each frame's markup into a program of canvas operations, so the commands are the same in every frame.
4. It turns the numbers that move into tracks, and frames whose operations differ into variants.

A program keeps upstream's look with what the canvas has:

- Transforms stay operations. Every shape becomes a path of lines and curves, each arc four curves.
- An outside outline and a badge's gap clip outside their shape. The shadow is the canvas's.
- The light rim is the layer less its copy 0.7 units down, overlaid in white.
- Colors stay references the runtime tints with the accent.
- A badged icon is its base with the badge's own program over the corner; `-off` icons compile whole.

Tracks run over a motion value: 0 at rest, 1 hovered and 2 at rest again, placed by the eased progress. They are simplified to within 0.008 units, written once and shared by numbers a constant apart. A track whose way out retraces its way in keeps one half. A variant is drawn only while the motion is in its frames.

The generator writes `icons_gen.go`, `icons_gen_test.go`, `LICENSE-ICONS` and `upstream.json` to `-out`, the repository root by default. It writes the gallery's list to `-gallery`. It reports the masks it left out and the accessors added and removed, and `-check` verifies the files instead of writing them.

Names are upstream's in PascalCase. A name with a leading digit, or one the package's API holds, takes the `Icon` prefix.

## Drawing

`Icon` draws the rest pose alone until the first hover. Then its drawing binds every moving number to an `Interpolate` of one `AnimatedValue`. Those are made inside the canvas's `Draw`, so they go with the drawing.

A 460 ms timing on upstream's curve moves the value. A hover that turns back jumps to the same pose on the other half. Accessors parse their program on first use and share it afterwards.

## Goals

The pack does not yet do these of upstream's:

- The light rim's blur.
- Luminance masks, such as `shuffle`'s and `ufo`'s fades.
- Tap icons' toggled state, and a touch playing the motion forward and back.

It has been checked on macOS only.
