## 1. Title rendering

- [ ] 1.1 Add a glyph selector: `◔` <50, `◑` <75, `◕` <95, `●` ≥95 (by 5h `c_pct`)
- [ ] 1.2 Render live title as `<glyph> 5h <c_pct>% · 7d <w_pct>%`
- [ ] 1.3 Idle (`claude_running == false`) → title `◌ —`
- [ ] 1.4 Error (poll/parse failure) → title `⚠ —`
- [ ] 1.5 Preserve staleness signal within the new format when `stale == true`

## 2. Dropdown menu

- [ ] 2.1 Add a top `Status: <state>` menu item (use `--status` state/error text)
- [ ] 2.2 Split 5h into `5h: <c_pct>%` and a sub-line `  resets in <fmt>`
- [ ] 2.3 Split 7d into `7d: <w_pct>%` and a sub-line `  resets in <fmt>`
- [ ] 2.4 Rename the refresh item `Refresh Now` → `Refresh now`
- [ ] 2.5 Keep the separator + `Quit`

## 3. Reset formatting

- [ ] 3.1 Decide source of compact reset format (`2h15m` / `3d01h` / `—`): reformat in tray vs. emit from CLI; record the decision
- [ ] 3.2 Apply the chosen formatter to both reset sub-lines and idle/empty cases

## 4. Verify

- [ ] 4.1 `go build ./readers/macos-tray/` on darwin
- [ ] 4.2 Visually confirm title shows the glyph and `5h …% · 7d …%`, dropdown shows the new four-line layout + `Status:`
- [ ] 4.3 Confirm idle (`◌ —`) and error (`⚠ —`) states render correctly
