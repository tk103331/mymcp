package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) profilesPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Scroll(c).Grow(1).Padding(30, 28, 26, 28).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, msgs.ProfilesTitle()).FontSize(24).Bold()
					ui.Text(c, msgs.ProfilesSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				if ui.PrimaryButton(c, msgs.GatewayNewProfile()).Clicked() {
					a.openProfileDialog("")
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
			a.gatewayProfiles(c, t, msgs)
			a.gatewayKeys(c, t, msgs)
		})
	})
}
