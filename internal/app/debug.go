package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tk103331/mymcp/internal/gateway"
	"github.com/tk103331/mymcp/locales"
)

func (a *app) loadDebug() {
	a.debugError = ""
	if a.usage != nil {
		if profiles, err := a.usage.ListProfiles(context.Background()); err == nil {
			a.gwProfiles = profiles
		}
	}
	profiles := make([]string, 0, len(a.gwProfiles))
	for _, profile := range a.gwProfiles {
		profiles = append(profiles, profile.Name)
	}
	if len(profiles) == 0 {
		a.debugTools, a.debugSelected, a.debugResult = nil, "", ""
		return
	}
	known := false
	for _, profile := range profiles {
		if profile == a.debugProfile {
			known = true
			break
		}
	}
	if !known {
		a.debugProfile = profiles[0]
		for _, profile := range profiles {
			if profile == "default" {
				a.debugProfile = "default"
				break
			}
		}
		a.debugSelected, a.debugArgs, a.debugResult, a.debugError = "", "{}", "", ""
	}
	a.loadDebugTools()
}

func (a *app) loadDebugTools() {
	a.debugTools = nil
	if !a.gwRunning || a.gwManager == nil {
		return
	}
	tools, err := a.gwManager.Tools(a.debugProfile)
	if err != nil {
		a.debugError = err.Error()
		return
	}
	a.debugTools = tools
	if a.debugSelected == "" {
		return
	}
	for _, tool := range tools {
		if tool.Name == a.debugSelected {
			return
		}
	}
	a.debugSelected, a.debugArgs, a.debugResult, a.debugError = "", "{}", "", ""
}

func (a *app) filteredDebugTools() []gateway.ToolInfo {
	query := strings.ToLower(strings.TrimSpace(a.debugQuery))
	if query == "" {
		return a.debugTools
	}
	out := make([]gateway.ToolInfo, 0, len(a.debugTools))
	for _, tool := range a.debugTools {
		if strings.Contains(strings.ToLower(tool.Name), query) ||
			strings.Contains(strings.ToLower(tool.Server), query) ||
			strings.Contains(strings.ToLower(tool.Description), query) {
			out = append(out, tool)
		}
	}
	return out
}

type debugToolGroup struct {
	Server string
	Tools  []gateway.ToolInfo
}

func groupDebugTools(tools []gateway.ToolInfo) []debugToolGroup {
	var groups []debugToolGroup
	for _, tool := range tools {
		if len(groups) == 0 || groups[len(groups)-1].Server != tool.Server {
			groups = append(groups, debugToolGroup{Server: tool.Server})
		}
		groups[len(groups)-1].Tools = append(groups[len(groups)-1].Tools, tool)
	}
	return groups
}

func (a *app) debugTool() (gateway.ToolInfo, bool) {
	for _, tool := range a.debugTools {
		if tool.Name == a.debugSelected {
			return tool, true
		}
	}
	return gateway.ToolInfo{}, false
}

func (a *app) selectDebugTool(name string) {
	a.debugSelected = name
	a.debugArgs = "{}"
	a.debugResult = ""
	a.debugError = ""
	a.debugLatency = 0
}

func (a *app) callDebugTool(msgs locales.Messages) {
	manager := a.gwManager
	if manager == nil || !a.gwRunning {
		a.debugError = msgs.DebugNoGateway()
		return
	}
	tool, ok := a.debugTool()
	if !ok {
		a.debugError = msgs.DebugNoSelection()
		return
	}
	text := strings.TrimSpace(a.debugArgs)
	if text == "" {
		text = "{}"
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		a.debugError = msgs.DebugInvalidArgs() + " " + err.Error()
		return
	}
	args, ok := parsed.(map[string]any)
	if !ok {
		a.debugError = msgs.DebugInvalidArgs()
		return
	}
	profile, name := a.debugProfile, tool.Name
	a.debugBusy, a.debugError, a.debugResult, a.debugLatency = true, "", "", 0
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		started := time.Now()
		result, err := manager.CallTool(ctx, profile, name, args)
		latency := time.Since(started).Milliseconds()
		apply := func() {
			a.debugBusy, a.debugLatency = false, latency
			if err != nil {
				a.debugError = err.Error()
				return
			}
			a.debugError = ""
			a.debugResult = debugResultText(result)
		}
		if a.win != nil {
			a.win.Update(apply)
		} else {
			apply()
		}
	}()
}

func debugResultText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprint(result)
	}
	return string(raw)
}

func debugSchemaText(raw json.RawMessage) string {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return "{}"
	}
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func (a *app) debugPage(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.Column(c).Fill().Padding(30, 28, 26, 28).Gap(16).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, msgs.DebugTitle()).FontSize(24).Bold()
				ui.Text(c, msgs.DebugSubtitle()).FontSize(12.5).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			profiles := make([]string, 0, len(a.gwProfiles))
			for _, profile := range a.gwProfiles {
				profiles = append(profiles, profile.Name)
			}
			if len(profiles) == 0 {
				profiles = []string{"default"}
			}
			ui.Text(c, msgs.DebugProfile()).FontSize(11.5).TextColor(t.TextMuted)
			if ui.Select(c, &a.debugProfile, profiles).Width(150).Changed() {
				a.debugSelected, a.debugResult, a.debugError = "", "", ""
				a.loadDebugTools()
			}
			if ui.Button(c, msgs.GatewayRefresh()).Clicked() {
				a.loadDebug()
			}
		})

		if !a.gwRunning {
			ui.Column(c).FillWidth().Padding(30).Gap(10).Radius(14).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, msgs.DebugNoGateway()).FontSize(14).Bold()
				if a.gwError != "" {
					ui.Text(c, a.gwError).FontSize(12).TextColor(t.Danger)
				}
				if ui.PrimaryButton(c, msgs.DebugStartGateway()).Clicked() {
					a.startGateway(msgs)
				}
			})
			return
		}

		tools := a.filteredDebugTools()
		ui.Row(c).Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
			list := ui.Column(c).Width(340).Shrink(0).Padding(14).Gap(10).Radius(14).
				Background(t.Surface).Border(1, t.Border)
			list.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, msgs.DebugTools()).FontSize(14).Bold()
					ui.Spacer(c)
					ui.Textf(c, "%d", len(tools)).FontSize(11).TextColor(t.TextMuted)
				})
				ui.SearchField(c, &a.debugQuery).Placeholder(msgs.DebugSearchTools()).
					Label(msgs.DebugSearchTools())
				if len(tools) == 0 {
					ui.Text(c, msgs.DebugNoTools()).FontSize(12).TextColor(t.TextMuted)
					return
				}
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, group := range groupDebugTools(tools) {
							group := group
							ui.Row(c).Padding(6, 4).AlignItems(ui.Center).Gap(6).Children(func() {
								ui.Text(c, group.Server).FontSize(10).Bold().TextColor(t.TextMuted)
								ui.Spacer(c)
								ui.Textf(c, "%d", len(group.Tools)).FontSize(10).TextColor(t.TextMuted)
							})
							for _, tool := range group.Tools {
								tool := tool
								row := ui.ButtonBase(c).Padding(7, 8).Gap(3).Radius(7).Justify(ui.Start)
								if a.debugSelected == tool.Name {
									row.Background(t.Accent.Alpha(0.12))
								}
								row.Children(func() {
									ui.Text(c, tool.Tool).FontSize(11.5).Bold().SingleLine()
									if tool.Description != "" {
										ui.Text(c, tool.Description).FontSize(9.5).TextColor(t.TextMuted).MaxLines(1)
									}
								})
								if row.Clicked() {
									a.selectDebugTool(tool.Name)
								}
							}
						}
					})
				})
			})

			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(14).Children(func() {
					tool, ok := a.debugTool()
					if !ok {
						ui.Column(c).FillWidth().Padding(30).Gap(8).Radius(14).
							Background(t.Surface).Border(1, t.Border).Children(func() {
							ui.Text(c, msgs.DebugSelectTool()).FontSize(12.5).TextColor(t.TextMuted)
						})
						return
					}
					header := ui.Column(c).Padding(16).Gap(4).Radius(14).
						Background(t.Surface).Border(1, t.Border)
					header.Children(func() {
						ui.Text(c, tool.Tool).FontSize(16).Bold().SingleLine()
						ui.Text(c, tool.Server).FontSize(11).TextColor(t.TextMuted)
						if tool.Description != "" {
							ui.Text(c, tool.Description).FontSize(12).TextColor(t.TextMuted)
						}
					})

					schema := debugSchemaText(tool.InputSchema)
					ui.Column(c).Padding(16).Gap(8).Radius(14).
						Background(t.Surface).Border(1, t.Border).Children(func() {
						ui.Text(c, msgs.DebugSchema()).FontSize(13).Bold()
						ui.TextArea(c, &schema).ReadOnly(true).Height(150).FontSize(11)
					})

					ui.Column(c).Padding(16).Gap(8).Radius(14).
						Background(t.Surface).Border(1, t.Border).Children(func() {
						ui.Text(c, msgs.DebugArguments()).FontSize(13).Bold()
						ui.TextArea(c, &a.debugArgs).Height(130).FontSize(11.5)
						ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
							label := msgs.DebugCall()
							if a.debugBusy {
								label = msgs.DebugCalling()
							}
							if ui.PrimaryButton(c, label).Disabled(a.debugBusy).Clicked() {
								a.callDebugTool(msgs)
							}
							if a.debugLatency > 0 {
								ui.Textf(c, "%s: %d ms", msgs.DebugLatency(), a.debugLatency).
									FontSize(11.5).TextColor(t.TextMuted)
							}
						})
						if a.debugError != "" {
							ui.Text(c, a.debugError).FontSize(11.5).TextColor(t.Danger)
						}
					})

					if a.debugResult != "" {
						result := a.debugResult
						ui.Column(c).Padding(16).Gap(8).Radius(14).
							Background(t.Surface).Border(1, t.Border).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Children(func() {
								ui.Text(c, msgs.DebugResult()).FontSize(13).Bold()
								ui.Spacer(c)
								if ui.Button(c, "").Label(msgs.Copy()).Tooltip(msgs.Copy()).
									Padding(3, 6).Children(func() {
									ui.Icon(c, iconCopy).Size(12, 12)
								}).Clicked() {
									mygo.Clipboard.WriteText(result)
									c.Toast(msgs.Copied())
								}
							})
							ui.TextArea(c, &result).ReadOnly(true).Height(260).FontSize(11)
						})
					}
				})
			})
		})
	})
}
