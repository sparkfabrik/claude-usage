## 1. Title rendering

- [x] 1.1 Add a glyph selector: `◔` <50, `◑` <75, `◕` <95, `●` ≥95 (by 5h `c_pct`)
- [x] 1.2 Render live title as `<glyph> 5h <c_pct>% · 7d <w_pct>%`
- [x] 1.3 Idle (`claude_running == false`) → title `◌ —`
- [x] 1.4 Error (poll/parse failure) → title `⚠ —`
- [x] 1.5 Preserve staleness signal within the new format when `stale == true`

## 2. Dropdown menu

- [x] 2.1 Add a top `Status: <state>` menu item (use `--status` state/error text)
- [x] 2.2 Split 5h into `5h: <c_pct>%` and a sub-line `  resets in <fmt>`
- [x] 2.3 Split 7d into `7d: <w_pct>%` and a sub-line `  resets in <fmt>`
- [x] 2.4 Rename the refresh item `Refresh Now` → `Refresh now`
- [x] 2.5 Keep the separator + `Quit`

## 3. Reset formatting

- [x] 3.1 Decide source of compact reset format (`2h15m` / `3d01h` / `—`): reformat in tray vs. emit from CLI; record the decision — **Decision: use CLI output as-is** (already emits compact `3h06m`/`6d02h` format)
- [x] 3.2 Apply the chosen formatter to both reset sub-lines and idle/empty cases

## 4. Verify

- [x] 4.1 `go build ./readers/macos-tray/` on darwin
- [x] 4.2 Visually confirm title shows the glyph and `5h …% · 7d …%`, dropdown shows the new four-line layout + `Status:`
- [x] 4.3 Confirm idle (`◌ —`) and error (`⚠ —`) states render correctly
