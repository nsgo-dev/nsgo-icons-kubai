# NSGo Kubai Icons

NSGo Kubai Icons is Kubai Icons for NSGo as a library of its own: layered icons that move on hover. Read [ARCHITECTURE.md](ARCHITECTURE.md) before changing code.

## Principles

- NSGo comes from the commit `go.mod` requires. When the pack needs a change in NSGo, land it on NSGo's `main` first, following that repository's own instructions, then run `go get github.com/oneo-me/nsgo@<commit>`.
- Keep the [boundaries](ARCHITECTURE.md#boundaries): NSGo's public packages only, no runtime registry, generated files only from `tools/kubaigen`.
- Keep `LICENSE` for this repository's code and `LICENSE-ICONS` for upstream's icons.

## Working with the user

- Never create sub-Agents or Git worktrees; work directly in the current checkout.
- Do not commit, tag, push or publish without explicit authorization.

## Verification

Run the checks in [Verification](ARCHITECTURE.md#verification) from the repository root. After changing the gallery's look, refresh `website/assets/gallery.png` from its new screenshot. A green headless run does not prove motion: review changes to drawing or hover in a desktop session.

## Documentation

- Update a document only when a fact its reader needs changes.
- Keep each document within its budget and written for its reader.
- Record completed user-visible outcomes as one-clause `Unreleased` bullets in [CHANGELOG.md](CHANGELOG.md).
- Repair a document you touch instead of appending to it.
- Keep `README.md` and `README.zh-CN.md` equivalent; everything else, including code comments, is English.
