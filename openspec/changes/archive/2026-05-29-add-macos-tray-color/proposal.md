## Why

The menu bar text renders black, not Claude orange (reported symptom: "color is
wrong"). The `macos-tray-reader` spec already *requires* color coding from the
`c_color` hex returned by `--status`, but it is unimplemented: the tray uses
`systray.SetTitle()`, which renders plain monochrome text. **`fyne.io/systray`
exposes no API to color title text** — black is its ceiling. The `CColor` /
`WColor` fields are parsed and then ignored.

The claudemeter reference achieves orange by setting an `NSAttributedString`
with an `NSColor` (calibrated `0xD9,0x77,0x57`, red on error) directly on the
status item's button — i.e. it bypasses any cross-platform title API.

This change picks a rendering strategy that can actually show color and
implements it. The strategy is a real fork (keep systray vs. go native); see
`design.md`.

## What Changes

- Choose a color-capable rendering path (see `design.md` — recommended:
  **render a colored icon image** consumed by `systray.SetIcon`, keeping the
  existing dependency and cross-platform binary).
- Map `c_color` to the rendered foreground: Claude orange when healthy, the
  status colors (`#32c850` green, `#dc3232` red, etc.) as returned, system red
  on error.
- Render the glyph + percentages (from `align-macos-tray-display`) in that color
  at menu-bar resolution (including @2x for Retina).

This change depends on `align-macos-tray-display` for the glyph/text content and
on `fix-macos-keychain-auth` for `--status` to return real data on macOS.

## Capabilities

### Modified Capabilities

- `macos-tray-reader`: the existing color-coding requirement becomes implemented
  via a color-capable render path, including the Claude-orange healthy state.

## Impact

- Modified: `readers/macos-tray/main.go` rendering path; possibly new helper for
  text→image rendering, or a new darwin-only file if the native option is chosen.
- Dependency impact depends on the chosen option (see `design.md`): Option B adds
  a Go image/font dependency; Option A drops `systray` for cgo Cocoa bindings.
- The `c_color` field finally drives presentation.
