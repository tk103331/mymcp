package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/internal/gateway"
	"github.com/tk103331/mymcp/internal/importer"
	"github.com/tk103331/mymcp/internal/oauth"
	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) loadGateway() {
	if a.usage == nil {
		a.gwServers, a.gwProfiles, a.gwCounts, a.gwStatus = nil, nil, nil, nil
		a.gwServerProfiles, a.gwServerErrors = nil, nil
		return
	}
	ctx := context.Background()
	servers, err := a.usage.ListServers(ctx)
	if err != nil {
		a.gwError = err.Error()
		return
	}
	profiles, err := a.usage.ListProfiles(ctx)
	if err != nil {
		a.gwError = err.Error()
		return
	}
	counts := map[string]int{}
	serverProfiles := map[string][]string{}
	for _, profile := range profiles {
		list, err := a.usage.AllServersForProfile(ctx, profile.Name)
		if err == nil {
			counts[profile.Name] = len(list)
			for _, server := range list {
				serverProfiles[server.Name] = append(serverProfiles[server.Name], profile.Name)
			}
		}
	}
	keys, err := a.usage.ListKeys(ctx)
	if err != nil {
		a.gwError = err.Error()
		return
	}
	a.gwServers, a.gwProfiles, a.gwCounts, a.gwKeys = servers, profiles, counts, keys
	a.gwServerProfiles = serverProfiles
	a.gwServerErrors = map[string]string{}
	if a.gwRunning && a.gwManager != nil {
		a.gwStatus = a.gwManager.Statuses()
		for _, status := range a.gwStatus {
			for name, why := range status.Errors {
				a.gwServerErrors[name] = why
			}
		}
	} else {
		a.gwStatus = nil
	}
}

func (a *app) startGateway(msgs locales.Messages) {
	if a.gwRunning || a.gwStarting {
		return
	}
	if a.usage == nil {
		a.gwError = msgs.GatewayDatabaseUnavailable()
		return
	}
	a.gwStarting, a.gwError = true, ""
	addr := a.gwAddr
	go func() {
		secretStore := a.secretsStore
		if secretStore == nil {
			var err error
			secretStore, err = secrets.Open(mymcpDir())
			if err != nil {
				a.finishGatewayStart(err)
				return
			}
		}
		manager, err := gateway.NewManager(context.Background(), a.usage, secretStore, gateway.ManagerOptions{
			BaseURL: "http://" + addr, ToolSearch: true, ToolSearchThreshold: 10,
		})
		if err != nil {
			a.finishGatewayStart(err)
			return
		}
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			manager.Close()
			a.finishGatewayStart(err)
			return
		}
		server := &http.Server{
			Handler:           gateway.NewAuth(a.usage, false).Wrap(manager.Handler()),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		apply := func() {
			a.gwManager, a.gwServer = manager, server
			a.gwRunning, a.gwStarting = true, false
			a.gwError, a.gwDirty = "", false
			a.loadGateway()
			if a.router.Path() == "/debug" {
				a.loadDebug()
			}
		}
		if a.win != nil {
			a.win.Update(apply)
		} else {
			apply()
		}
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Println("gateway:", err)
			a.finishGatewayStart(err)
		}
	}()
}

func (a *app) finishGatewayStart(err error) {
	log.Printf("gateway: %v", err)
	finish := func() {
		a.gwStarting, a.gwRunning, a.gwStatus = false, false, nil
		a.gwError = err.Error()
	}
	if a.win != nil {
		a.win.Update(finish)
	} else {
		finish()
	}
}

func (a *app) stopGateway() {
	server, manager := a.gwServer, a.gwManager
	a.gwServer, a.gwManager = nil, nil
	a.gwRunning, a.gwStarting = false, false
	a.gwError, a.gwDirty, a.gwStatus = "", false, nil
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = server.Shutdown(ctx)
		cancel()
	}
	if manager != nil {
		manager.Close()
	}
	a.loadGateway()
	if a.router.Path() == "/debug" {
		a.loadDebug()
	}
}

// restartGateway rebuilds the profile gateways so registry changes apply.
func (a *app) restartGateway(msgs locales.Messages) {
	a.stopGateway()
	a.startGateway(msgs)
}

func (a *app) gatewayStatus(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	text, color := msgs.GatewayStopped(), t.TextMuted
	if a.gwRunning {
		text, color = msgs.GatewayRunning(), t.Success
	} else if a.gwStarting {
		text, color = msgs.GatewayStarting(), t.Warning
	}
	ui.Row(c).Padding(3, 9).Radius(999).Background(color.Alpha(0.12)).
		AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(color)
		ui.Text(c, text).FontSize(11).TextColor(color)
	})
}

func (a *app) gatewayServers(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	card := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
	card.Children(func() {
		query := strings.ToLower(strings.TrimSpace(a.gwServerQuery))
		rows := make([]*store.Server, 0, len(a.gwServers))
		for _, server := range a.gwServers {
			if query == "" ||
				strings.Contains(strings.ToLower(server.Name), query) ||
				strings.Contains(strings.ToLower(server.Transport), query) ||
				strings.Contains(strings.ToLower(server.URL), query) ||
				strings.Contains(strings.ToLower(server.Command), query) {
				rows = append(rows, server)
			}
		}
		if len(rows) == 0 {
			ui.Text(c, msgs.GatewayNoServers()).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, msgs.GatewayNoServersHint()).FontSize(11.5).TextColor(t.TextMuted)
			return
		}
		columns := []ui.TableColumn{
			{Title: msgs.ServiceName()},
			{Title: msgs.GatewayTransport(), Width: 80},
			{Title: msgs.GatewayProfiles(), Width: 150},
			{Title: msgs.GatewayEnabled(), Width: 65},
			{Title: msgs.ServiceStatus(), Width: 100},
			{Title: msgs.ServiceActions(), Width: 150, Fixed: true},
		}
		ui.Table(c, nil, columns, len(rows), func(row, col int) {
			server := rows[row]
			switch col {
			case 0:
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, server.Name).FontSize(12).Bold().SingleLine()
					ui.Text(c, serverTarget(server)).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
				})
			case 1:
				ui.Text(c, server.Transport).FontSize(11.5).TextColor(t.TextMuted)
			case 2:
				profiles := a.gwServerProfiles[server.Name]
				value := "—"
				if len(profiles) > 0 {
					value = strings.Join(profiles, ", ")
				}
				ui.Text(c, value).FontSize(11).TextColor(t.TextMuted).SingleLine()
			case 3:
				enabled := server.Enabled
				if ui.Switch(c, &enabled).Label(server.Name).Changed() {
					a.setServerEnabled(server, enabled, c, msgs)
				}
			case 4:
				text, color := msgs.GatewayStopped(), t.TextMuted
				switch {
				case a.gwServerErrors[server.Name] != "":
					text, color = msgs.StatusError(), t.Danger
				case !server.Enabled:
					text, color = msgs.GatewayDisabled(), t.TextMuted
				case a.gwRunning:
					text, color = msgs.GatewayReady(), t.Success
				}
				badge := ui.Text(c, text).FontSize(11).TextColor(color)
				if a.gwServerErrors[server.Name] != "" {
					badge.Tooltip(a.gwServerErrors[server.Name])
				}
			case 5:
				ui.Row(c).Gap(4).Children(func() {
					if ui.Button(c, msgs.Details()).FontSize(11).Padding(3, 8).Clicked() {
						a.openServerDetail(server, msgs)
					}
					if ui.Button(c, "").Label(msgs.Edit()).Tooltip(msgs.Edit()).
						Padding(4, 7).Children(func() { ui.Icon(c, iconEdit).Size(12, 12) }).Clicked() {
						a.openEditServerDialog(server, c)
					}
					if ui.Button(c, "").Label(msgs.GatewayDelete()).Tooltip(msgs.GatewayDelete()).
						Padding(4, 7).Children(func() { ui.Icon(c, iconTrash).Size(12, 12).TextColor(t.Danger) }).Clicked() {
						a.gwDelete = server
					}
				})
			}
		}).Height(tableHeight(len(rows)))
	})
}

func serverTarget(server *store.Server) string {
	if server.Transport == "stdio" {
		target := strings.TrimSpace(server.Command + " " + strings.Join(server.Args, " "))
		if server.Executor == "docker" && server.Image != "" {
			target = "docker " + server.Image
		}
		return target
	}
	return server.URL
}

// gatewayHealth shows each running profile's server and tool counts and
// the upstream errors the gateway saw while opening servers.
func (a *app) gatewayHealth(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	if !a.gwRunning || len(a.gwStatus) == 0 {
		return
	}
	names := make([]string, 0, len(a.gwStatus))
	for name := range a.gwStatus {
		names = append(names, name)
	}
	sort.Strings(names)
	card := ui.Column(c).Padding(18).Gap(9).Radius(14).Background(t.Surface).Border(1, t.Border)
	card.Children(func() {
		ui.Text(c, msgs.GatewayHealth()).FontSize(14).Bold()
		for _, name := range names {
			status := a.gwStatus[name]
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, name).FontSize(12).Bold()
				ui.Textf(c, "%d/%d", status.Running, status.Servers).FontSize(11).TextColor(t.TextMuted)
				ui.Textf(c, "%s: %d", msgs.UsageTools(), status.Tools).FontSize(11).TextColor(t.TextMuted)
				if status.ToolSearch {
					ui.Text(c, msgs.GatewayToolSearch()).FontSize(10).TextColor(t.Accent)
				}
			})
			for _, serverName := range sortedKeys(status.Errors) {
				ui.Textf(c, "%s: %s", serverName, status.Errors[serverName]).FontSize(11).TextColor(t.Danger)
			}
		}
	})
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (a *app) gatewayProfiles(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	card := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
	card.Children(func() {
		if len(a.gwProfiles) == 0 {
			ui.Text(c, msgs.GatewayNoProfiles()).FontSize(12).TextColor(t.TextMuted)
			return
		}
		rows := a.gwProfiles
		columns := []ui.TableColumn{
			{Title: msgs.GatewayProfileName()},
			{Title: msgs.GatewayEndpoint(), Width: 250},
			{Title: msgs.GatewayServerCount(), Width: 80, Align: ui.End},
			{Title: msgs.ServiceActions(), Width: 90, Fixed: true},
		}
		ui.Table(c, nil, columns, len(rows), func(row, col int) {
			profile := rows[row]
			switch col {
			case 0:
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, profile.Name).FontSize(12).Bold()
					if profile.Description != "" {
						ui.Text(c, profile.Description).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
					}
				})
			case 1:
				ui.Text(c, "http://"+a.gwAddr+"/mcp/"+profile.Name).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
			case 2:
				ui.Textf(c, "%d", a.gwCounts[profile.Name]).FontSize(12)
			case 3:
				ui.Row(c).Gap(4).Children(func() {
					if ui.Button(c, "").Label(msgs.Edit()).Tooltip(msgs.Edit()).Padding(4, 7).
						Children(func() { ui.Icon(c, iconEdit).Size(12, 12) }).Clicked() {
						a.openProfileDialog(profile.Name)
					}
					if ui.Button(c, "").Label(msgs.Copy()).Tooltip(msgs.Copy()).Padding(4, 7).
						Children(func() { ui.Icon(c, iconCopy).Size(12, 12) }).Clicked() {
						mygo.Clipboard.WriteText("http://" + a.gwAddr + "/mcp/" + profile.Name)
						c.Toast(msgs.Copied())
					}
				})
			}
		}).Height(tableHeight(len(rows)))
	})
}

func (a *app) openProfileDialog(name string) {
	if a.usage == nil {
		return
	}
	a.gwProfileOpen = true
	a.gwProfileEditing = name
	a.gwProfileName, a.gwProfileDesc, a.gwProfileError = name, "", ""
	a.gwProfileServers = map[string]bool{}
	if name != "" {
		for _, profile := range a.gwProfiles {
			if profile.Name == name {
				a.gwProfileDesc = profile.Description
				break
			}
		}
		if list, err := a.usage.AllServersForProfile(context.Background(), name); err == nil {
			for _, server := range list {
				a.gwProfileServers[server.Name] = true
			}
		}
	}
}

func (a *app) saveProfile(c *ui.Context, msgs locales.Messages) {
	if a.usage == nil {
		a.gwProfileError = msgs.GatewayDatabaseUnavailable()
		return
	}
	name := strings.TrimSpace(a.gwProfileName)
	if name == "" {
		a.gwProfileError = msgs.GatewayRequired()
		return
	}
	if a.gwProfileEditing == "" {
		for _, existing := range a.gwProfiles {
			if existing.Name == name {
				a.gwProfileError = msgs.GatewayProfileExists(name)
				return
			}
		}
	}
	var profile *store.Profile
	var err error
	if a.gwProfileEditing == "" {
		profile, err = a.usage.UpsertProfile(context.Background(), &store.Profile{
			Name: name, Description: strings.TrimSpace(a.gwProfileDesc),
		})
	} else {
		profile, err = a.usage.Profile(context.Background(), a.gwProfileEditing)
		if err == nil {
			profile.Description = strings.TrimSpace(a.gwProfileDesc)
			profile, err = a.usage.UpsertProfile(context.Background(), profile)
		}
	}
	if err != nil {
		a.gwProfileError = err.Error()
		return
	}
	for _, server := range a.gwServers {
		if a.gwProfileServers[server.Name] {
			err = a.usage.AttachServer(context.Background(), profile.Name, server.Name, true)
		} else if a.gwProfileEditing != "" {
			err = a.usage.DetachServer(context.Background(), profile.Name, server.Name)
		}
		if err != nil {
			a.gwProfileError = err.Error()
			return
		}
	}
	a.gwProfileOpen = false
	a.gwDirty = true
	a.loadGateway()
	c.Toast(msgs.GatewayProfileSaved())
}

func (a *app) profileDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.gwProfileOpen, func() {
		title := msgs.GatewayNewProfile()
		if a.gwProfileEditing != "" {
			title = msgs.GatewayEditProfile()
		}
		ui.Column(c).Width(560).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(18).Bold()
			ui.Field(c, msgs.GatewayProfileName(), func() {
				input := ui.TextInput(c, &a.gwProfileName).ReadOnly(a.gwProfileEditing != "")
				if a.gwProfileEditing == "" {
					input.AutoFocus()
				}
			})
			ui.Field(c, msgs.GatewayProfileDescription(), func() {
				ui.TextArea(c, &a.gwProfileDesc).Height(64)
			})
			ui.Text(c, msgs.GatewayProfileServers()).FontSize(12.5).Bold()
			if len(a.gwServers) == 0 {
				ui.Text(c, msgs.GatewayProfileNoServers()).FontSize(11.5).TextColor(t.TextMuted)
			} else {
				ui.Scroll(c).Height(210).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, server := range a.gwServers {
							server := server
							attached := a.gwProfileServers[server.Name]
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								if ui.Switch(c, &attached).Label(server.Name).Changed() {
									a.gwProfileServers[server.Name] = attached
								}
								ui.Column(c).Gap(1).Children(func() {
									ui.Text(c, server.Name).FontSize(12).Bold().SingleLine()
									ui.Text(c, server.Transport).FontSize(10.5).TextColor(t.TextMuted)
								})
							})
						}
					})
				})
			}
			if a.gwProfileError != "" {
				ui.Text(c, a.gwProfileError).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.gwProfileOpen = false
				}
				if ui.PrimaryButton(c, msgs.GatewaySaveProfile()).Clicked() {
					a.saveProfile(c, msgs)
				}
			})
		})
	})
}

func (a *app) setServerEnabled(server *store.Server, enabled bool, c *ui.Context, msgs locales.Messages) {
	if a.usage == nil {
		c.Toast(msgs.GatewayDatabaseUnavailable())
		return
	}
	copy := *server
	copy.Enabled = enabled
	if _, err := a.usage.UpsertServer(context.Background(), &copy); err != nil {
		c.Toast(err.Error())
		return
	}
	a.gwDirty = true
	a.loadGateway()
	c.Toast(msgs.GatewayServerToggled())
}

func (a *app) openAddServerDialog(c *ui.Context, msgs locales.Messages) {
	if a.usage == nil {
		a.gwError = msgs.GatewayDatabaseUnavailable()
		return
	}
	_, _ = a.usage.EnsureDefaultProfile(context.Background())
	a.gwAddOpen = true
	a.gwEditing, a.gwTab = "", 0
	a.gwName, a.gwTransport, a.gwCommand, a.gwArgs, a.gwURL = "", "stdio", "", "", ""
	a.gwProfile = "default"
	a.gwEnvText, a.gwHeadersText = "", ""
	a.gwOAuth, a.gwAuthURL, a.gwTokenURL, a.gwClientID = false, "", "", ""
	a.gwClientSecret, a.gwOAuthRedirect, a.gwScopesText, a.gwPKCE = "", "", "", true
	a.gwOAuthBusy, a.gwOAuthNotice, a.gwOAuthError = false, "", ""
	a.gwExecutor, a.gwImage, a.gwNetwork, a.gwVolumesText = "local", "", "", ""
	a.gwError = ""
	a.loadGateway()
}

func (a *app) openEditServerDialog(server *store.Server, c *ui.Context) {
	a.gwAddOpen = true
	a.gwEditing, a.gwTab = server.Name, 0
	a.gwName, a.gwTransport, a.gwCommand, a.gwArgs, a.gwURL = server.Name, server.Transport, server.Command, joinArgs(server.Args), server.URL
	a.gwProfile = "default"
	a.gwEnvText, a.gwHeadersText = mapLines(server.Env), mapLines(server.Headers)
	a.gwOAuth = server.OAuth != nil
	a.gwOAuthBusy, a.gwOAuthNotice, a.gwOAuthError = false, "", ""
	a.gwAuthURL, a.gwTokenURL, a.gwClientID, a.gwClientSecret = "", "", "", ""
	a.gwOAuthRedirect, a.gwScopesText, a.gwPKCE = "", "", true
	if server.OAuth != nil {
		a.gwAuthURL = server.OAuth.AuthorizationURL
		a.gwTokenURL = server.OAuth.TokenURL
		a.gwClientID = server.OAuth.ClientID
		a.gwClientSecret = server.OAuth.ClientSecret
		a.gwOAuthRedirect = server.OAuth.RedirectURI
		a.gwScopesText = strings.Join(server.OAuth.Scopes, " ")
		a.gwPKCE = server.OAuth.UsePKCE
	}
	a.gwExecutor, a.gwImage, a.gwNetwork = server.Executor, server.Image, server.Network
	if a.gwExecutor == "" {
		a.gwExecutor = "local"
	}
	a.gwVolumesText = strings.Join(server.Volumes, "\n")
	a.gwError = ""
}

func (a *app) oauthConfig() *store.OAuthConfig {
	return &store.OAuthConfig{
		AuthorizationURL: strings.TrimSpace(a.gwAuthURL),
		TokenURL:         strings.TrimSpace(a.gwTokenURL),
		ClientID:         strings.TrimSpace(a.gwClientID),
		ClientSecret:     strings.TrimSpace(a.gwClientSecret),
		Scopes:           strings.Fields(a.gwScopesText),
		RedirectURI:      strings.TrimSpace(a.gwOAuthRedirect),
		UsePKCE:          a.gwPKCE,
	}
}

// loginOAuth saves the current OAuth fields, then runs the authorization
// code flow in the browser and stores the token in the secret vault.
func (a *app) loginOAuth(msgs locales.Messages) {
	if a.gwEditing == "" {
		a.gwOAuthError = msgs.GatewayOAuthSaveFirst()
		return
	}
	if a.gwAuthURL == "" || a.gwTokenURL == "" || a.gwClientID == "" {
		a.gwOAuthError = msgs.GatewayOAuthInvalid()
		return
	}
	if a.secretsStore == nil {
		a.gwOAuthError = msgs.SecretsUnavailable()
		return
	}
	cfg := a.oauthConfig()
	if err := a.usage.SetServerOAuth(context.Background(), a.gwEditing, cfg); err != nil {
		a.gwOAuthError = err.Error()
		return
	}
	a.gwOAuthBusy, a.gwOAuthNotice, a.gwOAuthError = true, "", ""
	name := a.gwEditing
	go func() {
		_, err := oauth.Login(context.Background(), cfg, name, a.secretsStore,
			func(url string) error { return mygo.Shell.OpenExternal(url) }, io.Discard)
		finish := func() {
			a.gwOAuthBusy = false
			if err != nil {
				a.gwOAuthError = err.Error()
				return
			}
			a.gwOAuthError, a.gwOAuthNotice = "", msgs.GatewayOAuthConnected()
			a.gwDirty = true
		}
		if a.win != nil {
			a.win.Update(finish)
		} else {
			finish()
		}
	}()
}

func (a *app) saveServer(c *ui.Context, msgs locales.Messages) {
	if a.usage == nil {
		a.gwError = msgs.GatewayDatabaseUnavailable()
		return
	}
	name := strings.TrimSpace(a.gwName)
	if name == "" {
		a.gwError = msgs.GatewayRequired()
		return
	}
	transport := a.gwTransport
	var server *store.Server
	if a.gwEditing == "" {
		if _, err := a.usage.GetServer(context.Background(), name); err == nil {
			a.gwError = msgs.GatewayServerExists(name)
			return
		}
	}
	if a.gwEditing != "" {
		existing, err := a.usage.GetServer(context.Background(), a.gwEditing)
		if err != nil {
			a.gwError = err.Error()
			return
		}
		copy := *existing
		server = &copy
	} else {
		server = &store.Server{IdleSeconds: 600, Enabled: true}
	}
	server.Name, server.Transport, server.Executor = name, transport, a.gwExecutor
	server.Image, server.Network = a.gwImage, a.gwNetwork
	server.Volumes = splitLines(a.gwVolumesText)
	env, err := linesMap(a.gwEnvText)
	if err != nil {
		a.gwError = msgs.GatewayEnvironment() + ": " + err.Error()
		return
	}
	headers, err := linesMap(a.gwHeadersText)
	if err != nil {
		a.gwError = msgs.GatewayHeaders() + ": " + err.Error()
		return
	}
	if transport == "stdio" {
		server.Env, server.Headers = env, nil
	} else {
		server.Env, server.Headers = nil, headers
	}
	if transport == "stdio" {
		server.Command = strings.TrimSpace(a.gwCommand)
		args, err := splitArgs(a.gwArgs)
		if err != nil {
			a.gwError = msgs.GatewayArguments() + ": " + err.Error()
			return
		}
		server.Args = args
		if a.gwExecutor != "docker" && server.Command == "" {
			a.gwError = msgs.GatewayRequired()
			return
		}
	} else {
		server.URL = strings.TrimSpace(a.gwURL)
		if server.URL == "" {
			a.gwError = msgs.GatewayRequired()
			return
		}
	}
	if a.gwOAuth {
		if a.gwAuthURL == "" || a.gwTokenURL == "" || a.gwClientID == "" {
			a.gwError = msgs.GatewayRequired()
			return
		}
		server.OAuth = a.oauthConfig()
	} else {
		server.OAuth = nil
	}
	if _, err := a.usage.UpsertServer(context.Background(), server); err != nil {
		a.gwError = err.Error()
		return
	}
	if a.gwEditing == "" {
		profileName := a.gwProfile
		if profileName == "" {
			profileName = "default"
		}
		if err := a.usage.AttachServer(context.Background(), profileName, server.Name, true); err != nil {
			a.gwError = err.Error()
			return
		}
	}
	a.gwAddOpen = false
	a.gwDirty = true
	a.loadGateway()
	if a.gwEditing != "" {
		c.Toast(msgs.GatewayServerUpdated())
	} else {
		c.Toast(msgs.GatewayServerAdded())
	}
}

func linesMap(text string) (map[string]string, error) {
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("%q: want KEY=VALUE", line)
		}
		values[key] = strings.TrimSpace(value)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
}

func mapLines(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+values[key])
	}
	return strings.Join(lines, "\n")
}

func splitLines(text string) []string {
	var values []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			values = append(values, line)
		}
	}
	return values
}

func (a *app) deleteServer(c *ui.Context, msgs locales.Messages) {
	if a.gwDelete == nil {
		return
	}
	if a.usage == nil {
		c.Toast(msgs.GatewayDatabaseUnavailable())
		a.gwDelete = nil
		return
	}
	name := a.gwDelete.Name
	if err := a.usage.DeleteServer(context.Background(), name); err != nil {
		c.Toast(err.Error())
	} else {
		if a.secretsStore != nil {
			_ = a.secretsStore.Delete(oauth.Key(name))
		}
		a.gwDirty = true
		a.loadGateway()
		c.Toast(msgs.GatewayServerDeleted())
	}
	a.gwDelete = nil
}

func (a *app) gatewayKeys(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	card := ui.Column(c).Padding(18).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, msgs.GatewayKeys()).FontSize(14).Bold()
			ui.Spacer(c)
			if ui.Button(c, msgs.GatewayCreateKey()).Clicked() {
				a.gwKeyOpen = true
				a.gwKeyName, a.gwKeyProfile, a.gwKeyExpiry, a.gwKeyToken, a.gwKeyError = "", msgs.GatewayAllProfiles(), "", "", ""
				for _, profile := range a.gwProfiles {
					if profile.Name == "default" {
						a.gwKeyProfile = "default"
						break
					}
				}
				a.gwKeyRateText = "0"
			}
		})
		if len(a.gwKeys) == 0 {
			ui.Text(c, msgs.GatewayNoKeys()).FontSize(12).TextColor(t.TextMuted)
			return
		}
		rows := a.gwKeys
		columns := []ui.TableColumn{
			{Title: msgs.GatewayKeyName()},
			{Title: msgs.GatewayProfile(), Width: 130},
			{Title: msgs.GatewayKeyRateShort(), Width: 120, Align: ui.End},
			{Title: msgs.GatewayKeyExpiryShort(), Width: 110},
			{Title: msgs.ServiceActions(), Width: 90, Fixed: true},
		}
		ui.Table(c, nil, columns, len(rows), func(row, col int) {
			key := rows[row]
			switch col {
			case 0:
				ui.Text(c, key.Name).FontSize(12).Bold().SingleLine()
			case 1:
				ui.Text(c, key.Profile).FontSize(11.5).TextColor(t.TextMuted)
			case 2:
				ui.Textf(c, "%d", key.RatePerMin).FontSize(11.5).TextColor(t.TextMuted)
			case 3:
				expires := "—"
				if !key.ExpiresAt.IsZero() {
					expires = key.ExpiresAt.Format("2006-01-02")
				}
				ui.Text(c, expires).FontSize(11.5).TextColor(t.TextMuted)
			case 4:
				if ui.Button(c, msgs.GatewayRevoke()).FontSize(11).Padding(3, 8).Clicked() {
					a.gwKeyRevoke = key
				}
			}
		}).Height(tableHeight(len(rows)))
	})
}

func (a *app) createKey(c *ui.Context, msgs locales.Messages) {
	if a.usage == nil {
		a.gwKeyError = msgs.GatewayDatabaseUnavailable()
		return
	}
	if strings.TrimSpace(a.gwKeyName) == "" {
		a.gwKeyError = msgs.GatewayRequired()
		return
	}
	expiry, err := parseExpiry(a.gwKeyExpiry, msgs)
	if err != nil {
		a.gwKeyError = err.Error()
		return
	}
	rate := 0
	if strings.TrimSpace(a.gwKeyRateText) != "" {
		rate, err = strconv.Atoi(strings.TrimSpace(a.gwKeyRateText))
		if err != nil || rate < 0 {
			a.gwKeyError = msgs.GatewayRateInvalid()
			return
		}
	}
	profile := strings.TrimSpace(a.gwKeyProfile)
	if profile == msgs.GatewayAllProfiles() {
		profile = ""
	}
	token, _, err := a.usage.CreateKey(context.Background(), strings.TrimSpace(a.gwKeyName),
		profile, rate, expiry)
	if err != nil {
		a.gwKeyError = err.Error()
		return
	}
	a.gwKeyToken = token
	a.gwKeyError = ""
	a.loadGateway()
	c.Toast(msgs.GatewayKeyCreated())
}

func (a *app) revokeKey(c *ui.Context, msgs locales.Messages) {
	if a.gwKeyRevoke == nil {
		return
	}
	if err := a.usage.RevokeKey(context.Background(), a.gwKeyRevoke.Name); err != nil {
		c.Toast(err.Error())
	} else {
		a.loadGateway()
		c.Toast(msgs.GatewayKeyRevoked())
	}
	a.gwKeyRevoke = nil
}

func parseExpiry(value string, msgs locales.Messages) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return time.Time{}, fmt.Errorf("%s: %q", msgs.GatewayExpiryInvalid(), value)
		}
		return time.Now().Add(time.Duration(days) * 24 * time.Hour), nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return time.Time{}, fmt.Errorf("%s: %q", msgs.GatewayExpiryInvalid(), value)
	}
	return time.Now().Add(duration), nil
}

func (a *app) gatewayDialogs(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	a.addServerDialog(c, t, msgs)
	a.deleteServerDialog(c, t, msgs)
	a.keyDialog(c, t, msgs)
	a.revokeKeyDialog(c, t, msgs)
	a.importDialog(c, t, msgs)
	a.profileDialog(c, t, msgs)
	a.serverDetailDialog(c, t, msgs)
}

func (a *app) addServerDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.gwAddOpen, func() {
		ui.Column(c).Width(620).Gap(12).Children(func() {
			title := msgs.GatewayAddServer()
			if a.gwEditing != "" {
				title = msgs.GatewayEditServer()
			}
			ui.Text(c, title).FontSize(18).Bold()
			ui.Tabs(c, &a.gwTab, msgs.GatewayBasics(), msgs.GatewayEnvironment(), msgs.GatewayOAuth(), msgs.GatewaySandbox())
			switch a.gwTab {
			case 0:
				ui.Field(c, msgs.GatewayServerName(), func() {
					ui.TextInput(c, &a.gwName).AutoFocus().ReadOnly(a.gwEditing != "")
				})
				ui.Field(c, msgs.GatewayTransport(), func() {
					ui.Select(c, &a.gwTransport, []string{"stdio", "http", "sse"})
				})
				if a.gwTransport == "stdio" {
					ui.Field(c, msgs.GatewayCommand(), func() {
						ui.TextInput(c, &a.gwCommand).Placeholder("npx")
					})
					ui.Field(c, msgs.GatewayArguments(), func() {
						ui.TextInput(c, &a.gwArgs).Placeholder("-y @modelcontextprotocol/server-github")
					})
				} else {
					ui.Field(c, msgs.GatewayURL(), func() { ui.TextInput(c, &a.gwURL) })
				}
				if a.gwEditing == "" {
					profiles := make([]string, 0, len(a.gwProfiles))
					for _, profile := range a.gwProfiles {
						profiles = append(profiles, profile.Name)
					}
					if len(profiles) == 0 {
						profiles = []string{"default"}
					}
					if a.gwProfile == "" {
						a.gwProfile = profiles[0]
					}
					ui.Field(c, msgs.GatewayProfile(), func() {
						ui.Select(c, &a.gwProfile, profiles)
					})
				}
			case 1:
				if a.gwTransport == "stdio" {
					ui.Field(c, msgs.GatewayEnvironment(), func() {
						ui.TextArea(c, &a.gwEnvText).Height(120).Placeholder(msgs.GatewayEnvPlaceholder())
					}).Description(msgs.GatewayEnvHint())
				} else {
					ui.Field(c, msgs.GatewayHeaders(), func() {
						ui.TextArea(c, &a.gwHeadersText).Height(120).Placeholder(msgs.GatewayHeadersPlaceholder())
					}).Description(msgs.GatewayHeadersHint())
				}
			case 2:
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Switch(c, &a.gwOAuth).Label(msgs.GatewayOAuth())
					ui.Text(c, msgs.GatewayOAuth()).FontSize(12)
					ui.Text(c, msgs.GatewayOAuthHint()).FontSize(10.5).TextColor(t.TextMuted)
				})
				if a.gwOAuth {
					ui.Field(c, msgs.GatewayAuthorizationURL(), func() { ui.TextInput(c, &a.gwAuthURL) })
					ui.Field(c, msgs.GatewayTokenURL(), func() { ui.TextInput(c, &a.gwTokenURL) })
					ui.Field(c, msgs.GatewayClientID(), func() { ui.TextInput(c, &a.gwClientID) })
					ui.Field(c, msgs.GatewayClientSecret(), func() { ui.TextInput(c, &a.gwClientSecret).Placeholder("secret://name") })
					ui.Field(c, msgs.GatewayScopes(), func() { ui.TextInput(c, &a.gwScopesText).Placeholder("mcp.read mcp.write") })
					ui.Field(c, msgs.GatewayRedirectURI(), func() { ui.TextInput(c, &a.gwOAuthRedirect).Placeholder("http://127.0.0.1:33418/callback") })
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Switch(c, &a.gwPKCE).Label(msgs.GatewayPKCE())
						ui.Text(c, msgs.GatewayPKCE()).FontSize(12)
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if a.gwEditing == "" {
							ui.Text(c, msgs.GatewayOAuthSaveFirst()).FontSize(11.5).TextColor(t.TextMuted)
							return
						}
						if ui.Button(c, msgs.GatewayOAuthLogin()).Disabled(a.gwOAuthBusy).Clicked() {
							a.loginOAuth(msgs)
						}
						if a.gwOAuthBusy {
							ui.Text(c, msgs.GatewayOAuthBusy()).FontSize(11.5).TextColor(t.Warning)
						}
						if a.gwOAuthNotice != "" {
							ui.Text(c, a.gwOAuthNotice).FontSize(11.5).TextColor(t.Success)
						}
						if a.gwOAuthError != "" {
							ui.Text(c, a.gwOAuthError).FontSize(11.5).TextColor(t.Danger)
						}
					})
				}
			case 3:
				ui.Field(c, msgs.GatewayExecutor(), func() {
					label := msgs.GatewayExecutorLocal()
					if a.gwExecutor == "docker" {
						label = msgs.GatewayExecutorDocker()
					}
					ui.MenuButton(c, label, func(m *ui.Menu) {
						if m.Item(msgs.GatewayExecutorLocal()).Radio(true).Checked(a.gwExecutor != "docker").Chosen() {
							a.gwExecutor = "local"
						}
						if m.Item(msgs.GatewayExecutorDocker()).Radio(true).Checked(a.gwExecutor == "docker").Chosen() {
							a.gwExecutor = "docker"
						}
					}).Height(28).Padding(0, 10).Radius(8).Border(1, t.Border).FontSize(12)
				})
				if a.gwExecutor == "docker" {
					ui.Field(c, msgs.GatewayImage(), func() { ui.TextInput(c, &a.gwImage) })
					ui.Field(c, msgs.GatewayNetwork(), func() { ui.TextInput(c, &a.gwNetwork).Placeholder(msgs.GatewayNetworkPlaceholder()) })
					ui.Field(c, msgs.GatewayVolumes(), func() {
						ui.TextArea(c, &a.gwVolumesText).Height(90).Placeholder(msgs.GatewayVolumesPlaceholder())
					})
				}
			}
			if a.gwError != "" {
				ui.Text(c, a.gwError).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.gwAddOpen = false
				}
				if ui.PrimaryButton(c, msgs.GatewaySave()).Clicked() {
					a.saveServer(c, msgs)
				}
			})
		})
	})
}

func (a *app) deleteServerDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	open := a.gwDelete != nil
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(420).Gap(12).Children(func() {
			ui.Text(c, msgs.GatewayDeleteTitle()).FontSize(17).Bold()
			if a.gwDelete != nil {
				ui.Text(c, msgs.GatewayDeleteMessage(a.gwDelete.Name)).FontSize(12.5).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.gwDelete = nil
				}
				if ui.PrimaryButton(c, msgs.GatewayDelete()).Clicked() {
					a.deleteServer(c, msgs)
				}
			})
		})
	})
	if !open {
		a.gwDelete = nil
	}
}

func (a *app) keyDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.gwKeyOpen, func() {
		ui.Column(c).Width(480).Gap(12).Children(func() {
			if a.gwKeyToken != "" {
				ui.Text(c, msgs.GatewayKeyTokenTitle()).FontSize(18).Bold()
				ui.Text(c, msgs.GatewayKeyTokenHint()).FontSize(12).TextColor(t.TextMuted)
				code := a.gwKeyToken
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &code).ReadOnly(true).Grow(1)
					if ui.Button(c, msgs.Copy()).Clicked() {
						mygo.Clipboard.WriteText(code)
						c.Toast(msgs.Copied())
					}
				})
				ui.Row(c).Justify(ui.End).Children(func() {
					if ui.PrimaryButton(c, msgs.GatewayDone()).Clicked() {
						a.gwKeyOpen, a.gwKeyToken = false, ""
					}
				})
				return
			}
			ui.Text(c, msgs.GatewayCreateKey()).FontSize(18).Bold()
			ui.Field(c, msgs.GatewayKeyName(), func() { ui.TextInput(c, &a.gwKeyName).AutoFocus() })
			ui.Field(c, msgs.GatewayProfile(), func() {
				profiles := []string{msgs.GatewayAllProfiles()}
				for _, profile := range a.gwProfiles {
					profiles = append(profiles, profile.Name)
				}
				ui.Select(c, &a.gwKeyProfile, profiles)
			})
			ui.Field(c, msgs.GatewayKeyRate(), func() { ui.TextInput(c, &a.gwKeyRateText) })
			ui.Field(c, msgs.GatewayKeyExpiry(), func() { ui.TextInput(c, &a.gwKeyExpiry).Placeholder("30d") })
			if a.gwKeyError != "" {
				ui.Text(c, a.gwKeyError).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.gwKeyOpen = false
				}
				if ui.PrimaryButton(c, msgs.GatewayCreateKey()).Clicked() {
					a.createKey(c, msgs)
				}
			})
		})
	})
}

func (a *app) revokeKeyDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	open := a.gwKeyRevoke != nil
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(420).Gap(12).Children(func() {
			ui.Text(c, msgs.GatewayKeyRevokeTitle()).FontSize(17).Bold()
			if a.gwKeyRevoke != nil {
				ui.Text(c, msgs.GatewayKeyRevokeMessage(a.gwKeyRevoke.Name)).FontSize(12.5).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.gwKeyRevoke = nil
				}
				if ui.PrimaryButton(c, msgs.GatewayRevoke()).Clicked() {
					a.revokeKey(c, msgs)
				}
			})
		})
	})
	if !open {
		a.gwKeyRevoke = nil
	}
}

func (a *app) openImportDialog(msgs locales.Messages) {
	a.gwImportOpen = true
	a.gwImportPlan, a.gwImportStats, a.gwImportError = nil, nil, ""
	a.gwImportSelected = map[string]bool{}
	a.gwImportAgent, a.gwImportQuery = msgs.GatewayImportAllAgents(), ""
	if a.usage == nil {
		a.gwImportError = msgs.GatewayDatabaseUnavailable()
		return
	}
	if a.syncManager == nil {
		a.gwImportError = msgs.GatewayConfigUnavailable()
		return
	}
	if profiles, err := a.usage.ListProfiles(context.Background()); err == nil {
		a.gwProfiles = profiles
	}
	if len(a.gwProfiles) == 0 {
		if _, err := a.usage.EnsureDefaultProfile(context.Background()); err == nil {
			a.gwProfiles, _ = a.usage.ListProfiles(context.Background())
		}
	}
	a.gwImportProfile = ""
	for _, profile := range a.gwProfiles {
		if a.gwImportProfile == "" {
			a.gwImportProfile = profile.Name
		}
		if profile.Name == "default" {
			a.gwImportProfile = "default"
			break
		}
	}
	plan, err := importer.Import(context.Background(), a.usage, a.syncManager, a.gwImportProfile, "", true)
	if err != nil {
		a.gwImportError = err.Error()
		return
	}
	a.gwImportPlan = plan
	for _, result := range plan.Results {
		if result.Status == "imported" || result.Status == "existing" {
			a.gwImportSelected[result.Name] = true
		}
	}
}

func importRows(stats *importer.Stats) []importer.Result {
	if stats == nil {
		return nil
	}
	seen := map[string]bool{}
	rows := make([]importer.Result, 0, len(stats.Results))
	for _, result := range stats.Results {
		if result.Status == "duplicate" || seen[result.Name] {
			continue
		}
		seen[result.Name] = true
		rows = append(rows, result)
	}
	return rows
}

type importAgent struct {
	ID    string
	Label string
}

func (a *app) importAgentLabel(id string) string {
	if a.syncManager != nil {
		for _, status := range a.syncManager.Statuses() {
			if status.Client == id {
				return status.Label
			}
		}
	}
	return id
}

func (a *app) importAgents(plan *importer.Stats) []importAgent {
	labels := map[string]string{}
	if a.syncManager != nil {
		for _, status := range a.syncManager.Statuses() {
			labels[status.Client] = status.Label
		}
	}
	seen := map[string]bool{}
	var agents []importAgent
	if plan == nil {
		return nil
	}
	for _, result := range plan.Results {
		if result.Client == "" || seen[result.Client] {
			continue
		}
		seen[result.Client] = true
		label := labels[result.Client]
		if label == "" {
			label = result.Client
		}
		agents = append(agents, importAgent{ID: result.Client, Label: label})
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Label < agents[j].Label })
	return agents
}

func (a *app) importDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.gwImportOpen, func() {
		ui.Column(c).Width(680).Gap(12).Children(func() {
			ui.Text(c, msgs.GatewayImportTitle()).FontSize(18).Bold()
			if a.gwImportStats != nil {
				stats := a.gwImportStats
				if stats.Imported+stats.Existing+stats.Duplicate+stats.Invalid == 0 {
					ui.Text(c, msgs.GatewayImportEmpty()).FontSize(12.5).TextColor(t.TextMuted)
				} else {
					ui.Text(c, msgs.GatewayImportSummary(stats.Imported, stats.Existing, stats.Duplicate, stats.Invalid)).
						FontSize(12.5).TextColor(t.TextMuted)
				}
				for _, result := range stats.Results {
					if result.Status == "invalid" {
						ui.Textf(c, "%s (%s): %s", result.Name, result.Client, result.Error).
							FontSize(11).TextColor(t.Danger)
					}
				}
				ui.Row(c).Justify(ui.End).Children(func() {
					if ui.PrimaryButton(c, msgs.GatewayImportDone()).Clicked() {
						a.gwImportOpen = false
					}
				})
				return
			}

			ui.Text(c, msgs.GatewayImportHint()).FontSize(12.5).TextColor(t.TextMuted)
			ui.Form(c, func() {
				ui.Field(c, msgs.GatewayProfile(), func() {
					names := make([]string, len(a.gwProfiles))
					for i, profile := range a.gwProfiles {
						names[i] = profile.Name
					}
					ui.Select(c, &a.gwImportProfile, names)
				})
				ui.Field(c, msgs.GatewayImportAgent(), func() {
					agents := a.importAgents(a.gwImportPlan)
					labels := make([]string, 0, len(agents)+1)
					labels = append(labels, msgs.GatewayImportAllAgents())
					for _, agent := range agents {
						labels = append(labels, agent.Label)
					}
					ui.Select(c, &a.gwImportAgent, labels)
				})
			})
			rows := importRows(a.gwImportPlan)
			query := strings.ToLower(strings.TrimSpace(a.gwImportQuery))
			filtered := make([]importer.Result, 0, len(rows))
			allAgents := msgs.GatewayImportAllAgents()
			for _, row := range rows {
				if a.gwImportAgent != "" && a.gwImportAgent != allAgents &&
					a.importAgentLabel(row.Client) != a.gwImportAgent {
					continue
				}
				if query == "" || strings.Contains(strings.ToLower(row.Name), query) || strings.Contains(strings.ToLower(row.Client), query) {
					filtered = append(filtered, row)
				}
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.SearchField(c, &a.gwImportQuery).Placeholder(msgs.ServersSearch()).Grow(1)
				if ui.Button(c, msgs.GatewayImportSelectAll()).Clicked() {
					for _, row := range filtered {
						if row.Status != "invalid" {
							a.gwImportSelected[row.Name] = true
						}
					}
				}
				if ui.Button(c, msgs.GatewayImportSelectNone()).Clicked() {
					for _, row := range filtered {
						a.gwImportSelected[row.Name] = false
					}
				}
			})
			if len(filtered) == 0 {
				ui.Text(c, msgs.GatewayImportEmpty()).FontSize(12).TextColor(t.TextMuted)
			} else {
				listHeight := float32(len(filtered)*42 + 8)
				if listHeight < 96 {
					listHeight = 96
				}
				if listHeight > 250 {
					listHeight = 250
				}
				ui.Scroll(c).Height(listHeight).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, row := range filtered {
							row := row
							checked := a.gwImportSelected[row.Name]
							ui.Column(c).Padding(6, 8).Gap(1).Radius(7).Children(func() {
								ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
									if ui.Checkbox(c, &checked, row.Name).Disabled(row.Status == "invalid").Changed() {
										a.gwImportSelected[row.Name] = checked
									}
									ui.Spacer(c)
									text, color := msgs.GatewayImportNew(), t.Success
									switch row.Status {
									case "existing":
										text, color = msgs.GatewayImportExisting(), t.Accent
									case "invalid":
										text, color = msgs.GatewayImportInvalid(), t.Danger
									}
									ui.Text(c, text).FontSize(10.5).TextColor(color)
								})
								ui.Text(c, a.importAgentLabel(row.Client)).FontSize(10).TextColor(t.TextMuted)
								if row.Error != "" {
									ui.Text(c, row.Error).FontSize(10.5).TextColor(t.Danger)
								}
							})
						}
					})
				})
			}
			selected := 0
			for _, row := range rows {
				if a.gwImportSelected[row.Name] {
					selected++
				}
			}
			ui.Text(c, msgs.GatewayImportSelected(strconv.Itoa(selected))).FontSize(11).TextColor(t.TextMuted)
			if a.gwImportError != "" {
				ui.Text(c, a.gwImportError).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, msgs.Cancel()).Clicked() {
					a.gwImportOpen = false
				}
				if ui.Button(c, msgs.GatewayImportRescan()).Clicked() {
					a.openImportDialog(msgs)
				}
				if ui.PrimaryButton(c, msgs.GatewayImport()).Disabled(selected == 0).Clicked() {
					if a.usage == nil {
						a.gwImportError = msgs.GatewayDatabaseUnavailable()
						return
					}
					stats, err := importer.ImportSelected(context.Background(), a.usage, a.syncManager,
						a.gwImportProfile, "", a.gwImportSelected, false)
					if err != nil {
						a.gwImportError = err.Error()
						return
					}
					a.gwImportStats = stats
					a.gwImportError = ""
					a.gwDirty = true
					a.loadGateway()
				}
			})
		})
	})
}
