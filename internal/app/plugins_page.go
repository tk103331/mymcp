package app

import (
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/internal/plugin"
	"github.com/tk103331/mymcp/locales"
)

func pluginsDir() string {
	dir := mymcpDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "plugins")
}

func (a *app) loadPlugins() {
	if a.syncManager == nil {
		a.pluginList, a.pluginErrors = nil, nil
		return
	}
	a.pluginList = a.syncManager.Plugins()
	a.builtinPluginList = a.syncManager.BuiltinPlugins()
	a.pluginErrors = a.syncManager.PluginErrors()
	for _, p := range a.pluginList {
		if raw, name, ok := p.Icon(); ok {
			registerClientIcon("plugin:"+p.Manifest.ID, raw, name)
		}
	}
	for _, p := range a.builtinPluginList {
		if raw, name, ok := p.Icon(); ok {
			registerClientIcon("plugin:"+p.Manifest.ID, raw, name)
		}
	}
}

func (a *app) installPlugin(c *ui.Context, msgs locales.Messages) {
	if a.syncManager == nil || a.win == nil {
		a.pluginError = msgs.GatewayConfigUnavailable()
		return
	}
	a.pluginError, a.pluginNotice = "", ""
	title := msgs.PluginsChoose()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win, Title: title, Directory: true, ButtonLabel: msgs.PluginsInstall(),
		})
		if err != nil {
			a.pluginResult(err, "")
			return
		}
		if len(paths) == 0 {
			return
		}
		installed, err := plugin.Install(paths[0], pluginsDir())
		if err != nil {
			a.pluginResult(err, "")
			return
		}
		a.pluginResult(nil, installed.Manifest.Label)
	}()
}

func (a *app) pluginResult(err error, label string) {
	if a.win == nil {
		return
	}
	a.win.Update(func() {
		if err != nil {
			a.pluginError = err.Error()
			return
		}
		a.syncManager.ReloadPlugins()
		a.loadPlugins()
		a.loadClients()
		a.pluginNotice = label
	})
}

func (a *app) removePlugin(c *ui.Context, msgs locales.Messages) {
	if a.pluginRemove == nil || a.syncManager == nil {
		return
	}
	for _, status := range a.syncManager.Statuses() {
		if status.Client == a.pluginRemove.Manifest.ID && status.Managed {
			c.Toast(msgs.PluginsDetachFirst())
			a.pluginRemove = nil
			return
		}
	}
	if err := plugin.Remove(pluginsDir(), a.pluginRemove.Manifest.ID); err != nil {
		c.Toast(err.Error())
		a.pluginRemove = nil
		return
	}
	a.syncManager.ReloadPlugins()
	a.loadPlugins()
	a.loadClients()
	c.Toast(msgs.PluginsRemoved())
	a.pluginRemove = nil
}

func (a *app) pluginsPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	if a.syncManager == nil {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, msgs.PluginsEmpty()).FontSize(15).TextColor(t.TextMuted)
		})
		return
	}
	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, msgs.PluginsTitle()).FontSize(24).Bold()
					ui.Text(c, msgs.PluginsSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				if ui.PrimaryButton(c, msgs.PluginsInstall()).Clicked() {
					a.installPlugin(c, msgs)
				}
				if ui.Button(c, msgs.PluginsReload()).Clicked() {
					a.syncManager.ReloadPlugins()
					a.loadPlugins()
					a.loadClients()
				}
			})
			if a.pluginNotice != "" {
				ui.Textf(c, "%s: %s", msgs.PluginsInstalled(), a.pluginNotice).FontSize(12).TextColor(t.Success)
			}
			if a.pluginError != "" {
				ui.Text(c, a.pluginError).FontSize(12).TextColor(t.Danger)
			}
			if len(a.pluginErrors) > 0 {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, msgs.PluginsLoadErrors()).FontSize(12).Bold().TextColor(t.Warning)
					for _, err := range a.pluginErrors {
						ui.Text(c, err.Error()).FontSize(11).TextColor(t.TextMuted)
					}
				})
			}

			a.pluginSection(c, t, msgs, msgs.PluginsInstalledSection(), len(a.pluginList), func() {
				if len(a.pluginList) == 0 {
					ui.Column(c).FillWidth().Padding(28, 24).Gap(8).Center().Radius(14).
						Background(t.Surface).Border(1, t.Border).BorderStyle(ui.BorderDashed).Children(func() {
						ui.Text(c, msgs.PluginsEmpty()).FontSize(14).Bold()
						ui.Text(c, msgs.PluginsEmptyHint()).FontSize(12).TextColor(t.TextMuted)
						if ui.PrimaryButton(c, msgs.PluginsInstall()).Clicked() {
							a.installPlugin(c, msgs)
						}
					})
					return
				}
				a.pluginGrid(c, t, msgs, a.pluginList, true)
			})

			a.pluginSection(c, t, msgs, msgs.PluginsBuiltinSection(), len(a.builtinPluginList), func() {
				ui.Text(c, msgs.PluginsBuiltinHint()).FontSize(11.5).TextColor(t.TextMuted)
				a.pluginGrid(c, t, msgs, a.builtinPluginList, false)
			})
		})
	})
}

func (a *app) pluginSection(c *ui.Context, t *ui.Theme, msgs locales.Messages, title string, count int, body func()) {
	ui.Row(c).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(14).Bold()
		ui.Spacer(c)
		ui.Textf(c, "%d", count).FontSize(11).TextColor(t.TextMuted)
	})
	body()
}

func (a *app) pluginGrid(c *ui.Context, t *ui.Theme, msgs locales.Messages, plugins []*plugin.Plugin, removable bool) {
	ui.Grid(c).Columns(2).Gap(14).Children(func() {
		for _, p := range plugins {
			p := p
			ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).
				Border(1, t.Border).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					clientIcon(c, "plugin:"+p.Manifest.ID, 30, t.Accent)
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, p.Manifest.Label).FontSize(15).Bold().SingleLine()
						ui.Textf(c, "%s · v%s", p.Manifest.ID, p.Manifest.Version).FontSize(10.5).TextColor(t.TextMuted)
					})
					if p.Manifest.Builtin {
						ui.Text(c, msgs.PluginsBuiltinBadge()).FontSize(10).TextColor(t.Accent)
					} else if removable {
						if ui.Button(c, "").Label(msgs.PluginsRemove()).Tooltip(msgs.PluginsRemove()).
							Padding(4, 7).Children(func() { ui.Icon(c, iconTrash).Size(12, 12).TextColor(t.Danger) }).Clicked() {
							a.pluginRemove = p
						}
					}
				})
				if p.Manifest.Description != "" {
					ui.Text(c, p.Manifest.Description).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
				}
				if p.Dir != "" {
					ui.Text(c, p.Dir).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
				} else if len(p.Manifest.Paths) == 0 {
					ui.Text(c, msgs.PluginsPreviewOnly()).FontSize(10.5).TextColor(t.TextMuted)
				} else if p.Manifest.Format != "" {
					ui.Textf(c, "%s · %s", p.Manifest.Format, p.Manifest.Section).FontSize(10.5).TextColor(t.TextMuted)
				}
			})
		}
	})
}

func (a *app) pluginDialogs(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	open := a.pluginRemove != nil
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(420).Gap(12).Children(func() {
			ui.Text(c, msgs.PluginsRemoveTitle()).FontSize(17).Bold()
			if a.pluginRemove != nil {
				ui.Text(c, msgs.PluginsRemoveMessage(a.pluginRemove.Manifest.Label)).FontSize(12.5).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.pluginRemove = nil
				}
				if ui.PrimaryButton(c, msgs.PluginsRemove()).Clicked() {
					a.removePlugin(c, msgs)
				}
			})
		})
	})
	if !open {
		a.pluginRemove = nil
	}
}
