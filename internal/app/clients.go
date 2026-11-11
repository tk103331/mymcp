package app

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/tk103331/mymcp/internal/data"
)

//go:embed assets/clients/*.png assets/clients/magpie/*.svg assets/clients/magpie/*.png
var clientIconFS embed.FS

// clientSVGs and clientBitmaps are the vendors' brand icons, decoded once and
// bundled so that the app shows them offline. Magpie's SVG set comes first
// (github.com/yetone/magpie/internal/gui/assets/icons); a client it does not
// have falls back to the icon fetched from the vendor's own site. See
// assets/clients/README.md for the sources.
var (
	clientSVGs    = map[string]*ui.SVG{}
	clientBitmaps = map[string]*ui.Bitmap{}
)

var iconOnlyClients = []string{"goose"}

func loadClientIcons() {
	names := make([]string, 0, len(supportedClients)+len(iconOnlyClients))
	for _, client := range supportedClients {
		names = append(names, client.Name)
	}
	names = append(names, iconOnlyClients...)
	for _, name := range names {
		if raw, err := clientIconFS.ReadFile("assets/clients/magpie/" + name + ".svg"); err == nil {
			svg, err := ui.ParseSVG(raw)
			if err == nil {
				clientSVGs[name] = svg
				continue
			}
			log.Println("client icon", name, ":", err)
		}
		path := "assets/clients/" + name + ".png"
		if _, err := clientIconFS.ReadFile(path); err != nil {
			path = "assets/clients/magpie/" + name + ".png"
		}
		raw, err := clientIconFS.ReadFile(path)
		if err != nil {
			log.Println("client icon", name, ":", err)
			continue
		}
		bitmap, err := ui.DecodeBitmap(raw)
		if err != nil {
			log.Println("client icon", name, ":", err)
			continue
		}
		clientBitmaps[name] = bitmap
	}
}

// SVG returns the client's vector brand icon, or nil when it has none.
func (c *clientDef) SVG() *ui.SVG { return clientSVGs[c.Name] }

// Icon returns the client's bitmap brand icon, or nil when it has none.
func (c *clientDef) Icon() *ui.Bitmap { return clientBitmaps[c.Name] }

// clientDef is an MCP client whose configuration the app can manage.
type clientDef struct {
	Name           string
	Label          string
	ConfigFile     map[string]string // GOOS -> path
	ConfigProperty string
	Generate       func(instances []*data.ServerInstance) map[string]any
	// Preview overrides the JSON form the dialog shows (Codex's TOML).
	Preview func(instances []*data.ServerInstance) string
	// Write overrides the JSON merge that saves the client's file.
	Write func(path string, instances []*data.ServerInstance) error
}

// supportedClients draws the file and entry shapes from Magpie's MCP layer
// (github.com/yetone/magpie/internal/library, targets.go and mcp.go):
// fmtCursor, fmtClaude, fmtDesktop, fmtCodex, fmtGemini, fmtOpenCode,
// fmtCopilot, fmtCline, fmtKiro, fmtOmp (Qoder) and fmtCrush. Claude
// Desktop, Codex and Kiro take commands or HTTP only, so a remote endpoint is
// bridged through mcp-pipe there. Windsurf, Trae, VS Code, LM Studio, Factory
// Droid, ChatWise, CherryStudio and DeepChat are not in Magpie's MCP layer;
// their vendor formats are kept.
var supportedClients = []*clientDef{
	{
		Name:           "cursor",
		Label:          "Cursor",
		ConfigFile:     map[string]string{"darwin": "~/.cursor/mcp.json", "linux": "~/.cursor/mcp.json", "windows": "~/.cursor/mcp.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "windsurf",
		Label:          "Windsurf",
		ConfigFile:     map[string]string{"darwin": "~/.codeium/windsurf/mcp_config.json", "linux": "~/.codeium/windsurf/mcp_config.json", "windows": "~/.codeium/windsurf/mcp_config.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"serverUrl": instance.Endpoint}
			})
		},
	},
	{
		Name:           "trae",
		Label:          "Trae",
		ConfigFile:     map[string]string{"darwin": "~/Library/Application Support/Trae/User/mcp.json", "linux": "~/.config/Trae/User/mcp.json", "windows": `%APPDATA%\Trae\User\mcp.json`},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "claudecode",
		Label:          "Claude Code",
		ConfigFile:     map[string]string{"darwin": "~/.claude.json", "linux": "~/.claude.json", "windows": "~/.claude.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "claude",
		Label:          "Claude Desktop",
		ConfigFile:     map[string]string{"darwin": "~/Library/Application Support/Claude/claude_desktop_config.json", "linux": "~/.config/Claude/claude_desktop_config.json", "windows": `%APPDATA%\Claude\claude_desktop_config.json`},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, stdioBridgeEntry)
		},
	},
	{
		Name:           "codex",
		Label:          "Codex",
		ConfigFile:     map[string]string{"darwin": "~/.codex/config.toml", "linux": "~/.codex/config.toml", "windows": "~/.codex/config.toml"},
		ConfigProperty: "mcp_servers",
		Generate:       codexEntries,
		Preview:        codexPreview,
		Write:          writeCodexConfig,
	},
	{
		Name:           "gemini",
		Label:          "Gemini CLI",
		ConfigFile:     map[string]string{"darwin": "~/.gemini/settings.json", "linux": "~/.gemini/settings.json", "windows": "~/.gemini/settings.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "opencode",
		Label:          "OpenCode",
		ConfigFile:     map[string]string{"darwin": "~/.config/opencode/opencode.json", "linux": "~/.config/opencode/opencode.json", "windows": "~/.config/opencode/opencode.json"},
		ConfigProperty: "mcp",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "remote", "url": instance.Endpoint, "enabled": true}
			}, "mcp")
		},
	},
	{
		Name:           "copilot",
		Label:          "GitHub Copilot",
		ConfigFile:     map[string]string{"darwin": "~/.copilot/mcp-config.json", "linux": "~/.copilot/mcp-config.json", "windows": "~/.copilot/mcp-config.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "cline",
		Label:          "Cline",
		ConfigFile:     map[string]string{"darwin": "~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json", "linux": "~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json", "windows": `%APPDATA%\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json`},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"transport": map[string]any{"type": "sse", "url": instance.Endpoint}}
			})
		},
	},
	{
		Name:           "kiro",
		Label:          "Kiro",
		ConfigFile:     map[string]string{"darwin": "~/.kiro/settings/mcp.json", "linux": "~/.kiro/settings/mcp.json", "windows": "~/.kiro/settings/mcp.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, stdioBridgeEntry)
		},
	},
	{
		Name:           "qoder",
		Label:          "Qoder",
		ConfigFile:     map[string]string{"darwin": "~/.qoder/settings.json", "linux": "~/.qoder/settings.json", "windows": "~/.qoder/settings.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "crush",
		Label:          "Crush",
		ConfigFile:     map[string]string{"darwin": "~/.config/crush/crush.json", "linux": "~/.config/crush/crush.json", "windows": `%LOCALAPPDATA%\crush\crush.json`},
		ConfigProperty: "mcp",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "url": instance.Endpoint}
			}, "mcp")
		},
	},
	{
		Name:           "vscode",
		Label:          "VS Code",
		ConfigFile:     map[string]string{"darwin": "~/Library/Application Support/Code/User/mcp.json", "linux": "~/.config/Code/User/mcp.json", "windows": `%APPDATA%\Code\User\mcp.json`},
		ConfigProperty: "servers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "url": instance.Endpoint}
			}, "servers")
		},
	},
	{
		Name:           "lmstudio",
		Label:          "LM Studio",
		ConfigFile:     map[string]string{"darwin": "~/.lmstudio/mcp.json", "linux": "~/.lmstudio/mcp.json", "windows": "~/.lmstudio/mcp.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "droid",
		Label:          "Factory Droid",
		ConfigFile:     map[string]string{"darwin": "~/.factory/mcp.json", "linux": "~/.factory/mcp.json", "windows": "~/.factory/mcp.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "zed",
		Label:          "Zed",
		ConfigFile:     map[string]string{"darwin": "~/.config/zed/settings.json", "linux": "~/.config/zed/settings.json", "windows": `%APPDATA%\Zed\settings.json`},
		ConfigProperty: "context_servers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, stdioBridgeEntry, "context_servers")
		},
	},
	{
		Name:           "kimi",
		Label:          "Kimi Code",
		ConfigFile:     map[string]string{"darwin": "~/.kimi/mcp.json", "linux": "~/.kimi/mcp.json", "windows": "~/.kimi/mcp.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"url": instance.Endpoint, "transport": "sse"}
			})
		},
	},
	{
		Name:           "codebuddy",
		Label:          "CodeBuddy",
		ConfigFile:     map[string]string{"darwin": "~/.codebuddy.json", "linux": "~/.codebuddy.json", "windows": "~/.codebuddy.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "url": instance.Endpoint}
			})
		},
	},
	{
		Name:           "antigravity",
		Label:          "Antigravity",
		ConfigFile:     map[string]string{"darwin": "~/.gemini/config/mcp_config.json", "linux": "~/.gemini/config/mcp_config.json", "windows": "~/.gemini/config/mcp_config.json"},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"type": "sse", "serverUrl": instance.Endpoint}
			})
		},
	},
	{
		Name:  "chatwise",
		Label: "ChatWise",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			return serversBy(instances, func(instance *data.ServerInstance) map[string]any {
				return map[string]any{"url": instance.Endpoint}
			})
		},
	},
	{
		Name:  "cherrystudio",
		Label: "CherryStudio",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			servers := map[string]any{}
			for _, instance := range instances {
				if instance.Config == nil {
					continue
				}
				servers[instance.Config.Name] = map[string]any{
					"name":     instance.Config.Name,
					"type":     "sse",
					"url":      instance.Endpoint,
					"isActive": true,
				}
			}
			return map[string]any{"mcpServers": servers}
		},
	},
	{
		Name:           "deepchat",
		Label:          "DeepChat",
		ConfigFile:     map[string]string{"darwin": "~/Library/Application Support/DeepChat/mcp-settings.json", "linux": "~/.config/DeepChat/mcp-settings.json", "windows": `%APPDATA%\DeepChat/mcp-settings.json`},
		ConfigProperty: "mcpServers",
		Generate: func(instances []*data.ServerInstance) map[string]any {
			servers := map[string]any{}
			for _, instance := range instances {
				if instance.Config == nil {
					continue
				}
				servers[instance.Config.Name+"-"+instance.Config.ID] = map[string]any{
					"type":         "sse",
					"icons":        "🛠️",
					"url":          instance.Endpoint,
					"descriptions": "From MyMCP",
					"disable":      false,
					"autoApprove":  []string{"all"},
				}
			}
			return map[string]any{"mcpServers": servers}
		},
	},
}

func serversBy(instances []*data.ServerInstance, entry func(*data.ServerInstance) map[string]any, keys ...string) map[string]any {
	key := "mcpServers"
	if len(keys) > 0 {
		key = keys[0]
	}
	servers := map[string]any{}
	for _, instance := range instances {
		if instance.Config == nil {
			continue
		}
		servers[instance.Config.Name] = entry(instance)
	}
	return map[string]any{key: servers}
}

// stdioBridgeEntry runs the pipe tool so an agent that takes commands alone
// reaches the SSE endpoint.
func stdioBridgeEntry(instance *data.ServerInstance) map[string]any {
	return map[string]any{
		"command": "mcp-pipe",
		"args":    []string{"--mode", "sse2stdio", "--url", instance.Endpoint},
	}
}

// codexEntries is Codex's [mcp_servers.<name>] tables, each a command since
// Codex reaches no SSE server itself (Magpie's fmtCodex).
func codexEntries(instances []*data.ServerInstance) map[string]any {
	servers := map[string]any{}
	for _, instance := range instances {
		if instance.Config == nil {
			continue
		}
		servers[instance.Config.Name] = map[string]any{
			"command": "mcp-pipe",
			"args":    []string{"--mode", "sse2stdio", "--url", instance.Endpoint},
		}
	}
	return map[string]any{"mcp_servers": servers}
}

func codexPreview(instances []*data.ServerInstance) string {
	raw, err := toml.Marshal(codexEntries(instances))
	if err != nil {
		return ""
	}
	return string(raw)
}

func writeCodexConfig(path string, instances []*data.ServerInstance) error {
	document := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		if err := toml.Unmarshal(raw, &document); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	servers, _ := document["mcp_servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	added, _ := codexEntries(instances)["mcp_servers"].(map[string]any)
	for name, entry := range added {
		servers[name] = entry
	}
	document["mcp_servers"] = servers
	raw, err := toml.Marshal(document)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func clientByName(name string) *clientDef {
	for _, client := range supportedClients {
		if client.Name == name {
			return client
		}
	}
	return nil
}

// platformKey names the current platform as the client paths do.
func platformKey() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "darwin"
	default:
		return "linux"
	}
}

// ConfigPath returns the client's configuration file on this platform, "" when it has none.
func (c *clientDef) ConfigPath() string {
	if c.ConfigFile == nil {
		return ""
	}
	return c.ConfigFile[platformKey()]
}

// Config returns the configuration the client expects for the instances.
func (c *clientDef) Config(instances []*data.ServerInstance) map[string]any {
	if c.Generate == nil {
		return nil
	}
	return c.Generate(instances)
}

// PreviewText is the configuration as the dialog shows it.
func (c *clientDef) PreviewText(instances []*data.ServerInstance) string {
	if c.Preview != nil {
		return c.Preview(instances)
	}
	raw, err := json.MarshalIndent(c.Config(instances), "", "  ")
	if err != nil {
		return ""
	}
	return string(raw)
}

func expandHome(path string) string {
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "%APPDATA%", os.Getenv("APPDATA"))
	path = strings.ReplaceAll(path, "%LOCALAPPDATA%", os.Getenv("LOCALAPPDATA"))
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// saveManagedClientConfig merges the generated configuration into the client's file.
func saveManagedClientConfig(client *clientDef, instances []*data.ServerInstance) error {
	path := client.ConfigPath()
	if path == "" {
		return fmt.Errorf("%s has no configuration file", client.Label)
	}
	path = expandHome(path)
	if path == "" || strings.Contains(path, "%") {
		return fmt.Errorf("%s is not supported on this platform", client.Label)
	}

	if client.Write != nil {
		return client.Write(path, instances)
	}
	document := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &document); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	generated := client.Config(instances)
	if client.ConfigProperty != "" {
		section, _ := document[client.ConfigProperty].(map[string]any)
		if section == nil {
			section = map[string]any{}
		}
		if add, ok := generated[client.ConfigProperty].(map[string]any); ok {
			for key, value := range add {
				section[key] = value
			}
		}
		document[client.ConfigProperty] = section
	} else {
		for key, value := range generated {
			document[key] = value
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
