package app

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) loadClients() {
	if a.syncManager == nil {
		a.clientStatuses = nil
		return
	}
	if a.usage != nil {
		if profiles, err := a.usage.ListProfiles(context.Background()); err == nil {
			a.gwProfiles = profiles
		}
	}
	a.clientStatuses = a.syncManager.Statuses()
	for _, status := range a.clientStatuses {
		if status.Icon == "" {
			continue
		}
		raw, err := os.ReadFile(status.Icon)
		if err != nil {
			continue
		}
		if strings.HasSuffix(status.Icon, ".svg") {
			if svg, err := ui.ParseSVG(raw); err == nil {
				clientSVGs[status.Client] = svg
			}
		} else if bitmap, err := ui.DecodeBitmap(raw); err == nil {
			clientBitmaps[status.Client] = bitmap
		}
	}
}

func (a *app) clientEndpoint() string {
	profile := a.clientProfile
	if profile == "" {
		profile = "default"
	}
	return "http://" + a.gwAddr + "/mcp/" + profile
}

func (a *app) clientsPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	if a.syncManager == nil {
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			ui.Text(c, msgs.ClientsNo()).FontSize(15).TextColor(t.TextMuted)
		})
		return
	}
	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			profiles := make([]string, 0, len(a.gwProfiles))
			for _, profile := range a.gwProfiles {
				profiles = append(profiles, profile.Name)
			}
			if len(profiles) == 0 {
				profiles = []string{"default"}
			}
			known := false
			for _, profile := range profiles {
				if profile == a.clientProfile {
					known = true
					break
				}
			}
			if !known {
				a.clientProfile = profiles[0]
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, msgs.ClientsTitle()).FontSize(24).Bold()
					ui.Text(c, msgs.ClientsSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				ui.Text(c, msgs.ClientsProfile()).FontSize(11.5).TextColor(t.TextMuted)
				ui.Select(c, &a.clientProfile, profiles).Width(160)
				if ui.Button(c, msgs.GatewayRefresh()).Clicked() {
					a.loadClients()
				}
			})
			ui.Textf(c, "%s: %s", msgs.ClientsEndpoint(), a.clientEndpoint()).
				FontSize(11.5).TextColor(t.TextMuted)
			if !a.gwRunning {
				ui.Text(c, msgs.GatewayStartFirst()).FontSize(11.5).TextColor(t.Warning)
			}

			card := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
			card.Children(func() {
				if len(a.clientStatuses) == 0 {
					ui.Text(c, msgs.ClientsNo()).FontSize(12).TextColor(t.TextMuted)
					return
				}
				rows := a.clientStatuses
				columns := []ui.TableColumn{
					{Title: msgs.ClientsClient(), Width: 150},
					{Title: msgs.ClientsFile()},
					{Title: msgs.ClientsDetected(), Width: 100},
					{Title: msgs.ClientsManaged(), Width: 100},
					{Title: msgs.ServiceActions(), Width: 250, Fixed: true},
				}
				ui.Table(c, nil, columns, len(rows), func(row, col int) {
					status := rows[row]
					switch col {
					case 0:
						ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
							clientIcon(c, status.Client, 16, t.TextMuted)
							ui.Text(c, status.Label).FontSize(12).Bold().SingleLine()
						})
					case 1:
						ui.Text(c, status.File).FontSize(11).TextColor(t.TextMuted).SingleLine()
					case 2:
						value, color := msgs.ClientsDetectedNo(), t.TextMuted
						if status.Present {
							value, color = msgs.ClientsDetectedYes(), t.Success
						}
						ui.Text(c, value).FontSize(11).TextColor(color)
					case 3:
						value, color := msgs.ClientsManagedNo(), t.TextMuted
						if status.Managed {
							value, color = msgs.ClientsManagedYes(), t.Accent
							if status.Drifted {
								value, color = msgs.ClientsDrifted(), t.Warning
							}
						}
						ui.Text(c, value).FontSize(11).TextColor(color)
					case 4:
						ui.Row(c).Gap(4).Children(func() {
							if ui.PrimaryButton(c, msgs.ClientsConnect()).FontSize(11).Padding(3, 8).
								Disabled(!a.gwRunning).Clicked() {
								a.connectClient(status.Client, c, msgs)
							}
							if ui.Button(c, msgs.ClientsPreview()).FontSize(11).Padding(3, 8).Clicked() {
								a.planClient(status.Client, c)
							}
							if status.Managed {
								if ui.Button(c, msgs.ClientsDisconnect()).FontSize(11).Padding(3, 8).Clicked() {
									a.disconnectClient(status.Client, c, msgs)
								}
							}
						})
					}
				})
			})
		})
	})
}

func (a *app) connectClient(id string, c *ui.Context, msgs locales.Messages) {
	if !a.gwRunning {
		c.Toast(msgs.GatewayStartFirst())
		return
	}
	plan, err := a.syncManager.Apply(id, a.clientEndpoint(), "mymcp", false)
	if err != nil {
		if plan != nil {
			a.clientPlan, a.clientPlanClient, a.clientPlanOpen = plan, id, true
		}
		c.Toast(err.Error())
		a.loadClients()
		return
	}
	a.loadClients()
	c.Toast(msgs.ClientsApplied())
}

func (a *app) planClient(id string, c *ui.Context) {
	plan, err := a.syncManager.Plan(id, a.clientEndpoint(), "mymcp")
	if err != nil {
		c.Toast(err.Error())
		return
	}
	a.clientPlan, a.clientPlanClient, a.clientPlanOpen = plan, id, true
}

func (a *app) disconnectClient(id string, c *ui.Context, msgs locales.Messages) {
	if err := a.syncManager.Detach(id, false); err != nil {
		if strings.Contains(err.Error(), "--force") {
			a.clientDetachForce = id
			return
		}
		c.Toast(err.Error())
		return
	}
	a.loadClients()
	c.Toast(msgs.ClientsDetached())
}

// detachDialog offers a force detach when a managed entry changed after
// MyMCP wrote it.
func (a *app) detachDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	open := a.clientDetachForce != ""
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			ui.Text(c, msgs.ClientsForceDetachTitle()).FontSize(17).Bold()
			ui.Text(c, msgs.ClientsForceDetachMessage()).FontSize(12.5).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.clientDetachForce = ""
				}
				if ui.PrimaryButton(c, msgs.ClientsForceDetach()).Clicked() {
					id := a.clientDetachForce
					a.clientDetachForce = ""
					if err := a.syncManager.Detach(id, true); err != nil {
						c.Toast(err.Error())
						return
					}
					a.loadClients()
					c.Toast(msgs.ClientsDetached())
				}
			})
		})
	})
	if !open {
		a.clientDetachForce = ""
	}
}

func (a *app) clientPlanDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.clientPlanOpen, func() {
		ui.Column(c).Width(680).Gap(12).Children(func() {
			ui.Text(c, msgs.ClientsPlanTitle()).FontSize(18).Bold()
			if a.clientPlan == nil {
				return
			}
			ui.Text(c, msgs.ClientsPlanHint()).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, a.clientPlan.File).FontSize(11).TextColor(t.TextMuted).SingleLine()
			preview := ""
			if text, ok := a.clientPlan.Desired.(string); ok {
				preview = text
			} else {
				raw, _ := json.MarshalIndent(a.clientPlan.Desired, "", "  ")
				preview = string(raw)
			}
			ui.TextArea(c, &preview).ReadOnly(true).Height(240).FontSize(11.5)
			if a.clientPlan.Conflict {
				ui.Text(c, msgs.ClientsConflict()).FontSize(12).TextColor(t.Warning)
				if a.clientPlan.Reason != "" {
					ui.Text(c, a.clientPlan.Reason).FontSize(11).TextColor(t.TextMuted)
				}
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.clientPlanOpen = false
				}
				action := msgs.ClientsApply()
				if a.clientPlan.Conflict {
					action = msgs.ClientsOverwrite()
				}
				if ui.PrimaryButton(c, action).Clicked() {
					client := a.clientPlanClient
					conflict := a.clientPlan.Conflict
					a.clientPlanOpen = false
					plan, err := a.syncManager.Apply(client, a.clientEndpoint(), "mymcp", conflict)
					if err != nil {
						if plan != nil {
							a.clientPlan, a.clientPlanClient, a.clientPlanOpen = plan, client, true
						}
						c.Toast(err.Error())
						return
					}
					a.loadClients()
					c.Toast(msgs.ClientsApplied())
				}
			})
		})
	})
}
