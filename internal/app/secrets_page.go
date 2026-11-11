package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) loadSecrets() {
	if a.secretsStore == nil {
		a.secretNames, a.secretError = nil, ""
		return
	}
	names, err := a.secretsStore.List()
	if err != nil {
		a.secretError = err.Error()
		return
	}
	a.secretNames, a.secretError = names, ""
}

func (a *app) secretsPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, msgs.SecretsTitle()).FontSize(24).Bold()
					ui.Text(c, msgs.SecretsSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				if ui.Button(c, msgs.GatewayRefresh()).Clicked() {
					a.loadSecrets()
				}
				if ui.PrimaryButton(c, msgs.SecretsAdd()).Disabled(a.secretsStore == nil).Clicked() {
					a.openSecretForm("")
				}
			})
			if a.secretNotice != "" {
				ui.Text(c, a.secretNotice).FontSize(12).TextColor(t.Success)
			}
			if a.secretsStore == nil || a.secretError != "" {
				message := a.secretError
				if message == "" {
					message = msgs.SecretsUnavailable()
				}
				ui.Text(c, message).FontSize(12).TextColor(t.Danger)
				return
			}
			if len(a.secretNames) == 0 {
				ui.Column(c).FillWidth().Padding(44, 36).Gap(10).Center().Radius(14).
					Background(t.Surface).Border(1, t.Border).BorderStyle(ui.BorderDashed).Children(func() {
					a.iconTile(c, t, iconKey, 44, 20)
					ui.Text(c, msgs.SecretsEmpty()).FontSize(15).Bold()
					ui.Text(c, msgs.SecretsEmptyHint()).FontSize(12).TextColor(t.TextMuted)
					if ui.PrimaryButton(c, msgs.SecretsAdd()).Clicked() {
						a.openSecretForm("")
					}
				})
				return
			}
			card := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
			card.Children(func() {
				columns := []ui.TableColumn{
					{Title: msgs.SecretsName()},
					{Title: msgs.ServiceActions(), Width: 160, Fixed: true},
				}
				rows := a.secretNames
				ui.Table(c, nil, columns, len(rows), func(row, col int) {
					name := rows[row]
					switch col {
					case 0:
						ui.Text(c, name).FontSize(12).Bold().SingleLine()
					case 1:
						ui.Row(c).Gap(4).Children(func() {
							if ui.Button(c, msgs.SecretsEdit()).FontSize(11).Padding(3, 8).Clicked() {
								a.openSecretForm(name)
							}
							if ui.Button(c, msgs.SecretsDelete()).FontSize(11).Padding(3, 8).Clicked() {
								a.secretDelete = name
							}
						})
					}
				})
			})
		})
	})
}

func (a *app) openSecretForm(name string) {
	a.secretFormOpen = true
	a.secretEditing = name
	a.secretName = name
	a.secretValue = ""
	a.secretFormError = ""
}

func (a *app) saveSecret(c *ui.Context, msgs locales.Messages) {
	if a.secretsStore == nil {
		a.secretFormError = msgs.SecretsUnavailable()
		return
	}
	name := strings.TrimSpace(a.secretName)
	value := a.secretValue
	if name == "" {
		a.secretFormError = msgs.GatewayRequired()
		return
	}
	if value == "" {
		a.secretFormError = msgs.SecretsValueRequired()
		return
	}
	if err := a.secretsStore.Set(name, value); err != nil {
		a.secretFormError = err.Error()
		return
	}
	a.secretFormOpen = false
	a.secretValue = ""
	a.loadSecrets()
	if a.gwRunning {
		a.gwDirty = true
	}
	a.secretNotice = msgs.SecretsSaved()
	c.Toast(msgs.SecretsSaved())
}

func (a *app) deleteSecret(c *ui.Context, msgs locales.Messages) {
	if a.secretsStore == nil || a.secretDelete == "" {
		return
	}
	if err := a.secretsStore.Delete(a.secretDelete); err != nil {
		c.Toast(err.Error())
	} else {
		a.loadSecrets()
		if a.gwRunning {
			a.gwDirty = true
		}
		a.secretNotice = msgs.SecretsDeleted()
		c.Toast(msgs.SecretsDeleted())
	}
	a.secretDelete = ""
}

func (a *app) secretDialogs(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.secretFormOpen, func() {
		title := msgs.SecretsAddTitle()
		if a.secretEditing != "" {
			title = msgs.SecretsEditTitle()
		}
		ui.Column(c).Width(480).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(18).Bold()
			ui.Field(c, msgs.SecretsName(), func() {
				input := ui.TextInput(c, &a.secretName).ReadOnly(a.secretEditing != "")
				if a.secretEditing == "" {
					input.AutoFocus()
				}
			}).Description(msgs.SecretsNameHint())
			ui.Field(c, msgs.SecretsValue(), func() {
				input := ui.TextInput(c, &a.secretValue).Password()
				if a.secretEditing != "" {
					input.AutoFocus()
				}
			}).Description(msgs.SecretsValueHint())
			if a.secretFormError != "" {
				ui.Text(c, a.secretFormError).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.secretFormOpen = false
				}
				if ui.PrimaryButton(c, msgs.SecretsSave()).Clicked() {
					a.saveSecret(c, msgs)
				}
			})
		})
	})
	open := a.secretDelete != ""
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			ui.Text(c, msgs.SecretsDeleteTitle()).FontSize(17).Bold()
			ui.Text(c, msgs.SecretsDeleteMessage(a.secretDelete)).FontSize(12.5).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.secretDelete = ""
				}
				if ui.PrimaryButton(c, msgs.SecretsDelete()).Clicked() {
					a.deleteSecret(c, msgs)
				}
			})
		})
	})
	if !open {
		a.secretDelete = ""
	}
}
