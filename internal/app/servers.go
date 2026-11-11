package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) serversPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, msgs.ServersTitle()).FontSize(24).Bold()
					ui.Text(c, msgs.ServersSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				if ui.Button(c, msgs.NavCatalog()).Clicked() {
					a.router.Push("/catalog")
				}
				if ui.Button(c, msgs.GatewayImport()).Clicked() {
					a.openImportDialog(msgs)
				}
				if ui.PrimaryButton(c, msgs.GatewayAddServer()).Clicked() {
					a.openAddServerDialog(c, msgs)
				}
				if a.gwDirty && a.gwRunning {
					if ui.Button(c, msgs.GatewayRestart()).Clicked() {
						a.restartGateway(msgs)
					}
				}
				if ui.Button(c, msgs.GatewayRefresh()).Clicked() {
					a.loadGateway()
				}
			})
			ui.SearchField(c, &a.gwServerQuery).Placeholder(msgs.ServersSearch()).Label(msgs.ServersSearch())
			a.gatewayServers(c, t, msgs)
		})
	})
}
