package app

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) loadUsage() {
	if a.usage == nil {
		a.usageError = "usage database unavailable"
		return
	}
	ctx := context.Background()
	since := time.Now().Add(-a.usageSince)
	summary, err := a.usage.UsageSummary(ctx, since)
	if err != nil {
		a.usageError = err.Error()
		return
	}
	calls, err := a.usage.RecentCallsSince(ctx, since, 50)
	if err != nil {
		a.usageError = err.Error()
		return
	}
	a.usageSummary, a.usageCalls, a.usageError = summary, calls, ""
}

func (a *app) usagePage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	if a.usage == nil {
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			a.iconTile(c, t, iconChart, 44, 20)
			ui.Text(c, msgs.UsageUnavailable()).FontSize(15).Bold()
			ui.Text(c, msgs.UsageNoCallsHint()).FontSize(12).TextColor(t.TextMuted)
		})
		return
	}
	if a.usageSummary == nil && a.usageError == "" {
		a.loadUsage()
	}
	if a.usageError != "" {
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			ui.Text(c, msgs.UsageUnavailable()).FontSize(15).Bold()
			ui.Text(c, a.usageError).FontSize(12).TextColor(t.Danger)
			if ui.Button(c, msgs.UsageRefresh()).Clicked() {
				a.loadUsage()
			}
		})
		return
	}

	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, msgs.UsageTitle()).FontSize(24).Bold()
					ui.Text(c, msgs.UsageSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				a.usageRangeMenu(c, t, msgs)
				if ui.Button(c, msgs.UsageRefresh()).Clicked() {
					a.loadUsage()
				}
			})

			summary := a.usageSummary
			if summary == nil {
				ui.Text(c, msgs.UsageNoCalls()).FontSize(12).TextColor(t.TextMuted)
				return
			}
			ui.Grid(c).Columns(3).Gap(12).Children(func() {
				a.usageCard(c, t, msgs.UsageCalls(), fmt.Sprint(summary.Calls))
				errors := fmt.Sprint(summary.Errors)
				if summary.Calls > 0 {
					errors = fmt.Sprintf("%d (%.1f%%)", summary.Errors, float64(summary.Errors)/float64(summary.Calls)*100)
				}
				a.usageCard(c, t, msgs.UsageErrors(), errors)
				a.usageCard(c, t, msgs.UsageP95(), fmt.Sprintf("%.0f ms", summary.P95MS))
				a.usageCard(c, t, msgs.UsageAvg(), fmt.Sprintf("%.0f ms", summary.AvgMS))
				a.usageCard(c, t, msgs.UsageServers(), fmt.Sprint(summary.Servers))
				a.usageCard(c, t, msgs.UsageTools(), fmt.Sprint(summary.Tools))
			})

			card := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
			card.Children(func() {
				ui.Text(c, msgs.UsageTopTools()).FontSize(14).Bold()
				if summary == nil || len(summary.Top) == 0 {
					ui.Text(c, msgs.UsageNoCalls()).FontSize(12).TextColor(t.TextMuted).Padding(4, 0)
					return
				}
				rows := summary.Top
				columns := []ui.TableColumn{
					{Title: msgs.UsageServer()},
					{Title: msgs.UsageTool()},
					{Title: msgs.UsageCallsShort(), Width: 80, Align: ui.End},
					{Title: msgs.UsageAvgShort(), Width: 90, Align: ui.End},
					{Title: msgs.UsageStatus(), Width: 90},
				}
				ui.Table(c, nil, columns, len(rows), func(row, col int) {
					stat := rows[row]
					switch col {
					case 0:
						ui.Text(c, stat.Server).FontSize(12).SingleLine()
					case 1:
						ui.Text(c, stat.Tool).FontSize(12).Bold().SingleLine()
					case 2:
						ui.Textf(c, "%d", stat.Calls).FontSize(12)
					case 3:
						ui.Textf(c, "%.0f ms", stat.AvgMS).FontSize(12)
					case 4:
						color := t.Success
						if stat.Status != "ok" {
							color = t.Danger
						}
						ui.Text(c, stat.Status).FontSize(11).TextColor(color)
					}
				})
			})

			callsCard := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
			callsCard.Children(func() {
				ui.Text(c, msgs.UsageRecentCalls()).FontSize(14).Bold()
				if len(a.usageCalls) == 0 {
					ui.Text(c, msgs.UsageNoCalls()).FontSize(12).TextColor(t.TextMuted).Padding(4, 0)
					return
				}
				rows := a.usageCalls
				columns := []ui.TableColumn{
					{Title: msgs.UsageTime(), Width: 90},
					{Title: msgs.UsageServer()},
					{Title: msgs.UsageTool()},
					{Title: msgs.UsageStatus(), Width: 90},
					{Title: msgs.UsageLatency(), Width: 90, Align: ui.End},
				}
				ui.Table(c, nil, columns, len(rows), func(row, col int) {
					call := rows[row]
					switch col {
					case 0:
						ui.Text(c, call.Time.Format("15:04:05")).FontSize(11.5).TextColor(t.TextMuted)
					case 1:
						ui.Text(c, call.Server).FontSize(12).SingleLine()
					case 2:
						ui.Text(c, call.Tool).FontSize(12).SingleLine()
					case 3:
						color := t.Success
						if call.Status != "ok" {
							color = t.Danger
						}
						ui.Text(c, call.Status).FontSize(11).TextColor(color)
					case 4:
						ui.Textf(c, "%d ms", call.LatencyMS).FontSize(11.5).TextColor(t.TextMuted)
					}
				})
			})
		})
	})
}

func (a *app) usageCard(c *ui.Context, t *ui.Theme, label, value string) {
	ui.Column(c).Padding(14, 16).Gap(3).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, value).FontSize(22).Bold()
		ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *app) usageRangeMenu(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	label := msgs.UsageLast24h()
	switch a.usageSince {
	case 7 * 24 * time.Hour:
		label = msgs.UsageLast7d()
	case 30 * 24 * time.Hour:
		label = msgs.UsageLast30d()
	}
	ui.MenuButton(c, label, func(m *ui.Menu) {
		for _, option := range []struct {
			label string
			since time.Duration
		}{
			{msgs.UsageLast24h(), 24 * time.Hour},
			{msgs.UsageLast7d(), 7 * 24 * time.Hour},
			{msgs.UsageLast30d(), 30 * 24 * time.Hour},
		} {
			option := option
			if m.Item(option.label).Radio(true).Checked(a.usageSince == option.since).Chosen() {
				a.usageSince = option.since
				a.loadUsage()
			}
		}
	}).Height(30).Padding(0, 12).Radius(8).Border(1, t.Border).FontSize(12)
}
