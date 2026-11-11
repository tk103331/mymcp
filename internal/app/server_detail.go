package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tk103331/mymcp/internal/gateway"
	"github.com/tk103331/mymcp/internal/store"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) openServerDetail(server *store.Server, msgs locales.Messages) {
	a.gwDetail = server
	a.gwDetailOpen = true
	a.gwDetailTab = 0
	a.gwDetailData = nil
	a.gwDetailError = ""
	a.gwDetailProfile = ""
	profiles := a.gwServerProfiles[server.Name]
	if len(profiles) > 0 {
		a.gwDetailProfile = profiles[0]
	}
	if a.gwDetailProfile == "" {
		a.gwDetailProfile = "default"
	}
	a.loadServerDetail(msgs)
}

func (a *app) loadServerDetail(msgs locales.Messages) {
	if a.gwDetail == nil {
		return
	}
	if !a.gwRunning || a.gwManager == nil {
		a.gwDetailLoading = false
		a.gwDetailData = nil
		a.gwDetailError = msgs.ServerDetailOffline()
		return
	}
	profile := a.gwDetailProfile
	if profile == "" {
		profile = "default"
	}
	name := a.gwDetail.Name
	a.gwDetailSeq++
	seq := a.gwDetailSeq
	a.gwDetailLoading, a.gwDetailData, a.gwDetailError = true, nil, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		detail, err := a.gwManager.ServerDetail(ctx, profile, name)
		apply := func() {
			if !a.gwDetailOpen || a.gwDetailSeq != seq ||
				a.gwDetail == nil || a.gwDetail.Name != name || a.gwDetailProfile != profile {
				return
			}
			a.gwDetailLoading = false
			if err != nil {
				a.gwDetailError = err.Error()
				return
			}
			a.gwDetailData = detail
		}
		if a.win != nil {
			a.win.Update(apply)
		} else {
			apply()
		}
	}()
}

func (a *app) serverDetailDialog(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Modal(c, &a.gwDetailOpen, func() {
		server := a.gwDetail
		if server == nil {
			return
		}
		profiles := append([]string(nil), a.gwServerProfiles[server.Name]...)
		if len(profiles) == 0 {
			profiles = []string{"default"}
		}
		known := false
		for _, profile := range profiles {
			if profile == a.gwDetailProfile {
				known = true
				break
			}
		}
		if !known {
			a.gwDetailProfile = profiles[0]
		}

		ui.Column(c).Width(780).Gap(12).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, server.Name).FontSize(18).Bold().SingleLine()
					ui.Text(c, serverTarget(server)).FontSize(11).TextColor(t.TextMuted).SingleLine()
				})
				if len(profiles) > 1 {
					ui.Text(c, msgs.GatewayProfile()).FontSize(11).TextColor(t.TextMuted)
					if ui.Select(c, &a.gwDetailProfile, profiles).Width(150).Changed() {
						a.gwDetailTab = 0
						a.loadServerDetail(msgs)
					}
				}
				if ui.Button(c, msgs.GatewayRefresh()).Clicked() {
					a.loadServerDetail(msgs)
				}
			})

			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				a.statusBadge(c, t, server.Transport, t.Accent)
				a.statusBadge(c, t, strings.Join(profiles, ", "), t.TextMuted)
				if server.Enabled {
					a.statusBadge(c, t, msgs.GatewayEnabled(), t.Success)
				} else {
					a.statusBadge(c, t, msgs.GatewayDisabled(), t.TextMuted)
				}
				if a.gwServerErrors[server.Name] != "" {
					a.statusBadge(c, t, msgs.StatusError(), t.Danger)
				}
			})
			if a.gwServerErrors[server.Name] != "" {
				ui.Text(c, a.gwServerErrors[server.Name]).FontSize(11).TextColor(t.Danger)
			}

			config := ui.Column(c).Padding(16).Gap(10).Radius(14).Background(t.Surface).Border(1, t.Border)
			config.Children(func() {
				ui.Text(c, msgs.GatewayBasics()).FontSize(13).Bold()
				if server.Transport == "stdio" {
					serverDetailValue(c, t, msgs.GatewayCommand(), server.Command)
					serverDetailValue(c, t, msgs.GatewayArguments(), strings.Join(server.Args, " "))
					serverDetailValue(c, t, msgs.GatewayEnvironment(), mapLines(server.Env))
				} else {
					serverDetailValue(c, t, msgs.GatewayURL(), server.URL)
					serverDetailValue(c, t, msgs.GatewayHeaders(), mapLines(server.Headers))
				}
			})

			live := ui.Column(c).Grow(1).Padding(16).Gap(10).Radius(14).
				Background(t.Surface).Border(1, t.Border)
			live.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, msgs.ServerCapabilities()).FontSize(13).Bold()
					ui.Spacer(c)
					if a.gwDetailLoading {
						ui.Text(c, msgs.ServerDetailLoading()).FontSize(11).TextColor(t.TextMuted)
					} else if detail := a.gwDetailData; detail != nil && detail.Info.Name != "" {
						ui.Textf(c, "%s %s", detail.Info.Name, detail.Info.Version).
							FontSize(10.5).TextColor(t.TextMuted)
					}
				})
				switch {
				case a.gwDetailLoading:
					ui.Text(c, msgs.ServerDetailLoading()).FontSize(12).TextColor(t.TextMuted)
				case a.gwDetailError != "":
					ui.Text(c, a.gwDetailError).FontSize(12).TextColor(t.Danger)
				case a.gwDetailData == nil:
					ui.Text(c, msgs.ServerDetailOffline()).FontSize(12).TextColor(t.TextMuted)
				default:
					a.serverCapabilities(c, t, msgs, a.gwDetailData)
				}
			})

			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c, msgs.Close()).Clicked() {
					a.gwDetailOpen = false
				}
			})
		})
	})
	if !a.gwDetailOpen {
		a.gwDetail = nil
		a.gwDetailData = nil
		a.gwDetailError = ""
		a.gwDetailLoading = false
		a.gwDetailSeq++
	}
}

func (a *app) serverCapabilities(c *ui.Context, t *ui.Theme, msgs locales.Messages, detail *gateway.ServerDetail) {
	if detail.Instructions != "" {
		ui.Text(c, detail.Instructions).FontSize(11).TextColor(t.TextMuted).MaxLines(4)
	}
	if a.gwDetailTab < 0 || a.gwDetailTab > 2 {
		a.gwDetailTab = 0
	}
	ui.Tabs(c, &a.gwDetailTab,
		fmt.Sprintf("%s (%d)", msgs.DebugTools(), len(detail.Tools)),
		fmt.Sprintf("%s (%d)", msgs.ServerPrompts(), len(detail.Prompts)),
		fmt.Sprintf("%s (%d)", msgs.ServerResources(), len(detail.Resources)+len(detail.ResourceTemplates)),
	)
	items := len(detail.Tools)
	switch a.gwDetailTab {
	case 1:
		items = len(detail.Prompts)
	case 2:
		items = len(detail.Resources) + len(detail.ResourceTemplates)
	}
	height := float32(items*52 + 12)
	if height < 120 {
		height = 120
	}
	if height > 330 {
		height = 330
	}
	ui.Scroll(c).Height(height).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			switch a.gwDetailTab {
			case 1:
				serverDetailPrompts(c, t, msgs, detail)
			case 2:
				serverDetailResources(c, t, msgs, detail)
			default:
				serverDetailTools(c, t, msgs, detail)
			}
		})
	})
}

func serverDetailTools(c *ui.Context, t *ui.Theme, msgs locales.Messages, detail *gateway.ServerDetail) {
	if !detail.ToolsSupported {
		ui.Text(c, msgs.ServerUnsupported()).FontSize(12).TextColor(t.TextMuted)
		return
	}
	if len(detail.Tools) == 0 {
		ui.Text(c, msgs.ServerNoTools()).FontSize(12).TextColor(t.TextMuted)
		return
	}
	for i, tool := range detail.Tools {
		if i > 0 {
			ui.Divider(c)
		}
		tool := tool
		ui.Column(c).Padding(5, 6).Gap(2).Children(func() {
			ui.Text(c, tool.Name).FontSize(12).Bold().SingleLine()
			if tool.Description != "" {
				ui.Text(c, tool.Description).FontSize(10.5).TextColor(t.TextMuted).MaxLines(2)
			}
			if params := toolParameters(tool); params != "" {
				ui.Text(c, msgs.Parameters()+": "+params).FontSize(10).TextColor(t.TextMuted)
			}
		})
	}
}

func serverDetailPrompts(c *ui.Context, t *ui.Theme, msgs locales.Messages, detail *gateway.ServerDetail) {
	if !detail.PromptsSupported {
		ui.Text(c, msgs.ServerUnsupported()).FontSize(12).TextColor(t.TextMuted)
		return
	}
	if err := detail.Errors["prompts"]; err != "" {
		ui.Text(c, err).FontSize(11).TextColor(t.Danger)
	}
	if len(detail.Prompts) == 0 {
		ui.Text(c, msgs.ServerNoPrompts()).FontSize(12).TextColor(t.TextMuted)
		return
	}
	for i, prompt := range detail.Prompts {
		if i > 0 {
			ui.Divider(c)
		}
		prompt := prompt
		ui.Column(c).Padding(5, 6).Gap(2).Children(func() {
			ui.Text(c, prompt.Name).FontSize(12).Bold().SingleLine()
			if prompt.Description != "" {
				ui.Text(c, prompt.Description).FontSize(10.5).TextColor(t.TextMuted).MaxLines(2)
			}
			if args := promptArguments(prompt); args != "" {
				ui.Text(c, msgs.Parameters()+": "+args).FontSize(10).TextColor(t.TextMuted)
			}
		})
	}
}

func serverDetailResources(c *ui.Context, t *ui.Theme, msgs locales.Messages, detail *gateway.ServerDetail) {
	if !detail.ResourcesSupported {
		ui.Text(c, msgs.ServerUnsupported()).FontSize(12).TextColor(t.TextMuted)
		return
	}
	if err := detail.Errors["resources"]; err != "" {
		ui.Text(c, err).FontSize(11).TextColor(t.Danger)
	}
	if err := detail.Errors["resourceTemplates"]; err != "" {
		ui.Text(c, err).FontSize(11).TextColor(t.Danger)
	}
	if len(detail.Resources) == 0 && len(detail.ResourceTemplates) == 0 {
		ui.Text(c, msgs.ServerNoResources()).FontSize(12).TextColor(t.TextMuted)
		return
	}
	if len(detail.Resources) > 0 {
		ui.Text(c, msgs.ServerResources()).FontSize(10.5).Bold().TextColor(t.TextMuted)
		for i, resource := range detail.Resources {
			if i > 0 {
				ui.Divider(c)
			}
			resource := resource
			ui.Column(c).Padding(5, 6).Gap(2).Children(func() {
				ui.Text(c, resource.Name).FontSize(12).Bold().SingleLine()
				meta := resource.URI
				if resource.MIMEType != "" {
					meta += " · " + resource.MIMEType
				}
				ui.Text(c, meta).FontSize(10).TextColor(t.TextMuted).SingleLine()
				if resource.Description != "" {
					ui.Text(c, resource.Description).FontSize(10.5).TextColor(t.TextMuted).MaxLines(2)
				}
			})
		}
	}
	if len(detail.ResourceTemplates) > 0 {
		ui.Text(c, msgs.ServerResourceTemplates()).FontSize(10.5).Bold().TextColor(t.TextMuted)
		for i, template := range detail.ResourceTemplates {
			if i > 0 {
				ui.Divider(c)
			}
			template := template
			ui.Column(c).Padding(5, 6).Gap(2).Children(func() {
				ui.Text(c, template.Name).FontSize(12).Bold().SingleLine()
				uri := ""
				if template.URITemplate != nil && template.URITemplate.Template != nil {
					uri = template.URITemplate.Raw()
				}
				if uri != "" {
					ui.Text(c, uri).FontSize(10).TextColor(t.TextMuted).SingleLine()
				}
				if template.Description != "" {
					ui.Text(c, template.Description).FontSize(10.5).TextColor(t.TextMuted).MaxLines(2)
				}
			})
		}
	}
}

func serverDetailValue(c *ui.Context, t *ui.Theme, label, value string) {
	if strings.TrimSpace(value) == "" {
		value = "—"
	}
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, label).FontSize(10.5).TextColor(t.TextMuted)
		ui.Text(c, value).FontSize(11.5)
	})
}

func toolParameters(tool mcp.Tool) string {
	if len(tool.InputSchema.Properties) == 0 {
		return ""
	}
	required := map[string]bool{}
	for _, name := range tool.InputSchema.Required {
		required[name] = true
	}
	names := make([]string, 0, len(tool.InputSchema.Properties))
	for name := range tool.InputSchema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return markRequired(names, required)
}

func promptArguments(prompt mcp.Prompt) string {
	names := make([]string, 0, len(prompt.Arguments))
	required := map[string]bool{}
	for _, arg := range prompt.Arguments {
		names = append(names, arg.Name)
		if arg.Required {
			required[arg.Name] = true
		}
	}
	return markRequired(names, required)
}

func markRequired(names []string, required map[string]bool) string {
	for i, name := range names {
		if required[name] {
			names[i] = name + "*"
		}
	}
	return strings.Join(names, ", ")
}
