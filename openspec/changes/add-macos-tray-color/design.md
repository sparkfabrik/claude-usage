## Context

`fyne.io/systray` can set: title text (monochrome), tooltip, and an **icon**
(`SetIcon` for a colored PNG, `SetTemplateIcon` for a system-tinted monochrome
template). There is no colored-title API. To show Claude orange we must either
render color into an icon image, or abandon systray for native Cocoa.

## Decision

**Open** — pick one of the options below before implementing. Recommendation: B.

## Options

### Option A — Native `NSStatusItem` (cgo Cocoa)

Drop `systray`; create the status item directly and set an `NSAttributedString`
with `NSColor`, exactly like claudemeter.

```
+ Pixel-perfect parity: native menu-bar font, baseline, spacing, color.
+ Trivial color/error switching (system red on error).
- cgo + Objective-C bindings; darwin-only source file.
- Reimplements menu/click handling that systray gave for free.
- Heaviest change; most maintenance surface.
```

### Option B — Rendered colored icon via `systray.SetIcon`  ← recommended

Render `<glyph> 5h x% · 7d y%` into a small PNG in the target color and set it
as a (non-template) icon; clear the title.

```
+ Keeps systray, keeps the single cross-platform binary, no cgo.
+ Full control of color → Claude orange and c_color states work.
~ Must render text-as-image: font embedding, baseline, width sizing,
  and @2x for Retina; risk of looking subtly off vs. native menu-bar text.
- New Go image/font dependency (e.g. x/image/font + a bundled face).
```

### Option C — Monochrome template icon / text (drop color)

Accept system-tinted monochrome; amend the spec to remove the color requirement.

```
+ Cheapest; nothing to render; respects light/dark menu bar automatically.
- Abandons Claude-orange brand and c_color state coloring — does not fix the
  reported symptom, only reframes it. Listed for completeness.
```

## Comparison

| | A native | B icon | C mono |
|---|---|---|---|
| Claude orange | ✅ | ✅ | ❌ |
| c_color states | ✅ | ✅ | ❌ |
| cgo | yes | no | no |
| keeps systray | no | yes | yes |
| effort | high | medium | low |
| fidelity to claudemeter | exact | close | none |

## Risks / open questions

- B: does rendered text read crisply at 22pt menu-bar height on Retina? Needs a
  quick spike before committing.
- A: do we want a darwin-only divergence from the systray-based readers?
- Both A and B should keep the system-red error treatment from claudemeter.
