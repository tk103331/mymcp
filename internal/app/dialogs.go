package app

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/manager"
	"github.com/tk103331/mymcp/internal/store"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) dialogs(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	a.createWorkspaceDialog(c, t, msgs)
	a.deleteWorkspaceDialog(c, t, msgs)
	a.addServiceDialog(c, t, msgs)
	a.clientConfigDialog(c, t, msgs)
	a.connectDialog(c, t, msgs)
	a.installDialog(c, t, msgs)
	a.gatewayDialogs(c, t, msgs)
	a.clientPlanDialog(c, t, msgs)
	a.detachDialog(c, t, msgs)
	a.secretDialogs(c, t, msgs)
	a.pluginDialogs(c, t, msgs)
}

func (a *app) createWorkspaceDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.createOpen, func() {
		ui.Column(c).Width(440).Gap(12).Children(func() {
			ui.Text(c, msgs.CreateWorkspace()).FontSize(18).Bold()
			ui.Field(c, msgs.Name(), func() {
				ui.TextInput(c, &a.newName).AutoFocus().Placeholder(msgs.NamePlaceholder()).
					Invalid(a.createError != "" && a.newName == "")
			})
			ui.Field(c, msgs.Description(), func() {
				ui.TextArea(c, &a.newDesc).Height(80).Placeholder(msgs.DescriptionPlaceholder())
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Switch(c, &a.newAutoRun).Label(msgs.AutoStart())
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, msgs.AutoStart()).FontSize(12)
					ui.Text(c, msgs.AutoStartHint()).FontSize(10.5).TextColor(t.TextMuted)
				})
			})
			if a.createError != "" {
				ui.Text(c, a.createError).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.createOpen = false
				}
				if ui.PrimaryButton(c, msgs.Create()).Clicked() {
					switch {
					case strings.TrimSpace(a.newName) == "":
						a.createError = msgs.RequiredName()
					case strings.TrimSpace(a.newDesc) == "":
						a.createError = msgs.RequiredDescription()
					default:
						name := strings.TrimSpace(a.newName)
						for _, existing := range a.workspaces {
							if existing.Name == name {
								a.createError = msgs.WorkspaceExists(name)
								return
							}
						}
						workspace := &data.Workspace{
							Name:        name,
							Description: strings.TrimSpace(a.newDesc),
							Enabled:     true,
							AutoRun:     a.newAutoRun,
						}
						if err := data.SaveWorkspace(workspace); err != nil {
							a.createError = err.Error()
							return
						}
						a.loadWorkspaces()
						a.createOpen = false
						a.newName, a.newDesc, a.newAutoRun, a.createError = "", "", false, ""
						c.Toast(msgs.WorkspaceCreated())
					}
				}
			})
		})
	})
}

func (a *app) deleteWorkspaceDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.deleteOpen, func() {
		ui.Column(c).Width(420).Gap(12).Children(func() {
			ui.Text(c, msgs.DeleteWorkspaceTitle()).FontSize(17).Bold()
			if a.current != nil {
				ui.Text(c, msgs.DeleteWorkspaceMessage(a.current.Name)).FontSize(12.5).TextColor(t.TextMuted)
			}
			if a.current != nil && len(a.current.ManagedClients) > 0 {
				ui.Text(c, msgs.WorkspaceManagedClients()).FontSize(11.5).TextColor(t.Warning)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.deleteOpen = false
				}
				if ui.PrimaryButton(c, msgs.DeleteWorkspaceConfirm()).
					Disabled(a.current != nil && len(a.current.ManagedClients) > 0).Clicked() {
					if a.current != nil {
						if err := manager.StopWorkspace(a.current.ID); err != nil {
							log.Println("stop workspace:", err)
						}
						if err := data.DeleteWorkspace(a.current.ID); err != nil {
							c.Toast(err.Error())
						} else {
							a.current = nil
							a.instances = nil
							a.loadWorkspaces()
							if strings.HasPrefix(a.router.Path(), "/workspaces/") {
								a.router.Push("/workspaces")
							}
							c.Toast(msgs.WorkspaceDeleted())
						}
					}
					a.deleteOpen = false
				}
			})
		})
	})
}

func (a *app) openAddDialog() {
	a.addOpen = true
	a.addStep = 0
	a.addQuery = ""
	a.addCategory = ""
	a.addSelected = ""
	a.addType = ""
	a.addParams = nil
	a.addError = ""
	a.addList = ui.ListState{}
}

func (a *app) addServiceDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.addOpen, func() {
		ui.Column(c).Width(760).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, msgs.AddServer()).FontSize(18).Bold()
				ui.Spacer(c)
				ui.Textf(c, "%d / 2", a.addStep+1).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Tabs(c, &a.addStep, msgs.StepSearch(), msgs.StepConfig())

			switch a.addStep {
			case 0:
				a.addSearchStep(c, t, msgs)
			case 1:
				a.addConfigStep(c, t, msgs)
			}

			if a.addError != "" {
				ui.Text(c, a.addError).FontSize(12).TextColor(t.Danger)
			}

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.addOpen = false
				}
				if a.addStep == 1 {
					if ui.Button(c, msgs.Previous()).Clicked() {
						a.addStep = 0
						a.addError = ""
					}
				}
				if a.addStep == 0 {
					if ui.PrimaryButton(c, msgs.Next()).Disabled(a.addSelected == "").Clicked() {
						a.addStep = 1
						a.addError = ""
						a.prepareParams()
					}
				} else {
					server := a.catalog.Server(a.addSelected)
					ready := server != nil && len(server.InstallTypes()) > 0
					if ui.PrimaryButton(c, msgs.Add()).Disabled(!ready).Clicked() {
						a.confirmAdd(c, msgs)
					}
				}
			})
		})
	})
}

func (a *app) addSearchStep(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	results := a.catalog.Filter(a.addQuery, a.addFilterCategory(msgs))
	ui.Row(c).Gap(10).Children(func() {
		ui.SearchField(c, &a.addQuery).Placeholder(msgs.SearchPlaceholder()).Grow(1)
		a.addCategoryMenu(c, t, msgs)
	})
	a.addList.Key = func(row int) any { return results[row].Name }
	a.addList.Label = func(row int) string { return results[row].DisplayName }
	ui.List(c, &a.addList, len(results), func(row int) {
		server := results[row]
		background := ui.Transparent
		if a.addSelected == server.Name {
			background = t.Accent.Alpha(0.10)
		}
		item := ui.ButtonBase(c).Padding(8, 10).Gap(8).Radius(6).
			Background(background)
		item.Children(func() {
			ui.Radio(c, &a.addSelected, server.Name, "").Label(server.DisplayName)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, server.DisplayName).FontSize(13).Bold().SingleLine()
				ui.Text(c, plainText(server.Description)).FontSize(11.5).TextColor(t.TextMuted).MaxLines(1)
			})
		})
		if item.Clicked() {
			a.addSelected = server.Name
		}
		ui.Divider(c).MarginX(6)
	}).Height(320).Children(func() {
		if len(results) == 0 {
			ui.Text(c, msgs.NoResults()).Padding(20).TextColor(t.TextMuted)
		}
	})
}

func (a *app) addConfigStep(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	server := a.catalog.Server(a.addSelected)
	if server == nil {
		ui.Text(c, msgs.NoResults()).TextColor(t.TextMuted)
		return
	}
	types := server.InstallTypes()
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconServer).Size(20, 20).TextColor(t.Accent)
			ui.Text(c, server.DisplayName).FontSize(15).Bold()
			ui.Text(c, server.Name).FontSize(11.5).TextColor(t.TextMuted)
		})
		if len(types) == 0 {
			ui.Text(c, msgs.NoConfigSupport()).FontSize(12).TextColor(t.Warning)
			return
		}
		ui.Form(c, func() {
			ui.Field(c, msgs.InstallType(), func() {
				if ui.Select(c, &a.addType, types).Changed() {
					a.prepareParams()
				}
			})
		})
		if a.addType != "" {
			a.commandPreview(c, t, server.InstallCommand(a.addType), msgs)
			a.paramFields(c, server, &a.addParams)
		}
	})
}

func (a *app) prepareParams() {
	server := a.catalog.Server(a.addSelected)
	if server == nil {
		return
	}
	params := map[string]string{}
	for _, key := range server.ArgumentKeys() {
		if a.addParams != nil {
			params[key] = a.addParams[key]
		}
	}
	a.addParams = params
	types := server.InstallTypes()
	valid := false
	for _, kind := range types {
		if kind == a.addType {
			valid = true
		}
	}
	if !valid && len(types) > 0 {
		a.addType = types[0]
	}
}

func (a *app) confirmAdd(c *ui.Context, msgs locales.Messages) {
	server := a.catalog.Server(a.addSelected)
	if server == nil {
		a.addError = msgs.NoResults()
		return
	}
	if a.addType == "" {
		a.addError = msgs.InstallTypeRequired()
		return
	}
	if key := requiredArgument(server, a.addParams); key != "" {
		a.addError = key + ": " + msgs.GatewayRequired()
		return
	}
	if a.current == nil {
		a.addError = msgs.WorkspaceRequired()
		return
	}
	if workspaceHasServer(a.current.ID, server.Name) {
		a.addError = msgs.InstallAlready(server.DisplayName)
		return
	}
	if err := a.addServerConfig(a.current.ID, server.Name, a.addType, a.addParams); err != nil {
		if workspaceHasServer(a.current.ID, server.Name) {
			a.addOpen = false
			c.Toast(msgs.InstallFailed(err.Error()))
			return
		}
		a.addError = err.Error()
		return
	}
	a.addOpen = false
	c.Toast(msgs.ServerAdded())
}

func (a *app) addFilterCategory(msgs locales.Messages) string {
	if a.addCategory == msgs.AllCategories() {
		return ""
	}
	return a.addCategory
}

func (a *app) addCategoryMenu(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	label := a.addCategory
	if label == "" {
		label = msgs.AllCategories()
	}
	ui.MenuButton(c, label, func(m *ui.Menu) {
		if m.Item(msgs.AllCategories()).Radio(true).Checked(a.addCategory == "").Chosen() {
			a.addCategory = ""
		}
		for _, category := range a.catalog.Categories() {
			category := category
			if m.Item(category).Radio(true).Checked(a.addCategory == category).Chosen() {
				a.addCategory = category
			}
		}
	}).Height(30).Padding(0, 12).Radius(8).Border(1, t.Border).FontSize(12)
}

func (a *app) clientConfigDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.configOpen, func() {
		ui.Column(c).Width(780).Gap(14).Children(func() {
			ui.Text(c, msgs.ConfigTitle()).FontSize(18).Bold()
			if a.current == nil {
				return
			}
			clientTabs(c, t, &a.configTab)
			if a.configTab < 0 || a.configTab >= len(supportedClients) {
				a.configTab = 0
			}
			client := supportedClients[a.configTab]
			managed := hasClient(a.current, client.Name)
			path := client.ConfigPath()
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				clientLogo(c, client, 22, t.Text)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, client.Label).FontSize(12.5).Bold()
					if path == "" {
						ui.Text(c, msgs.NoConfigSupport()).FontSize(11).TextColor(t.Warning)
					} else {
						ui.Textf(c, "%s: %s", msgs.ConfigPath(), path).FontSize(11).TextColor(t.TextMuted).SingleLine()
					}
				})
				ui.Text(c, msgs.ManageConfig()).FontSize(11.5).TextColor(t.TextMuted)
				if ui.Switch(c, &managed).Label(msgs.ManageConfig()).Disabled(path == "").Changed() {
					a.setManagedClient(c, client, managed, msgs)
				}
			})

			preview := a.clientPreview(client)
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, msgs.ConfigPreview()).FontSize(13).Bold()
				ui.Spacer(c)
				if ui.Button(c, "").Label(msgs.Copy()).Tooltip(msgs.Copy()).Padding(3, 6).
					Children(func() { ui.Icon(c, iconCopy).Size(12, 12) }).Clicked() {
					mygo.Clipboard.WriteText(preview)
					c.Toast(msgs.Copied())
				}
			})
			ui.TextArea(c, &preview).ReadOnly(true).Height(300).FontSize(11.5)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.SyncConfig()).Clicked() {
					if err := a.syncManagedClients(); err != nil {
						c.Toast(err.Error())
					} else {
						c.Toast(msgs.SyncSuccess())
					}
				}
				if ui.PrimaryButton(c, msgs.Close()).Clicked() {
					a.configOpen = false
				}
			})
		})
	})
}

func hasClient(workspace *data.Workspace, name string) bool {
	if workspace.ManagedClients == nil {
		return false
	}
	_, ok := workspace.ManagedClients[name]
	return ok
}

func (a *app) setManagedClient(c *ui.Context, client *clientDef, managed bool, msgs locales.Messages) {
	if a.current == nil {
		return
	}
	if a.current.ManagedClients == nil {
		a.current.ManagedClients = map[string]data.ManagedClient{}
	}
	if managed {
		a.current.ManagedClients[client.Name] = data.ManagedClient{Config: client.ConfigPath()}
		if err := saveManagedClientConfig(client, a.instances); err != nil {
			delete(a.current.ManagedClients, client.Name)
			c.Toast(err.Error())
			return
		}
	} else {
		delete(a.current.ManagedClients, client.Name)
	}
	if err := data.SaveWorkspace(a.current); err != nil {
		c.Toast(err.Error())
		return
	}
	a.loadWorkspaces()
	c.Toast(msgs.SyncSuccess())
}

func (a *app) clientPreview(client *clientDef) string {
	return client.PreviewText(a.instances)
}

func (a *app) connectDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.connectOpen, func() {
		ui.Column(c).Width(780).Gap(14).Children(func() {
			ui.Text(c, msgs.ConnectConfig()).FontSize(18).Bold()
			instance := a.connectInstance
			if instance == nil {
				return
			}
			name := instance.ID
			if instance.Config != nil && instance.Config.Name != "" {
				name = instance.Config.Name
			}
			ui.Text(c, msgs.EndpointHint()).FontSize(12).TextColor(t.TextMuted)
			endpoint := instance.Endpoint
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if endpoint == "" {
					ui.Text(c, msgs.StatusStopped()).FontSize(12).TextColor(t.Warning)
					return
				}
				ui.TextInput(c, &endpoint).ReadOnly(true).Grow(1)
				if ui.Button(c, msgs.Copy()).Clicked() {
					mygo.Clipboard.WriteText(endpoint)
					c.Toast(msgs.Copied())
				}
			})

			clientTabs(c, t, &a.connectTab)
			if a.connectTab < 0 || a.connectTab >= len(supportedClients) {
				a.connectTab = 0
			}
			client := supportedClients[a.connectTab]
			preview := a.instancePreview(client, instance)
			ui.Row(c).AlignItems(ui.Center).Gap(7).Children(func() {
				clientLogo(c, client, 18, t.Text)
				ui.Textf(c, "%s · %s", client.Label, name).FontSize(13).Bold()
				ui.Spacer(c)
				if ui.Button(c, "").Label(msgs.Copy()).Tooltip(msgs.Copy()).Padding(3, 6).
					Children(func() { ui.Icon(c, iconCopy).Size(12, 12) }).Clicked() {
					mygo.Clipboard.WriteText(preview)
					c.Toast(msgs.Copied())
				}
			})
			ui.TextArea(c, &preview).ReadOnly(true).Height(300).FontSize(11.5)
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c, msgs.Close()).Clicked() {
					a.connectOpen = false
				}
			})
		})
	})
}

func (a *app) instancePreview(client *clientDef, instance *data.ServerInstance) string {
	return client.PreviewText([]*data.ServerInstance{instance})
}

func (a *app) openInstall(slug string) {
	server := a.catalog.Server(slug)
	if server == nil {
		return
	}
	a.installOpen = true
	a.installSlug = slug
	a.installType = ""
	a.installParms = map[string]string{}
	a.installError = ""
	a.installProfile = ""
	if a.usage != nil {
		_, _ = a.usage.EnsureDefaultProfile(context.Background())
		if profiles, err := a.usage.ListProfiles(context.Background()); err == nil {
			a.gwProfiles = profiles
		}
	}
	for _, profile := range a.gwProfiles {
		if a.installProfile == "" {
			a.installProfile = profile.Name
		}
		if profile.Name == "default" {
			a.installProfile = "default"
			break
		}
	}
	types := server.InstallTypes()
	if len(types) > 0 {
		a.installType = types[0]
	}
	for _, key := range server.ArgumentKeys() {
		a.installParms[key] = ""
	}
}

func (a *app) installDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.installOpen, func() {
		server := a.catalog.Server(a.installSlug)
		ui.Column(c).Width(680).Gap(14).Children(func() {
			ui.Text(c, msgs.InstallTitle()).FontSize(18).Bold()
			if server == nil {
				return
			}
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, server.DisplayName).FontSize(14).Bold()
				ui.Text(c, plainText(server.Description)).FontSize(11.5).TextColor(t.TextMuted).MaxLines(2)
			})

			ui.Form(c, func() {
				ui.Field(c, msgs.GatewayProfile(), func() {
					names := make([]string, len(a.gwProfiles))
					for i, profile := range a.gwProfiles {
						names[i] = profile.Name
					}
					ui.Select(c, &a.installProfile, names)
				})
				ui.Field(c, msgs.InstallType(), func() {
					types := server.InstallTypes()
					if len(types) == 0 {
						ui.Text(c, msgs.NoConfigSupport()).FontSize(12).TextColor(t.Warning)
						return
					}
					ui.Select(c, &a.installType, types).Changed()
				})
			})
			if a.installType != "" {
				a.commandPreview(c, t, server.InstallCommand(a.installType), msgs)
			}
			a.paramFields(c, server, &a.installParms)
			if len(a.gwProfiles) == 0 {
				ui.Text(c, msgs.GatewayNoProfiles()).FontSize(12).TextColor(t.Warning)
			}
			if a.installError != "" {
				ui.Text(c, a.installError).FontSize(12).TextColor(t.Danger)
			}

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.installOpen = false
				}
				if ui.PrimaryButton(c, msgs.InstallConfirm()).Disabled(len(a.gwProfiles) == 0 || a.installType == "").Clicked() {
					a.confirmInstall(c, server, msgs)
				}
			})
		})
	})
}

func catalogValue(value string, params map[string]string) string {
	for key, param := range params {
		value = strings.ReplaceAll(value, "${"+key+"}", param)
	}
	return value
}

func catalogStoreServer(server *CatalogServer, kind string, params map[string]string) (*store.Server, error) {
	installation, ok := server.Installations[kind]
	if !ok || strings.TrimSpace(installation.Command) == "" {
		return nil, fmt.Errorf("%s has no %s installation", server.DisplayName, kind)
	}
	args := make([]string, len(installation.Args))
	for i, value := range installation.Args {
		args[i] = catalogValue(value, params)
	}
	var env map[string]string
	if len(installation.Env) > 0 {
		env = map[string]string{}
		for key, value := range installation.Env {
			env[key] = catalogValue(value, params)
		}
	}
	return &store.Server{
		Name: server.Name, Transport: "stdio",
		Command: catalogValue(installation.Command, params), Args: args, Env: env,
		Enabled: true, IdleSeconds: 600,
	}, nil
}

func (a *app) confirmInstall(c *ui.Context, server *CatalogServer, msgs locales.Messages) {
	if a.usage == nil {
		a.installError = msgs.GatewayDatabaseUnavailable()
		return
	}
	profile := ""
	for _, candidate := range a.gwProfiles {
		if candidate.Name == a.installProfile {
			profile = candidate.Name
			break
		}
	}
	if profile == "" {
		a.installError = msgs.GatewayNoProfiles()
		return
	}
	if key := requiredArgument(server, a.installParms); key != "" {
		a.installError = key + ": " + msgs.GatewayRequired()
		return
	}
	if _, err := a.usage.GetServer(context.Background(), server.Name); err == nil {
		a.installError = msgs.InstallAlready(server.DisplayName)
		return
	}
	entry, err := catalogStoreServer(server, a.installType, a.installParms)
	if err != nil {
		a.installError = msgs.InstallFailed(err.Error())
		return
	}
	if _, err := a.usage.UpsertServer(context.Background(), entry); err != nil {
		a.installError = msgs.InstallFailed(err.Error())
		return
	}
	if err := a.usage.AttachServer(context.Background(), profile, entry.Name, true); err != nil {
		a.installError = msgs.InstallFailed(err.Error())
		return
	}
	a.gwDirty = true
	a.loadGateway()
	a.installOpen = false
	c.Toast(msgs.InstallSuccess())
}

func workspaceHasServer(workspaceID, name string) bool {
	configs, err := data.GetWorkspaceServerConfigs(workspaceID)
	if err != nil {
		return false
	}
	for _, config := range configs {
		if config.Name == name {
			return true
		}
	}
	return false
}

func (a *app) commandPreview(c *ui.Context, t *ui.Theme, command string, msgs locales.Messages) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, msgs.InstallCommand()).FontSize(12).TextColor(t.TextMuted).Width(80).Shrink(0)
		a.codeBox(c, t, command)
		if ui.Button(c, "").Label(msgs.Copy()).Tooltip(msgs.Copy()).Padding(3, 6).
			Children(func() { ui.Icon(c, iconCopy).Size(12, 12) }).Clicked() {
			mygo.Clipboard.WriteText(command)
			c.Toast(msgs.Copied())
		}
	})
}

func requiredArgument(server *CatalogServer, values map[string]string) string {
	for _, key := range server.ArgumentKeys() {
		if server.Arguments[key].Required && strings.TrimSpace(values[key]) == "" {
			return key
		}
	}
	return ""
}

func (a *app) paramFields(c *ui.Context, server *CatalogServer, values *map[string]string) {
	keys := server.ArgumentKeys()
	if len(keys) == 0 {
		return
	}
	if *values == nil {
		*values = map[string]string{}
	}
	ui.Form(c, func() {
		for _, key := range keys {
			key := key
			argument := server.Arguments[key]
			label := key
			if argument.Required {
				label += " *"
			}
			ui.Field(c, label, func() {
				value := (*values)[key]
				if ui.TextInput(c, &value).Placeholder(argument.Example).Changed() {
					(*values)[key] = value
				}
			}).Description(argument.Description)
		}
	})
}
