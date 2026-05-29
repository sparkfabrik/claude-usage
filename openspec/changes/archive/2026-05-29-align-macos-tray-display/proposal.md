## Why

The Go tray's wording diverges from the `claudemeter` reference it ports. The
reference shows a quota glyph and conversational labels; the Go port shows a
terse `C:42% W:67%` with no glyph, no status line, and a different idle/error
indicator. Reported symptoms: "output wording is different" and "glyph is
missing". This change brings the tray's text to parity with claudemeter.

Reference vs. current:

| element | claudemeter | current Go tray |
|---------|-------------|-----------------|
| title (live) | `◔ 5h 42% · 7d 67%` | `C:42% W:67%` |
| title (idle) | `◌ —` | `C:--` |
| title (error)| `⚠ —` | `C:?` |
| title (stale)| (none) | ` ?` suffix |
| glyph ladder | `◔ ◑ ◕ ●` by 5h % | none |
| menu | `Status: …`, `5h: 42%`, `  resets in 2h15m`, `7d: 67%`, `  resets in 3d01h` | `5h: 42% (resets in 2h 15m)`, `7d: …` |
| refresh item | `Refresh now` | `Refresh Now` |

Glyph ladder (by 5h utilization): `◔` <50, `◑` <75, `◕` <95, `●` ≥95.

## What Changes

- Title format → `<glyph> 5h <c>% · 7d <w>%`, with the glyph chosen from the 5h
  ladder above.
- Idle (Claude not running) → `◌ —`; error → `⚠ —`; keep a staleness indicator
  but apply it within the new format.
- Dropdown gains a top `Status: <state>` item; split each percentage and its
  reset onto two lines (`5h: 42%` / `  resets in 2h15m`); rename `Refresh Now`
  → `Refresh now`.
- Reset duration formatting matches claudemeter (`2h15m`, `3d01h`, `—` for none).
  Decide whether the tray reformats `c_reset`/`w_reset` from `--status` or the
  CLI emits the compact form.

This is display-only. Color is intentionally **out of scope** here and handled
separately (see `add-macos-tray-color`). This change updates the existing
`macos-tray-reader` spec, whose scenarios currently mandate the old wording.

## Capabilities

### Modified Capabilities

- `macos-tray-reader`: menu bar title, dropdown labels, glyph, and idle/error
  indicators aligned to the claudemeter reference.

## Impact

- Modified: `readers/macos-tray/main.go` (`updateDisplay`, `setIdleState`,
  `onReady` menu construction).
- Modified spec: `macos-tray-reader` display scenarios.
- No new dependencies; no change to the `--status` contract beyond the optional
  reset-format decision noted above.
