## 1. Decide render strategy

- [x] 1.1 Resolve `design.md` open decision (A native / B icon / C mono); record choice
- [x] 1.2 If B: spike text→image render at 22pt + @2x and eyeball crispness before committing

## 2. Implement color path

- [x] 2.1 Map `c_color` hex → render color; default healthy = Claude orange `#D97757` (`0xD9,0x77,0x57`)
- [x] 2.2 Apply status colors as returned (e.g. `#32c850` green, `#dc3232` red)
- [x] 2.3 Error state → system red, matching claudemeter
- [x] 2.4 Render the glyph + `5h x% · 7d y%` content (from `align-macos-tray-display`) in the chosen color via the chosen path

## 3. Verify

- [x] 3.1 `go build ./readers/macos-tray/` (and confirm any new dep / cgo builds clean)
- [x] 3.2 Confirm menu bar shows Claude orange in the healthy state, not black
- [x] 3.3 Confirm color tracks `c_color` across states and goes red on error
- [x] 3.4 Confirm legibility in both light and dark menu bars
