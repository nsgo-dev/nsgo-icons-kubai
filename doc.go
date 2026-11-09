// Package kubai provides Kubai Icons for NSGo: layered, colored icons that
// move on hover, one accessor per icon, drawn by Icon. Click plays the
// motion once on a press.
//
//	kubai.Icon{Symbol: kubai.Folder(), Size: 32, Label: "Folder"}
//
// Upstream draws each icon with a program that moves its layers. kubaigen
// runs that program at moments of the motion and compiles the frames into
// canvas operations whose moving numbers follow tracks; Icon binds them to
// an animated value, so the platform plays the motion on its frame clock.
//
// Each accessor parses its program on first use and returns the same
// shared Symbol afterwards. There is no runtime registry. Names follow
// upstream kebab-case converted to PascalCase (folder-plus becomes
// FolderPlus); a name starting with a digit or taken by the package's own
// API takes the Icon prefix. A badged icon is its base with the badge
// drawn over its corner.
//
// The generated assets are pinned in upstream.json and covered by
// upstream's MIT license in LICENSE-ICONS.
package kubai
