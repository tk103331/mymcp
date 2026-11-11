package app

import (
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) loadOverview() {
	a.loadGateway()
	a.loadClients()
	a.usageSince = 24 * time.Hour
	a.loadUsage()
}

func (a *app) overviewPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, msgs.OverviewTitle()).FontSize(24).Bold()
					ui.Text(c, msgs.OverviewSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				a.gatewayStatus(c, t, msgs)
				if a.gwRunning || a.gwStarting {
					if ui.Button(c, msgs.GatewayRestart()).Disabled(a.gwStarting).Clicked() {
						a.restartGateway(msgs)
					}
					if ui.Button(c, msgs.GatewayStop()).Disabled(a.gwStarting).Clicked() {
						a.stopGateway()
					}
				} else if ui.PrimaryButton(c, msgs.GatewayStart()).Clicked() {
					a.startGateway(msgs)
				}
			})
			if a.gwError != "" {
				ui.Text(c, a.gwError).FontSize(12).TextColor(t.Danger)
			}

			ui.Column(c).Padding(18).Gap(8).Radius(14).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, msgs.GatewayEndpoint()).FontSize(13).Bold()
				endpoint := "http://" + a.gwAddr + "/mcp/<profile>"
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, endpoint).FontSize(12).TextColor(t.TextMuted).Grow(1).SingleLine()
					if ui.Button(c, "").Label(msgs.Copy()).Tooltip(msgs.Copy()).Padding(3, 6).
						Children(func() { ui.Icon(c, iconCopy).Size(12, 12) }).Clicked() {
						mygo.Clipboard.WriteText(endpoint)
						c.Toast(msgs.Copied())
					}
				})
			})

			tools := 0
			for _, status := range a.gwStatus {
				tools += status.Tools
			}
			managed := 0
			for _, status := range a.clientStatuses {
				if status.Managed {
					managed++
				}
			}
			var calls, errors int64
			if a.usageSummary != nil {
				calls, errors = a.usageSummary.Calls, a.usageSummary.Errors
			}
			ui.Grid(c).Columns(3).Gap(12).Children(func() {
				a.usageCard(c, t, msgs.GatewayServers(), strconv.Itoa(len(a.gwServers)))
				a.usageCard(c, t, msgs.UsageTools(), strconv.Itoa(tools))
				a.usageCard(c, t, msgs.GatewayProfiles(), strconv.Itoa(len(a.gwProfiles)))
				a.usageCard(c, t, msgs.OverviewManagedClients(), strconv.Itoa(managed))
				a.usageCard(c, t, msgs.OverviewCalls24h(), strconv.FormatInt(calls, 10))
				a.usageCard(c, t, msgs.OverviewErrors24h(), strconv.FormatInt(errors, 10))
			})

			actions := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
			actions.Children(func() {
				ui.Text(c, msgs.OverviewQuickActions()).FontSize(14).Bold()
				ui.Row(c).Gap(8).Wrap().Children(func() {
					if ui.PrimaryButton(c, msgs.GatewayAddServer()).Clicked() {
						a.openAddServerDialog(c, msgs)
					}
					if ui.Button(c, msgs.GatewayImport()).Clicked() {
						a.gwImportOpen, a.gwImportStats, a.gwImportError = true, nil, ""
					}
					if ui.Button(c, msgs.NavCatalog()).Clicked() {
						a.router.Push("/catalog")
					}
					if ui.Button(c, msgs.NavClients()).Clicked() {
						a.router.Push("/clients")
					}
					if ui.Button(c, msgs.NavDebug()).Clicked() {
						a.router.Push("/debug")
					}
				})
			})

			if a.gwRunning {
				a.gatewayHealth(c, t, msgs)
			}

			recent := ui.Column(c).Padding(18).Gap(9).Radius(14).Background(t.Surface).Border(1, t.Border)
			recent.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, msgs.OverviewRecentCalls()).FontSize(14).Bold()
					ui.Spacer(c)
					if ui.Button(c, msgs.NavUsage()).Clicked() {
						a.router.Push("/usage")
					}
				})
				rows := a.usageCalls
				if len(rows) > 8 {
					rows = rows[:8]
				}
				if len(rows) == 0 {
					ui.Text(c, msgs.OverviewNoCalls()).FontSize(12).TextColor(t.TextMuted)
					return
				}
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
