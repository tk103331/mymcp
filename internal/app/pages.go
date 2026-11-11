package app

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/manager"
	"github.com/tk103331/mymcp/internal/model"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) workspacesPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(22).Children(func() {
			var action func()
			if len(a.workspaces) > 0 {
				action = func() {
					if ui.PrimaryButton(c, msgs.CreateWorkspace()).Clicked() {
						a.newName, a.newDesc, a.newAutoRun, a.createError = "", "", false, ""
						a.createOpen = true
					}
				}
			}
			a.pageHeader(c, t, msgs.WorkspacesTitle(), msgs.WorkspacesSubtitle(), action)

			if len(a.workspaces) == 0 {
				a.emptyWorkspaces(c, t, msgs)
				return
			}

			ui.Grid(c).Columns(3).Gap(16).Children(func() {
				for _, workspace := range a.workspaces {
					a.workspaceCard(c, t, msgs, workspace)
				}
			})
		})
	})
}

func (a *app) emptyWorkspaces(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Column(c).FillWidth().Padding(50, 38).Gap(12).Center().Radius(16).
		Background(t.Surface).Border(1, t.Border).BorderStyle(ui.BorderDashed).Children(func() {
		a.iconTile(c, t, iconGrid, 46, 20)
		ui.Text(c, msgs.WorkspacesEmpty()).FontSize(16).Bold()
		ui.Text(c, msgs.WorkspacesEmptyHint()).FontSize(12.5).TextColor(t.TextMuted)
		if ui.PrimaryButton(c, msgs.CreateWorkspace()).Clicked() {
			a.newName, a.newDesc, a.newAutoRun, a.createError = "", "", false, ""
			a.createOpen = true
		}
	})
}

func (a *app) workspaceCard(c *ui.Context, t *ui.Theme, msgs locales.Messages, workspace *data.Workspace) {
	card := ui.Column(c).MinHeight(158).Padding(18).Gap(11).Radius(14).
		Background(t.Surface).Border(1, t.Border).
		Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.05))
	if card.Hovered() {
		card.Border(1, t.Accent.Alpha(0.35))
		card.Background(t.SurfaceHover.Alpha(0.45))
	}
	card.Children(func() {
		ui.Row(c).Gap(11).AlignItems(ui.Center).Children(func() {
			a.iconTile(c, t, iconGrid, 38, 17)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, workspace.Name).FontSize(15.5).Bold().SingleLine()
				services := a.workspaceCounts[workspace.ID]
				ui.Textf(c, "%s · %s", msgs.ServicesCount(services), msgs.ClientsCount(len(workspace.ManagedClients))).
					FontSize(10.5).TextColor(t.TextMuted).SingleLine()
			})
			if ui.Button(c, "").Label(msgs.Delete()).Tooltip(msgs.Delete()).Padding(4, 6).Radius(7).
				Children(func() { ui.Icon(c, iconTrash).Size(12, 12).TextColor(t.TextMuted) }).Clicked() {
				a.deleteOpen = true
				a.current = workspace
			}
		})

		description := workspace.Description
		if description == "" {
			description = msgs.WorkspacesSubtitle()
		}
		ui.Text(c, description).FontSize(12.5).TextColor(t.TextMuted).MaxLines(2).MinHeight(34)

		ui.Spacer(c)

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			clients := managedClients(workspace)
			for i, client := range clients {
				if i >= 4 {
					ui.Textf(c, "+%d", len(clients)-4).FontSize(10).TextColor(t.TextMuted)
					break
				}
				clientLogo(c, client, 16, t.TextMuted)
			}
			switch {
			case workspace.Disabled:
				ui.Box(c).Padding(2, 7).Radius(999).Background(t.TextMuted.Alpha(0.12)).Children(func() {
					ui.Text(c, msgs.WorkspaceDisabledBadge()).FontSize(9.5).TextColor(t.TextMuted)
				})
			case workspace.AutoRun:
				ui.Box(c).Padding(2, 7).Radius(999).Background(t.Success.Alpha(0.12)).Children(func() {
					ui.Text(c, msgs.AutoStartShort()).FontSize(9.5).TextColor(t.Success)
				})
			}
			ui.Spacer(c)
			if ui.PrimaryButton(c, msgs.Open()).FontSize(12).Padding(4, 12).Clicked() {
				a.current = workspace
				a.reloadInstances()
				a.router.Push("/workspaces/" + workspace.ID)
			}
		})
	})
}

func (a *app) workspaceDetail(c *ui.Context, t *ui.Theme, msgs locales.Messages, id string) {
	a.loadWorkspace(id)
	if a.current == nil {
		ui.Column(c).Fill().Center().Gap(12).Children(func() {
			ui.Text(c, msgs.NoResults()).FontSize(15).TextColor(t.TextMuted)
			if ui.Button(c, msgs.Back()).Clicked() {
				a.router.Push("/workspaces")
			}
		})
		return
	}

	workspace := a.current
	ui.Column(c).Fill().Padding(30, 28, 22, 28).Gap(18).Children(func() {
		ui.Column(c).Gap(7).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "").Label(msgs.Back()).Tooltip(msgs.Back()).Padding(5, 7).Radius(8).
					Children(func() { ui.Icon(c, iconBack).Size(13, 13) }).Clicked() {
					a.router.Push("/workspaces")
				}
				ui.Text(c, workspace.Name).FontSize(24).Bold().SingleLine()
				ui.Spacer(c)
				a.managedChips(c, t, msgs, workspace)
			})
			if workspace.Description != "" {
				ui.Text(c, workspace.Description).FontSize(12.5).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(22).AlignItems(ui.Center).Children(func() {
				enabled := !workspace.Disabled
				if ui.Switch(c, &enabled).Label(msgs.WorkspaceEnabled()).Changed() {
					a.setWorkspaceEnabled(workspace, enabled, c, msgs)
				}
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, msgs.WorkspaceEnabled()).FontSize(11.5)
					hint := msgs.WorkspaceEnabledHint()
					if workspace.Disabled {
						hint = msgs.WorkspaceDisabledHint()
					}
					ui.Text(c, hint).FontSize(10.5).TextColor(t.TextMuted)
				})
				auto := workspace.AutoRun
				if ui.Switch(c, &auto).Label(msgs.AutoStart()).Disabled(workspace.Disabled).Changed() {
					a.setAutoStart(workspace, auto, c, msgs)
				}
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, msgs.AutoStart()).FontSize(11.5)
					ui.Text(c, msgs.AutoStartHint()).FontSize(10.5).TextColor(t.TextMuted)
				})
			})
		})

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.PrimaryButton(c, msgs.AddServer()).Clicked() {
				a.openAddDialog()
			}
			if ui.Button(c, msgs.ClientConfig()).Clicked() {
				a.configOpen = true
			}
			ui.Spacer(c)
			if ui.Button(c, msgs.StartAll()).TextColor(t.Success).Disabled(workspace.Disabled).Clicked() {
				id := workspace.ID
				go func() {
					if err := manager.StartWorkspace(id); err != nil {
						log.Println("start workspace:", err)
					}
					if a.win != nil {
						a.win.Update(a.reloadInstances)
					}
				}()
			}
			if ui.Button(c, msgs.StopAll()).TextColor(t.Warning).Clicked() {
				for _, instance := range a.instances {
					_ = manager.StopServerInstance(instance.ID)
				}
				a.reloadInstances()
			}
		})

		if len(a.instances) == 0 {
			ui.Column(c).Grow(1).Center().Gap(10).Radius(14).
				Background(t.Surface).Border(1, t.Border).BorderStyle(ui.BorderDashed).Children(func() {
				a.iconTile(c, t, iconServer, 44, 20)
				ui.Text(c, msgs.NoServers()).FontSize(15).Bold()
				ui.Text(c, msgs.NoServersHint()).FontSize(12).TextColor(t.TextMuted)
				if ui.PrimaryButton(c, msgs.AddServer()).Clicked() {
					a.openAddDialog()
				}
			})
			return
		}

		card := ui.Column(c).Grow(1).Radius(14).Background(t.Surface).
			Border(1, t.Border).Clip()
		card.Children(func() {
			columns := []ui.TableColumn{
				{Title: msgs.ServiceName()},
				{Title: msgs.ServiceType(), Width: 130},
				{Title: msgs.ServiceStatus(), Width: 130},
				{Title: msgs.ServiceActions(), Width: 190, Fixed: true},
			}
			instances := a.instances
			ui.Table(c, nil, columns, len(instances), func(row, col int) {
				instance := instances[row]
				switch col {
				case 0:
					name := instance.ID
					if instance.Config != nil && instance.Config.Name != "" {
						name = instance.Config.Name
					}
					ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(2, 10).Children(func() {
						ui.Box(c).Size(24, 24).Radius(7).Background(t.Accent.Alpha(0.11)).
							Center().Children(func() {
							ui.Icon(c, iconServer).Size(12, 12).TextColor(t.Accent)
						})
						ui.Text(c, name).FontSize(12.5).Bold().SingleLine()
					})
				case 1:
					kind := ""
					if instance.Config != nil {
						kind = instance.Config.Type
						if kind == "" {
							kind = instance.Config.Transport
						}
					}
					ui.Text(c, kind).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
				case 2:
					status := msgs.StatusStopped()
					color := t.TextMuted
					switch instance.Status {
					case "started":
						status, color = msgs.StatusStarted(), t.Success
					case "error":
						status, color = msgs.StatusError(), t.Danger
					}
					a.statusBadge(c, t, status, color)
				case 3:
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						if ui.Button(c, "").Label(msgs.Start()).Tooltip(msgs.Start()).Padding(4, 8).
							Radius(7).Disabled(workspace.Disabled || instance.Status == "started").
							Children(func() { ui.Icon(c, iconPlay).Size(12, 12) }).Clicked() {
							if err := a.startInstance(instance.ID); err != nil {
								c.Toast(err.Error())
							} else {
								c.Toast(msgs.ServerStarted())
							}
						}
						if ui.Button(c, "").Label(msgs.Stop()).Tooltip(msgs.Stop()).Padding(4, 8).
							Radius(7).Disabled(instance.Status != "started").
							Children(func() { ui.Icon(c, iconStop).Size(12, 12) }).Clicked() {
							if err := a.stopInstance(instance.ID); err != nil {
								c.Toast(err.Error())
							} else {
								c.Toast(msgs.ServerStopped())
							}
						}
						if ui.Button(c, "").Label(msgs.Connect()).Tooltip(msgs.Connect()).Padding(4, 8).
							Radius(7).Children(func() { ui.Icon(c, iconLink).Size(12, 12) }).Clicked() {
							a.connectInstance = instance
							a.connectOpen = true
						}
						if ui.Button(c, "").Label(msgs.Delete()).Tooltip(msgs.Delete()).Padding(4, 8).
							Radius(7).Disabled(instance.Status == "started").
							Children(func() { ui.Icon(c, iconTrash).Size(12, 12).TextColor(t.Danger) }).Clicked() {
							if err := data.DeleteServerConfig(instance.ID); err != nil {
								c.Toast(err.Error())
							} else {
								a.reloadInstances()
								a.loadWorkspaces()
								if err := a.syncManagedClients(); err != nil {
									c.Toast(err.Error())
								} else {
									c.Toast(msgs.ServerDeleted())
								}
							}
						}
					})
				}
			}).Grow(1)
		})
	})
}

// setWorkspaceEnabled enables or disables a workspace, stopping its running
// services when it is disabled.
func (a *app) setWorkspaceEnabled(workspace *data.Workspace, enabled bool, c *ui.Context, msgs locales.Messages) {
	workspace.Disabled = !enabled
	if err := data.SaveWorkspace(workspace); err != nil {
		c.Toast(err.Error())
		return
	}
	if !enabled {
		if err := manager.StopWorkspace(workspace.ID); err != nil {
			log.Println("stop workspace:", err)
		}
		a.reloadInstances()
	}
	a.loadWorkspaces()
	if enabled {
		c.Toast(msgs.WorkspaceEnabledOn())
	} else {
		c.Toast(msgs.WorkspaceEnabledOff())
	}
}

// setAutoStart remembers whether the workspace starts with MyMCP.
func (a *app) setAutoStart(workspace *data.Workspace, enabled bool, c *ui.Context, msgs locales.Messages) {
	workspace.AutoRun = enabled
	if enabled {
		workspace.Enabled = true
	}
	if err := data.SaveWorkspace(workspace); err != nil {
		c.Toast(err.Error())
		return
	}
	a.loadWorkspaces()
	if enabled {
		c.Toast(msgs.AutoStartOn())
	} else {
		c.Toast(msgs.AutoStartOff())
	}
}

func (a *app) managedChips(c *ui.Context, t *ui.Theme, msgs locales.Messages, workspace *data.Workspace) {
	if len(workspace.ManagedClients) == 0 {
		return
	}
	clients := managedClients(workspace)
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, client := range clients {
			client := client
			ui.Row(c).Padding(3, 8).Gap(5).Radius(999).Background(t.Accent.Alpha(0.11)).
				AlignItems(ui.Center).Children(func() {
				clientLogo(c, client, 13, t.Accent)
				ui.Text(c, client.Label).FontSize(10).TextColor(t.Accent).SingleLine()
			})
		}
		if ui.Button(c, "").Label(msgs.SyncConfig()).Tooltip(msgs.SyncConfig()).Padding(2, 6).
			Radius(6).Children(func() { ui.Icon(c, iconSync).Size(12, 12) }).Clicked() {
			if err := a.syncManagedClients(); err != nil {
				c.Toast(err.Error())
			} else {
				c.Toast(msgs.SyncSuccess())
			}
		}
	})
}

func (a *app) discoverPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Column(c).Fill().Padding(30, 28, 20, 28).Gap(18).Children(func() {
		ui.Column(c).Gap(5).Children(func() {
			ui.Text(c, msgs.CatalogTitle()).FontSize(24).Bold()
			ui.Text(c, msgs.CatalogSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
		})

		results := a.catalog.Filter(a.query, a.filterCategory(msgs))
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &a.query).Placeholder(msgs.SearchPlaceholder()).
				Label(msgs.SearchPlaceholder()).Width(340)
			a.categoryMenu(c, t, msgs)
			ui.Spacer(c)
			ui.Text(c, msgs.ResultsCount(len(results))).FontSize(11.5).TextColor(t.TextMuted)
		})

		a.discoverList.Key = func(row int) any { return results[row].Name }
		a.discoverList.Label = func(row int) string { return results[row].DisplayName }
		ui.List(c, &a.discoverList, len(results), func(row int) {
			a.serverRow(c, t, msgs, results[row])
		}).Grow(1).Children(func() {
			if len(results) == 0 {
				ui.Column(c).Padding(50).Center().Gap(8).Children(func() {
					a.iconTile(c, t, iconSearch, 44, 20)
					ui.Text(c, msgs.NoResults()).FontSize(15).Bold()
					ui.Text(c, msgs.NoResultsHint()).FontSize(12).TextColor(t.TextMuted)
				})
			}
		})
	})
}

func (a *app) serverRow(c *ui.Context, t *ui.Theme, msgs locales.Messages, server *CatalogServer) {
	row := ui.Row(c).Padding(11, 12).Gap(13).AlignItems(ui.Center).Radius(10)
	if row.Hovered() {
		row.Background(t.SurfaceHover.Alpha(0.5))
	}
	row.Children(func() {
		a.iconTile(c, t, iconServer, 36, 17)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, server.DisplayName).FontSize(14).Bold().SingleLine()
			ui.Text(c, plainText(server.Description)).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
			if len(server.Tags) > 1 {
				a.tagRow(c, t, server.Tags)
			}
		})
		ui.Row(c).Gap(6).Shrink(0).Children(func() {
			if ui.Button(c, msgs.ViewDetails()).FontSize(11.5).Padding(4, 10).Clicked() {
				a.detail = server.Name
				a.router.Push("/catalog/" + server.Name)
			}
			if ui.PrimaryButton(c, msgs.Install()).FontSize(11.5).Padding(4, 10).Clicked() {
				a.openInstall(server.Name)
			}
		})
	})
	ui.Divider(c).MarginX(12)
}

func (a *app) detailPage(c *ui.Context, t *ui.Theme, msgs locales.Messages, name string) {
	server := a.catalog.Server(name)
	if server == nil {
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			ui.Text(c, msgs.NoResults()).FontSize(15).TextColor(t.TextMuted)
			if ui.Button(c, msgs.Back()).Clicked() {
				a.router.Push("/catalog")
			}
		})
		return
	}
	a.detail = name

	ui.Scroll(c).Grow(1).Padding(30).Children(func() {
		ui.Column(c).MaxWidth(1000).Gap(18).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "").Label(msgs.Back()).Tooltip(msgs.Back()).Padding(5, 7).Radius(8).
					Children(func() { ui.Icon(c, iconBack).Size(13, 13) }).Clicked() {
					a.router.Push("/catalog")
				}
				ui.Text(c, msgs.Back()).FontSize(12.5).TextColor(t.TextMuted)
			})

			hero := ui.Row(c).Padding(20).Gap(16).Radius(14).
				Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center)
			hero.Children(func() {
				a.iconTile(c, t, iconServer, 52, 24)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, server.DisplayName).FontSize(23).Bold().SingleLine()
					ui.Text(c, plainText(server.Description)).FontSize(12.5).TextColor(t.TextMuted).MaxLines(3)
					if len(server.Categories) > 0 {
						ui.Row(c).Gap(5).Wrap().Children(func() {
							for _, category := range server.Categories {
								a.pill(c, t, category, t.Accent)
							}
						})
					}
				})
				if ui.PrimaryButton(c, msgs.Install()).Clicked() {
					a.openInstall(server.Name)
				}
			})

			card := ui.Column(c).Padding(20).Gap(10).Radius(14).
				Background(t.Surface).Border(1, t.Border)
			card.Children(func() {
				ui.Text(c, msgs.Details()).FontSize(14).Bold()
				ui.Grid(c).ColumnTracks(ui.FitContent(), ui.Fr(1)).GapX(28).GapY(8).Children(func() {
					a.infoCell(c, t, msgs.DetailName(), server.Name)
					a.infoCell(c, t, msgs.DetailAuthor(), server.Author.Name)
					a.infoCell(c, t, msgs.DetailLicense(), server.License)
					if server.Repository.URL != "" {
						a.linkCell(c, t, msgs.DetailRepository(), server.Repository.URL)
					}
					if server.Homepage != "" {
						a.linkCell(c, t, msgs.DetailHomepage(), server.Homepage)
					}
				})
			})

			if len(server.Tags) > 0 {
				ui.Column(c).Gap(7).Children(func() {
					ui.Text(c, msgs.DetailTags()).FontSize(14).Bold()
					a.tagRow(c, t, server.Tags)
				})
			}

			types := server.InstallTypes()
			if len(types) > 0 {
				card := ui.Column(c).Padding(20).Gap(9).Radius(14).
					Background(t.Surface).Border(1, t.Border)
				card.Children(func() {
					ui.Text(c, msgs.DetailInstallation()).FontSize(14).Bold()
					for _, kind := range types {
						command := server.InstallCommand(kind)
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							a.pill(c, t, strings.ToUpper(kind), t.TextMuted)
							a.codeBox(c, t, command)
							if ui.Button(c, "").Label(msgs.Copy()).Tooltip(msgs.Copy()).Padding(3, 6).
								Radius(6).Children(func() { ui.Icon(c, iconCopy).Size(12, 12) }).Clicked() {
								mygo.Clipboard.WriteText(command)
								c.Toast(msgs.Copied())
							}
						})
					}
				})
			}

			if len(server.Examples) > 0 {
				ui.Column(c).Gap(8).Children(func() {
					ui.Text(c, msgs.DetailExamples()).FontSize(14).Bold()
					ui.Grid(c).Columns(2).Gap(12).Children(func() {
						for _, example := range server.Examples {
							ui.Column(c).Padding(15).Gap(6).Radius(12).
								Background(t.Surface).Border(1, t.Border).Children(func() {
								ui.Text(c, example.Title).FontSize(13).Bold()
								if example.Description != "" {
									ui.Text(c, plainText(example.Description)).FontSize(11.5).TextColor(t.TextMuted).MaxLines(3)
								}
								if example.Prompt != "" {
									a.codeBox(c, t, example.Prompt)
								}
							})
						}
					})
				})
			}

			if len(server.Tools) > 0 {
				ui.Column(c).Gap(8).Children(func() {
					ui.Text(c, msgs.DetailTools()).FontSize(14).Bold()
					ui.Grid(c).Columns(2).Gap(12).Children(func() {
						for _, tool := range server.Tools {
							ui.Column(c).Padding(15).Gap(5).Radius(12).
								Background(t.Surface).Border(1, t.Border).Children(func() {
								ui.Text(c, tool.Name).FontSize(12.5).Bold()
								if tool.Description != "" {
									ui.Text(c, plainText(tool.Description)).FontSize(11.5).TextColor(t.TextMuted).MaxLines(3)
								}
								if len(tool.InputSchema) > 0 {
									var pretty map[string]any
									if json.Unmarshal(tool.InputSchema, &pretty) == nil {
										if raw, err := json.MarshalIndent(pretty, "", "  "); err == nil {
											a.codeBox(c, t, string(raw))
										}
									}
								}
							})
						}
					})
				})
			}
		})
	})
}

func (a *app) settingsPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	a.initSettingsChoices(msgs)
	ui.Scroll(c).Grow(1).Padding(30).Children(func() {
		ui.Column(c).Gap(20).MaxWidth(760).Children(func() {
			a.pageHeader(c, t, msgs.SettingsTitle(), msgs.SettingsSubtitle(), nil)

			appearance := ui.Column(c).Padding(20).Gap(14).Radius(14).
				Background(t.Surface).Border(1, t.Border)
			appearance.Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, msgs.Appearance()).FontSize(14).Bold()
					ui.Text(c, msgs.AppearanceHint()).FontSize(11.5).TextColor(t.TextMuted)
				})
				ui.Column(c).Padding(14).Gap(4).Radius(10).
					Background(t.Background.Alpha(0.45)).Children(func() {
					ui.Form(c, func() {
						ui.Field(c, msgs.Language(), func() {
							if ui.Select(c, &a.languageChoice, []string{
								msgs.FollowSystem(), msgs.Chinese(), msgs.English(),
							}).Width(260).Changed() {
								a.applyLanguageChoice(msgs)
							}
						})
						ui.Field(c, msgs.Theme(), func() {
							if ui.Select(c, &a.themeChoice, []string{
								msgs.FollowSystem(), msgs.Light(), msgs.Dark(),
							}).Width(260).Changed() {
								a.settings.Theme = a.themeTag(a.themeChoice, msgs)
								a.queueTheme()
								if err := data.SaveSettings(&a.settings); err != nil {
									c.Toast(err.Error())
								}
							}
						}).Description(msgs.SettingsThemeHint())
					})
				})
			})

			startup := ui.Column(c).Padding(20).Gap(14).Radius(14).
				Background(t.Surface).Border(1, t.Border)
			startup.Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, msgs.Startup()).FontSize(14).Bold()
					ui.Text(c, msgs.StartupHint()).FontSize(11.5).TextColor(t.TextMuted)
				})
				ui.Column(c).Padding(14).Gap(11).Radius(10).
					Background(t.Background.Alpha(0.45)).Children(func() {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, msgs.LaunchAtLogin()).FontSize(12.5).Bold()
							ui.Text(c, msgs.LaunchAtLoginHint()).FontSize(11).TextColor(t.TextMuted)
						})
						launch := a.settings.LaunchAtLogin
						if ui.Switch(c, &launch).Label(msgs.LaunchAtLogin()).Changed() {
							if err := a.setLaunchAtLogin(launch); err != nil {
								c.Toast(msgs.LaunchAtLoginFailed(err.Error()))
							} else if launch {
								c.Toast(msgs.LaunchAtLoginOn())
							} else {
								c.Toast(msgs.LaunchAtLoginOff())
							}
						}
					})
					ui.Divider(c)
					ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconLogo).Size(12, 12).TextColor(t.TextMuted)
						ui.Text(c, msgs.BackgroundHint()).FontSize(11).TextColor(t.TextMuted)
					})
				})
			})

			gateway := ui.Column(c).Padding(20).Gap(14).Radius(14).
				Background(t.Surface).Border(1, t.Border)
			gateway.Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, msgs.NavGateway()).FontSize(14).Bold()
					ui.Text(c, msgs.GatewayAddressHint()).FontSize(11.5).TextColor(t.TextMuted)
				})
				ui.Column(c).Padding(14).Gap(10).Radius(10).
					Background(t.Background.Alpha(0.45)).Children(func() {
					ui.Form(c, func() {
						ui.Field(c, msgs.GatewayAddress(), func() {
							ui.TextInput(c, &a.settings.GatewayAddr).Width(260).Placeholder("127.0.0.1:8422")
						}).Description(msgs.GatewayAddressHint())
					})
				})
				if a.settingsError != "" {
					ui.Text(c, a.settingsError).FontSize(12).TextColor(t.Danger)
				}
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Spacer(c)
					if ui.PrimaryButton(c, msgs.SaveSettings()).Clicked() {
						a.saveSettings(c, msgs)
					}
				})
			})

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "%s %s", msgs.AppTitle(), appVersion()).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, "·").TextColor(t.TextMuted).Opacity(0.5)
				ui.Text(c, "MyGo").FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func (a *app) saveSettings(c *ui.Context, msgs locales.Messages) {
	language := a.settings.Language
	if a.languageChoice != "" {
		language = a.languageTag(a.languageChoice, msgs)
	}
	theme := a.settings.Theme
	if a.themeChoice != "" {
		theme = a.themeTag(a.themeChoice, msgs)
	}
	gatewayAddr, err := normalizeGatewayAddr(a.settings.GatewayAddr)
	if err != nil {
		a.settingsError = msgs.GatewayAddressInvalid()
		return
	}
	if gatewayAddr != a.gwAddr && a.syncManager != nil {
		for _, status := range a.syncManager.Statuses() {
			if status.Managed {
				a.settingsError = msgs.GatewayAddressManaged()
				return
			}
		}
	}
	a.settingsError = ""
	oldGatewayAddr := a.gwAddr
	a.settings.Language = language
	a.settings.Theme = theme
	a.settings.GatewayAddr = gatewayAddr
	if err := data.SaveSettings(&a.settings); err != nil {
		c.Toast(err.Error())
		return
	}
	a.queueTheme()
	a.queueLocale()
	a.languageChoice, a.themeChoice = "", ""
	if a.settings.GatewayAddr != oldGatewayAddr {
		wasRunning := a.gwRunning
		hadError := a.gwError != ""
		if wasRunning {
			a.stopGateway()
		}
		a.gwAddr = a.settings.GatewayAddr
		if wasRunning {
			a.restartGateway(msgs)
		} else if hadError {
			a.startGateway(msgs)
		}
	}
	c.Toast(msgs.SettingsSaved())
}

// applyLanguageChoice switches the UI language as soon as the select
// changes, then records it. The Save button still covers the gateway address.
func (a *app) applyLanguageChoice(msgs locales.Messages) {
	language := a.languageTag(a.languageChoice, msgs)
	if language == a.settings.Language {
		return
	}
	a.settings.Language = language
	if err := data.SaveSettings(&a.settings); err != nil {
		a.settingsError = err.Error()
		return
	}
	a.settingsError = ""
	a.queueLocale()
	a.languageChoice = ""
}

func (a *app) initSettingsChoices(msgs locales.Messages) {
	if a.languageChoice == "" {
		a.languageChoice = a.languageLabel(a.settings.Language, msgs)
	}
	if a.themeChoice == "" {
		a.themeChoice = a.themeLabel(a.settings.Theme, msgs)
	}
}

func (a *app) languageLabel(tag string, msgs locales.Messages) string {
	switch tag {
	case "zh":
		return msgs.Chinese()
	case "en":
		return msgs.English()
	default:
		return msgs.FollowSystem()
	}
}

func (a *app) languageTag(label string, msgs locales.Messages) string {
	switch label {
	case msgs.Chinese():
		return "zh"
	case msgs.English():
		return "en"
	default:
		return "system"
	}
}

func (a *app) themeLabel(theme string, msgs locales.Messages) string {
	switch theme {
	case "light":
		return msgs.Light()
	case "dark":
		return msgs.Dark()
	default:
		return msgs.FollowSystem()
	}
}

func (a *app) themeTag(label string, msgs locales.Messages) string {
	switch label {
	case msgs.Light():
		return "light"
	case msgs.Dark():
		return "dark"
	default:
		return "system"
	}
}

func (a *app) filterCategory(msgs locales.Messages) string {
	if a.category == msgs.AllCategories() {
		return ""
	}
	return a.category
}

func (a *app) categoryMenu(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	label := a.category
	if label == "" {
		label = msgs.AllCategories()
	}
	ui.MenuButton(c, label, func(m *ui.Menu) {
		if m.Item(msgs.AllCategories()).Radio(true).Checked(a.category == "").Chosen() {
			a.category = ""
		}
		for _, category := range a.catalog.Categories() {
			category := category
			if m.Item(category).Radio(true).Checked(a.category == category).Chosen() {
				a.category = category
			}
		}
	}).Label(msgs.SelectCategory()).Height(32).Padding(0, 12).Radius(8).
		Border(1, t.Border).FontSize(12)
}

func (a *app) pageHeader(c *ui.Context, t *ui.Theme, title, subtitle string, action func()) {
	ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(24).Bold()
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(12.5).TextColor(t.TextMuted)
			}
		})
		ui.Spacer(c)
		if action != nil {
			action()
		}
	})
}

func (a *app) tagRow(c *ui.Context, t *ui.Theme, tags []string) {
	ui.Row(c).Gap(5).Wrap().Children(func() {
		for i, tag := range tags {
			if i >= 6 {
				break
			}
			a.pill(c, t, tag, t.TextMuted)
		}
	})
}

func (a *app) pill(c *ui.Context, t *ui.Theme, text string, color ui.Color) {
	ui.Box(c).Padding(2, 8).Radius(999).Background(color.Alpha(0.12)).Children(func() {
		ui.Text(c, text).FontSize(10.5).TextColor(color).SingleLine()
	})
}

func (a *app) statusBadge(c *ui.Context, t *ui.Theme, text string, color ui.Color) {
	ui.Row(c).Padding(3, 9).Radius(999).Background(color.Alpha(0.12)).
		AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(color)
		ui.Text(c, text).FontSize(11).TextColor(color)
	})
}

func (a *app) iconTile(c *ui.Context, t *ui.Theme, icon *ui.SVG, size, iconSize float32) {
	ui.Box(c).Size(size, size).Radius(size * 0.28).Background(t.Accent.Alpha(0.12)).
		Center().Children(func() {
		ui.Icon(c, icon).Size(iconSize, iconSize).TextColor(t.Accent)
	})
}

// managedClients lists a workspace's clients in the catalog's order.
func managedClients(workspace *data.Workspace) []*clientDef {
	clients := make([]*clientDef, 0, len(workspace.ManagedClients))
	for _, client := range supportedClients {
		if _, ok := workspace.ManagedClients[client.Name]; ok {
			clients = append(clients, client)
		}
	}
	return clients
}

// clientTabs draws the client picker with each vendor's official icon.
func clientTabs(c *ui.Context, t *ui.Theme, selected *int) {
	tabs := ui.TabsBase(c, selected, len(supportedClients))
	tabs.List.Gap(3).Wrap().Children(func() {
		for i, client := range supportedClients {
			client := client
			tab := tabs.Tab(i).Padding(5, 9).Radius(8)
			if i == *selected {
				tab.Background(t.Accent.Alpha(0.12))
			}
			tab.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					color := t.TextMuted
					if i == *selected {
						color = t.Accent
					}
					clientLogo(c, client, 14, color)
					ui.Text(c, client.Label).FontSize(11.5).TextColor(color).SingleLine()
				})
			})
		}
	})
}

// clientLogo draws a managed client's official icon at size DIPs.
func clientLogo(c *ui.Context, client *clientDef, size float32, color ui.Color) {
	clientIcon(c, client.Name, size, color)
}

func clientIcon(c *ui.Context, name string, size float32, color ui.Color) {
	if svg := clientSVGs[name]; svg != nil {
		ui.Image(c, svg).Size(size, size).Fit(ui.Contain).TextColor(color)
		return
	}
	if bitmap := clientBitmaps[name]; bitmap != nil {
		ui.Image(c, bitmap).Size(size, size).Fit(ui.Contain).Radius(size * 0.22)
	}
}

func tableHeight(rows int) float32 {
	height := 42 + float32(rows)*40
	if height < 160 {
		return 160
	}
	if height > 460 {
		return 460
	}
	return height
}

func (a *app) codeBox(c *ui.Context, t *ui.Theme, code string) {
	ui.Box(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(8).
		Background(t.Background.Alpha(0.6)).Border(1, t.Border).Children(func() {
		ui.Text(c, code).FontSize(11.5).TextColor(t.Text).Wrap()
	})
}

func (a *app) infoCell(c *ui.Context, t *ui.Theme, label, value string) {
	if value == "" {
		return
	}
	ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
	ui.Text(c, value).FontSize(12.5).SingleLine()
}

func (a *app) linkCell(c *ui.Context, t *ui.Theme, label, link string) {
	ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
	ui.Link(c, link, link).FontSize(12.5).SingleLine()
}

var (
	markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	htmlTag      = regexp.MustCompile(`<[^>]*>`)
)

func plainText(markdown string) string {
	text := markdownLink.ReplaceAllString(markdown, "$1")
	text = htmlTag.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "`", "")
	return strings.Join(strings.Fields(text), " ")
}

func modelServerConfig(workspaceID string, server *CatalogServer, kind string, params map[string]string) model.ServerConfig {
	clean := map[string]string{}
	for key, value := range params {
		if strings.TrimSpace(value) != "" {
			clean[key] = value
		}
	}
	return model.ServerConfig{
		Workspace: workspaceID,
		Name:      server.Name,
		Type:      kind,
		Transport: "stdio",
		Cmd:       server.InstallCommand(kind),
		Env:       []string{},
		Params:    clean,
	}
}

var errUnknownServer = fmt.Errorf("unknown service")
