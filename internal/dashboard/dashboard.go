// Package dashboard renders the CLI dashboard output.
package dashboard

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/sparkfabrik/claude-usage/internal/analyzer"
	"github.com/sparkfabrik/claude-usage/internal/auth"
	"github.com/sparkfabrik/claude-usage/internal/cache"
	"github.com/sparkfabrik/claude-usage/internal/config"
	"github.com/sparkfabrik/claude-usage/internal/reader"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dimStyle    = lipgloss.NewStyle().Faint(true)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	orangeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	// Base cell style for table rendering (pre-created to avoid per-cell allocation).
	cellStyle = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
)

// DimText renders text in a faint/dim style.
func DimText(s string) string {
	return dimStyle.Render(s)
}

func utilizationStyle(pct float64, cfg *config.Config) lipgloss.Style {
	if pct >= float64(cfg.Colors.OrangeBelow) {
		return redStyle.Bold(true)
	}
	if pct >= float64(cfg.Colors.GreenBelow) {
		return orangeStyle
	}
	return greenStyle
}

// FormatTimeRemaining formats minutes as human-readable duration.
func FormatTimeRemaining(minutes *float64) string {
	if minutes == nil {
		return "?"
	}
	m := *minutes
	if m < 1 {
		return "<1m"
	}
	if m < 60 {
		return fmt.Sprintf("%dm", int(m))
	}
	hours := int(m) / 60
	mins := int(m) % 60
	if hours < 24 {
		return fmt.Sprintf("%dh%02dm", hours, mins)
	}
	days := hours / 24
	rh := hours % 24
	return fmt.Sprintf("%dd%02dh", days, rh)
}

func makeBar(fraction float64, width int, cfg *config.Config) string {
	pct := fraction * 100
	filled := int(fraction * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	style := utilizationStyle(pct, cfg)
	return style.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", empty))
}

// RenderAccount renders account info.
func RenderAccount(creds *auth.Credentials) string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("40")).
		Padding(0, 1).
		Width(60)

	if creds == nil {
		border = border.BorderForeground(lipgloss.Color("196"))
		return border.Render(
			redStyle.Bold(true).Render("No Claude Code login found!") + "\n" +
				dimStyle.Render("Run 'claude' to authenticate first."),
		)
	}

	sub := creds.DisplaySubscription()
	tier := creds.DisplayTier()
	expired := ""
	if creds.IsExpired() {
		expired = "\n  " + redStyle.Render("Auth expired") + dimStyle.Render(" — run Claude Code to refresh")
	}

	return border.Render(
		titleStyle.Render("Account") + "\n" +
			fmt.Sprintf("  Plan: %s\n  Rate Tier: %s", boldStyle.Render(sub), dimStyle.Render(tier)) + expired,
	)
}

// RenderQuota renders rate limit quota panel.
func RenderQuota(q *cache.QuotaCache, cfg *config.Config) string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(60)

	if q == nil {
		return border.Render(
			titleStyle.Render("Rate Limits") + "\n" +
				dimStyle.Render("  No quota data. Run with --force-poll to fetch."),
		)
	}

	staleMark := ""
	if q.IsStale() {
		staleMark = dimStyle.Render(" (?)")
	}

	pct5h := q.Utilization5h * 100
	pct7d := q.Utilization7d * 100
	r5 := FormatTimeRemaining(q.MinutesToReset5h())
	r7 := FormatTimeRemaining(q.MinutesToReset7d())
	bar5h := makeBar(q.Utilization5h, 20, cfg)
	bar7d := makeBar(q.Utilization7d, 20, cfg)

	style5h := utilizationStyle(pct5h, cfg)
	style7d := utilizationStyle(pct7d, cfg)

	title := titleStyle.Render("Rate Limits")
	if q.Status5h != "" {
		title += dimStyle.Render(fmt.Sprintf(" | %s", q.Status5h))
	}

	body := title + "\n" +
		fmt.Sprintf("  5h  %s  %s  resets in %s%s\n", bar5h, style5h.Render(fmt.Sprintf("%5.1f%%", pct5h)), r5, staleMark) +
		fmt.Sprintf("  7d  %s  %s  resets in %s%s", bar7d, style7d.Render(fmt.Sprintf("%5.1f%%", pct7d)), r7, staleMark)

	// Model-scoped windows sit below the two flat ones. They exist only when
	// the OAuth source reported them, and a window that has already reset is
	// dropped rather than shown with a stale percentage.
	for _, w := range q.OpenWindows(time.Now()) {
		if w.Model == "" {
			continue
		}
		pct := w.Utilization * 100
		// A narrower bar and a spelled-out name: these windows are a
		// different class from the two headline ones, and "Fab" would be a
		// poor trade for keeping the columns identical.
		body += fmt.Sprintf("\n  %-11s %s  %s  resets in %s%s",
			truncateLabel(w.Model, 11),
			makeBar(w.Utilization, 12, cfg),
			utilizationStyle(pct, cfg).Render(fmt.Sprintf("%5.1f%%", pct)),
			FormatTimeRemaining(w.MinutesToReset()),
			staleMark)
	}

	return border.Render(body)
}

// truncateLabel shortens a model name to fit the fixed-width quota column.
func truncateLabel(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// costFootnote clarifies that the cost column is an estimate based on public
// API pricing, not actual spend (subscription plans are billed at a flat rate).
const costFootnote = "* Estimated pay-as-you-go API cost. Subscription plans are billed flat."

// RenderUsageTable renders usage summaries as a proper table.
func RenderUsageTable(summaries []*analyzer.Summary, showCost bool) string {
	if len(summaries) == 0 {
		return dimStyle.Render("No usage data.")
	}

	headers := []string{"Period", "Msgs", "Input", "Output", "Cache W/R", "Total"}
	if showCost {
		headers = append(headers, "Est. API Cost")
	}

	const (
		colPeriod = 0
		colTotal  = 5
		colCost   = 6
	)

	rows := make([][]string, 0, len(summaries))
	for _, s := range summaries {
		cacheStr := fmt.Sprintf("%s/%s",
			reader.FormatTokens(s.TotalCacheWriteTokens),
			reader.FormatTokens(s.TotalCacheReadTokens))
		row := []string{
			s.PeriodLabel,
			fmt.Sprintf("%d", s.MessageCount),
			reader.FormatTokens(s.TotalInputTokens),
			reader.FormatTokens(s.TotalOutputTokens),
			cacheStr,
			reader.FormatTokens(s.TotalTokens()),
		}
		if showCost {
			row = append(row, fmt.Sprintf("$%.2f", s.TotalCostUSD))
		}
		rows = append(rows, row)
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("238"))).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := cellStyle
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(lipgloss.Color("39"))
			}
			if col > colPeriod {
				s = s.Align(lipgloss.Right)
			}
			if col == colTotal {
				s = s.Bold(true)
			}
			if showCost && col == colCost {
				s = s.Foreground(lipgloss.Color("40"))
			}
			return s
		})

	out := titleStyle.Render("Usage by Period") + "\n" + t.Render()
	if showCost {
		out += "\n" + dimStyle.Render(costFootnote)
	}
	return out
}

// RenderModelTable renders per-model breakdown.
func RenderModelTable(summaries []*analyzer.Summary, showCost bool) string {
	if len(summaries) == 0 {
		return ""
	}

	headers := []string{"Model", "Msgs", "Input", "Output", "Cache W/R", "Total"}
	if showCost {
		headers = append(headers, "Est. API Cost")
	}

	const (
		colModelName  = 0
		colModelTotal = 5
		colModelCost  = 6
	)

	rows := make([][]string, 0, len(summaries))
	for _, s := range summaries {
		cacheStr := fmt.Sprintf("%s/%s",
			reader.FormatTokens(s.TotalCacheWriteTokens),
			reader.FormatTokens(s.TotalCacheReadTokens))
		row := []string{
			s.PeriodLabel,
			fmt.Sprintf("%d", s.MessageCount),
			reader.FormatTokens(s.TotalInputTokens),
			reader.FormatTokens(s.TotalOutputTokens),
			cacheStr,
			reader.FormatTokens(s.TotalTokens()),
		}
		if showCost {
			row = append(row, fmt.Sprintf("$%.2f", s.TotalCostUSD))
		}
		rows = append(rows, row)
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("238"))).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := cellStyle
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(lipgloss.Color("170"))
			}
			if col > colModelName {
				s = s.Align(lipgloss.Right)
			}
			if col == colModelTotal {
				s = s.Bold(true)
			}
			if showCost && col == colModelCost {
				s = s.Foreground(lipgloss.Color("40"))
			}
			return s
		})

	out := titleStyle.Render("Usage by Model") + "\n" + t.Render()
	if showCost {
		out += "\n" + dimStyle.Render(costFootnote)
	}
	return out
}

// RenderBurnRate renders the burn rate line.
func RenderBurnRate(summaries []*analyzer.Summary) string {
	if len(summaries) == 0 {
		return ""
	}
	last := summaries[len(summaries)-1]
	rate := last.BurnRatePerMinute()
	if rate < 0 {
		return ""
	}
	return dimStyle.Render(fmt.Sprintf("Burn rate (%s): %s/min", last.PeriodLabel, reader.FormatTokens(int(math.Round(rate)))))
}
