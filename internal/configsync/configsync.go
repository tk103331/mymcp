// Package configsync writes one router entry into the native configuration
// files of MCP clients, and removes only the entries it wrote.
package configsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tidwall/jsonc"
	"github.com/tk103331/mymcp/internal/plugin"
	"gopkg.in/yaml.v3"
)

// Kind is how a client's configuration file is written.
type Kind int

const (
	// JSON is a JSON object with a root section of server entries.
	JSON Kind = iota
	// CodexTOML is Codex's config.toml with [mcp_servers.*] tables.
	CodexTOML
	// YAML is Goose's config.yaml, a root map of sections.
	YAML
)

// Client is one supported MCP client.
type Client struct {
	ID    string
	Label string
	Kind  Kind
	Root  string
	Paths map[string]string
	Entry func(endpoint string) map[string]any
	// Plugin is a JavaScript adapter; its render function owns the whole
	// file, so Kind, Root and Entry are unused.
	Plugin *plugin.Plugin
}

// Clients lists every client configsync can write.
var Clients = []*Client{
	{
		ID: "cursor", Label: "Cursor", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.cursor/mcp.json", "linux": "~/.cursor/mcp.json", "windows": "~/.cursor/mcp.json"},
		Entry: func(endpoint string) map[string]any { return map[string]any{"url": endpoint} },
	},
	{
		ID: "windsurf", Label: "Windsurf", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.codeium/windsurf/mcp_config.json", "linux": "~/.codeium/windsurf/mcp_config.json", "windows": "~/.codeium/windsurf/mcp_config.json"},
		Entry: func(endpoint string) map[string]any { return map[string]any{"serverUrl": endpoint} },
	},
	{
		ID: "trae", Label: "Trae", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/Library/Application Support/Trae/User/mcp.json", "linux": "~/.config/Trae/User/mcp.json", "windows": `%APPDATA%\Trae\User\mcp.json`},
		Entry: func(endpoint string) map[string]any { return map[string]any{"url": endpoint} },
	},
	{
		ID: "claudecode", Label: "Claude Code", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.claude.json", "linux": "~/.claude.json", "windows": "~/.claude.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}
		},
	},
	{
		ID: "claude", Label: "Claude Desktop", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/Library/Application Support/Claude/claude_desktop_config.json", "linux": "~/.config/Claude/claude_desktop_config.json", "windows": `%APPDATA%\Claude\claude_desktop_config.json`},
		Entry: bridgeEntry,
	},
	{
		ID: "codex", Label: "Codex", Kind: CodexTOML, Root: "mcp_servers",
		Paths: map[string]string{"darwin": "~/.codex/config.toml", "linux": "~/.codex/config.toml", "windows": "~/.codex/config.toml"},
		Entry: bridgeEntry,
	},
	{
		ID: "gemini", Label: "Gemini CLI", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.gemini/settings.json", "linux": "~/.gemini/settings.json", "windows": "~/.gemini/settings.json"},
		Entry: func(endpoint string) map[string]any { return map[string]any{"url": endpoint} },
	},
	{
		ID: "opencode", Label: "OpenCode", Kind: JSON, Root: "mcp",
		Paths: map[string]string{"darwin": "~/.config/opencode/opencode.json", "linux": "~/.config/opencode/opencode.json", "windows": "~/.config/opencode/opencode.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "remote", "url": endpoint, "enabled": true}
		},
	},
	{
		ID: "copilot", Label: "GitHub Copilot", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.copilot/mcp-config.json", "linux": "~/.copilot/mcp-config.json", "windows": "~/.copilot/mcp-config.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}
		},
	},
	{
		ID: "cline", Label: "Cline", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json", "linux": "~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json", "windows": `%APPDATA%\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json`},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"transport": map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}}
		},
	},
	{
		ID: "kiro", Label: "Kiro", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.kiro/settings/mcp.json", "linux": "~/.kiro/settings/mcp.json", "windows": "~/.kiro/settings/mcp.json"},
		Entry: bridgeEntry,
	},
	{
		ID: "qoder", Label: "Qoder", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.qoder/settings.json", "linux": "~/.qoder/settings.json", "windows": "~/.qoder/settings.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}
		},
	},
	{
		ID: "crush", Label: "Crush", Kind: JSON, Root: "mcp",
		Paths: map[string]string{"darwin": "~/.config/crush/crush.json", "linux": "~/.config/crush/crush.json", "windows": `%LOCALAPPDATA%\crush\crush.json`},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}
		},
	},
	{
		ID: "vscode", Label: "VS Code", Kind: JSON, Root: "servers",
		Paths: map[string]string{"darwin": "~/Library/Application Support/Code/User/mcp.json", "linux": "~/.config/Code/User/mcp.json", "windows": `%APPDATA%\Code\User\mcp.json`},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}
		},
	},
	{
		ID: "lmstudio", Label: "LM Studio", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.lmstudio/mcp.json", "linux": "~/.lmstudio/mcp.json", "windows": "~/.lmstudio/mcp.json"},
		Entry: func(endpoint string) map[string]any { return map[string]any{"url": endpoint} },
	},
	{
		ID: "droid", Label: "Factory Droid", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.factory/mcp.json", "linux": "~/.factory/mcp.json", "windows": "~/.factory/mcp.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}
		},
	},
	{
		ID: "zed", Label: "Zed", Kind: JSON, Root: "context_servers",
		Paths: map[string]string{"darwin": "~/.config/zed/settings.json", "linux": "~/.config/zed/settings.json", "windows": `%APPDATA%\Zed\settings.json`},
		Entry: bridgeEntry,
	},
	{
		ID: "goose", Label: "Goose", Kind: YAML, Root: "extensions",
		Paths: map[string]string{"darwin": "~/.config/goose/config.yaml", "linux": "~/.config/goose/config.yaml", "windows": `%APPDATA%\Block\goose\config\config.yaml`},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"enabled": true, "name": "mymcp", "type": "sse", "uri": sseEndpoint(endpoint), "timeout": 300}
		},
	},
	{
		ID: "kimi", Label: "Kimi Code", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.kimi/mcp.json", "linux": "~/.kimi/mcp.json", "windows": "~/.kimi/mcp.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"url": sseEndpoint(endpoint), "transport": "sse"}
		},
	},
	{
		ID: "codebuddy", Label: "CodeBuddy", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.codebuddy.json", "linux": "~/.codebuddy.json", "windows": "~/.codebuddy.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "url": sseEndpoint(endpoint)}
		},
	},
	{
		ID: "antigravity", Label: "Antigravity", Kind: JSON, Root: "mcpServers",
		Paths: map[string]string{"darwin": "~/.gemini/config/mcp_config.json", "linux": "~/.gemini/config/mcp_config.json", "windows": "~/.gemini/config/mcp_config.json"},
		Entry: func(endpoint string) map[string]any {
			return map[string]any{"type": "sse", "serverUrl": sseEndpoint(endpoint)}
		},
	},
}

// sseEndpoint returns the gateway's SSE endpoint for a streamable endpoint.
// The gateway serves both; clients that declare the SSE transport need the
// /sse path, not the streamable one.
func sseEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/sse") {
		return endpoint
	}
	return endpoint + "/sse"
}

// bridgeCommand is the mcp-pipe executable written into bridged client
// configs. The desktop app points it at its bundled copy.
var bridgeCommand = "mcp-pipe"

// SetBridgeCommand sets the command used by bridged clients.
func SetBridgeCommand(command string) {
	if command != "" {
		bridgeCommand = command
	}
}

// BridgeCommand returns the command used by bridged clients.
func BridgeCommand() string { return bridgeCommand }

func bridgeEntry(endpoint string) map[string]any {
	return map[string]any{
		"command": bridgeCommand,
		"args":    []string{"--mode", "sse2stdio", "--url", sseEndpoint(endpoint)},
	}
}

// sseBridgeClient is a client whose config bridges through mcp-pipe and
// needs an SSE endpoint.
func sseBridgeClient(id string) bool {
	switch id {
	case "claude", "codex", "kiro", "zed":
		return true
	}
	return false
}

// ClientByID finds a built-in client.
func ClientByID(id string) *Client {
	for _, client := range Clients {
		if client.ID == id {
			return client
		}
	}
	return nil
}

// clients lists built-in and JavaScript clients.
func (m *Manager) clients() []*Client {
	out := make([]*Client, 0, len(Clients)+len(m.plugins))
	out = append(out, Clients...)
	for _, p := range m.plugins {
		out = append(out, &Client{
			ID: p.Manifest.ID, Label: p.Manifest.Label, Paths: p.Manifest.Paths, Plugin: p,
		})
	}
	return out
}

func (m *Manager) clientByID(id string) *Client {
	for _, client := range m.clients() {
		if client.ID == id {
			return client
		}
	}
	return nil
}

// Plugins returns the loaded JavaScript plugins.
func (m *Manager) Plugins() []*plugin.Plugin { return m.plugins }

// ReloadPlugins rescans the plugins directory after an install or removal.
func (m *Manager) ReloadPlugins() []error {
	plugins, errs := plugin.Discover(filepath.Join(m.dir, "plugins"))
	m.plugins, m.pluginErrs = plugins, errs
	return errs
}

// PluginErrors are plugins that failed to load.
func (m *Manager) PluginErrors() []error { return m.pluginErrs }

// Path returns the client's configuration file on this platform.
func (c *Client) Path() string {
	if c.Plugin != nil {
		return c.Plugin.Path()
	}
	path := c.Paths[runtime.GOOS]
	return expandPath(path)
}

// Record is what MyMCP wrote into one client. Router mode records one entry;
// Direct mode records every server entry it wrote.
type Record struct {
	Client    string            `json:"client"`
	File      string            `json:"file"`
	Entry     string            `json:"entry,omitempty"`
	Hash      string            `json:"hash,omitempty"`
	Mode      string            `json:"mode,omitempty"`
	Entries   map[string]string `json:"entries,omitempty"`
	Backup    string            `json:"backup,omitempty"`
	AppliedAt time.Time         `json:"appliedAt"`
}

// DirectServer is one server rendered in Direct mode.
type DirectServer struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// State is the ownership manifest.
type State struct {
	Clients map[string]Record `json:"clients"`
}

// Plan is the diff between a client's file and the router entry.
type Plan struct {
	Client   string         `json:"client"`
	File     string         `json:"file"`
	Entry    string         `json:"entry,omitempty"`
	Mode     string         `json:"mode,omitempty"`
	Exists   bool           `json:"exists"`
	Managed  bool           `json:"managed"`
	Conflict bool           `json:"conflict"`
	Current  any            `json:"current,omitempty"`
	Desired  any            `json:"desired,omitempty"`
	Entries  map[string]any `json:"entries,omitempty"`
	Reason   string         `json:"reason,omitempty"`
}

// Manager applies plans and keeps the ownership manifest.
type Manager struct {
	dir        string
	statePath  string
	state      State
	plugins    []*plugin.Plugin
	pluginErrs []error
}

// New opens the sync state in dir.
func New(dir string) (*Manager, error) {
	m := &Manager{dir: dir, statePath: filepath.Join(dir, "sync.json"), state: State{Clients: map[string]Record{}}}
	m.plugins, m.pluginErrs = plugin.Discover(filepath.Join(dir, "plugins"))
	raw, err := os.ReadFile(m.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m.state); err != nil {
			return nil, err
		}
	}
	if m.state.Clients == nil {
		m.state.Clients = map[string]Record{}
	}
	return m, nil
}

// Plan computes what applying endpoint as entry would change.
func (m *Manager) Plan(clientID, endpoint, entry string) (*Plan, error) {
	client := m.clientByID(clientID)
	if client == nil {
		return nil, fmt.Errorf("unknown client %q", clientID)
	}
	if client.Plugin != nil {
		return m.pluginPlan(client, "router", endpoint, nil, entry)
	}
	path := client.Path()
	doc, err := readDocument(client, path)
	if err != nil {
		return nil, err
	}
	section := sectionOf(doc, client.Root)
	current, exists := section[entry]
	desired := client.Entry(endpoint)
	record, managed := m.state.Clients[client.ID]
	conflict := false
	reason := ""
	if exists && managed && hashAny(current) != record.Hash {
		conflict = true
		reason = "the entry changed since MyMCP wrote it"
	}
	// A previous Direct-mode record may still own native entries. Do not
	// delete one that the user changed without asking for force.
	if !conflict && managed {
		for name, hash := range record.Entries {
			if name == entry {
				continue
			}
			if current, exists := section[name]; exists && hashAny(current) != hash {
				conflict = true
				reason = fmt.Sprintf("entry %q changed since MyMCP wrote it", name)
				break
			}
		}
		if !conflict && record.Entry != "" && record.Entry != entry {
			if current, exists := section[record.Entry]; exists && hashAny(current) != record.Hash {
				conflict = true
				reason = "the previous router entry changed since MyMCP wrote it"
			}
		}
	}
	return &Plan{
		Client: client.ID, File: path, Entry: entry, Exists: exists,
		Managed: managed, Conflict: conflict, Current: current, Desired: desired, Reason: reason,
	}, nil
}

// Apply writes the router entry, backing up the file first.
// PlanDirect compares the client's file with every entry Direct mode would
// write for servers.
func (m *Manager) PlanDirect(clientID string, servers []DirectServer, force bool) (*Plan, error) {
	client := m.clientByID(clientID)
	if client == nil {
		return nil, fmt.Errorf("unknown client %q", clientID)
	}
	if client.Plugin != nil {
		return m.pluginPlan(client, "direct", "", servers, "plugin")
	}
	path := client.Path()
	doc, err := readDocument(client, path)
	if err != nil {
		return nil, err
	}
	section := sectionOf(doc, client.Root)
	desired := map[string]any{}
	for _, server := range servers {
		if hasSecretRef(server) {
			return nil, fmt.Errorf("%s: direct mode cannot carry secret references; use router mode", server.Name)
		}
		if sseBridgeClient(client.ID) && server.Transport == "http" {
			return nil, fmt.Errorf("%s: direct mode needs an SSE server for %s; use router mode", server.Name, client.Label)
		}
		desired[server.Name] = directEntry(client.ID, server)
	}
	record := m.state.Clients[client.ID]
	conflict := false
	reason := ""
	for name, want := range desired {
		current, exists := section[name]
		if !exists {
			continue
		}
		known, managed := record.Entries[name]
		if !managed {
			conflict = true
			reason = fmt.Sprintf("entry %q already exists and is not managed by MyMCP", name)
			break
		}
		if hashAny(current) != known && hashAny(current) != hashAny(want) {
			conflict = true
			reason = fmt.Sprintf("entry %q changed since MyMCP wrote it", name)
			break
		}
	}
	if !conflict {
		for name, hash := range record.Entries {
			if _, keep := desired[name]; keep {
				continue
			}
			if current, exists := section[name]; exists && hashAny(current) != hash {
				conflict = true
				reason = fmt.Sprintf("entry %q changed since MyMCP wrote it", name)
				break
			}
		}
	}
	if !conflict && record.Entry != "" {
		if current, exists := section[record.Entry]; exists && hashAny(current) != record.Hash {
			conflict = true
			reason = "the router entry changed since MyMCP wrote it"
		}
	}
	return &Plan{Client: client.ID, File: path, Mode: "direct", Conflict: conflict,
		Managed: record.Mode == "direct", Desired: desired, Entries: desired, Reason: reason}, nil
}

// ApplyDirect writes every server's native entry, removing entries MyMCP
// wrote before that are no longer in the profile.
func (m *Manager) ApplyDirect(clientID string, servers []DirectServer, force bool) (*Plan, error) {
	plan, err := m.PlanDirect(clientID, servers, force)
	if err != nil {
		return nil, err
	}
	if plan.Conflict && !force {
		return plan, fmt.Errorf("%s: %s; use --force to overwrite", plan.File, plan.Reason)
	}
	client := m.clientByID(clientID)
	if client.Plugin != nil {
		return m.pluginApply(client, plan)
	}
	doc, err := readDocument(client, plan.File)
	if err != nil {
		return nil, err
	}
	backup, err := backupFile(m.dir, client.ID, plan.File)
	if err != nil {
		return nil, err
	}
	section := sectionOf(doc, client.Root)
	record := m.state.Clients[client.ID]
	// Remove entries this client used to have from MyMCP.
	for name := range record.Entries {
		if _, keep := plan.Entries[name]; !keep {
			delete(section, name)
		}
	}
	if record.Entry != "" {
		if _, keep := plan.Entries[record.Entry]; !keep {
			delete(section, record.Entry)
		}
	}
	hashes := map[string]string{}
	for name, entry := range plan.Entries {
		section[name] = entry
		hashes[name] = hashAny(entry)
	}
	doc[client.Root] = section
	if err := writeDocument(client, plan.File, doc); err != nil {
		return nil, err
	}
	m.state.Clients[client.ID] = Record{
		Client: client.ID, File: plan.File, Mode: "direct", Entries: hashes,
		Backup: backup, AppliedAt: time.Now(),
	}
	if err := m.saveState(); err != nil {
		return nil, err
	}
	return plan, nil
}

func (m *Manager) Apply(clientID, endpoint, entry string, force bool) (*Plan, error) {
	plan, err := m.Plan(clientID, endpoint, entry)
	if err != nil {
		return nil, err
	}
	if plan.Conflict && !force {
		return plan, fmt.Errorf("%s: %s; use --force to overwrite", plan.File, plan.Reason)
	}
	client := m.clientByID(clientID)
	if client.Plugin != nil {
		return m.pluginApply(client, plan)
	}
	path := plan.File
	doc, err := readDocument(client, path)
	if err != nil {
		return nil, err
	}
	backup, err := backupFile(m.dir, client.ID, path)
	if err != nil {
		return nil, err
	}
	section := sectionOf(doc, client.Root)
	record := m.state.Clients[client.ID]
	// Switching from Direct mode must not leave the old native entries
	// behind, and a renamed router entry must not linger either.
	for name := range record.Entries {
		if name != entry {
			delete(section, name)
		}
	}
	if record.Entry != "" && record.Entry != entry {
		delete(section, record.Entry)
	}
	section[entry] = plan.Desired
	doc[client.Root] = section
	if err := writeDocument(client, path, doc); err != nil {
		return nil, err
	}
	m.state.Clients[client.ID] = Record{
		Client: client.ID, File: path, Entry: entry, Hash: hashAny(plan.Desired),
		Backup: backup, AppliedAt: time.Now(),
	}
	if err := m.saveState(); err != nil {
		return nil, err
	}
	return plan, nil
}

// Detach removes the entry MyMCP wrote. A changed entry is kept unless force.
func (m *Manager) Detach(clientID string, force bool) error {
	client := m.clientByID(clientID)
	if client == nil {
		return fmt.Errorf("unknown client %q", clientID)
	}
	record, ok := m.state.Clients[client.ID]
	if !ok {
		return fmt.Errorf("%s is not managed", client.Label)
	}
	path := record.File
	if client.Plugin != nil {
		return m.pluginDetach(client, record, force)
	}
	doc, err := readDocument(client, path)
	if err != nil {
		return err
	}
	section := sectionOf(doc, client.Root)
	entries := map[string]string{}
	for name, hash := range record.Entries {
		entries[name] = hash
	}
	if record.Entry != "" {
		entries[record.Entry] = record.Hash
	}
	for name, hash := range entries {
		current, exists := section[name]
		if exists && hashAny(current) != hash && !force {
			return fmt.Errorf("%s changed since MyMCP wrote it; use --force", path)
		}
	}
	if _, err := backupFile(m.dir, client.ID, path); err != nil {
		return err
	}
	for name := range entries {
		delete(section, name)
	}
	doc[client.Root] = section
	if err := writeDocument(client, path, doc); err != nil {
		return err
	}
	delete(m.state.Clients, client.ID)
	return m.saveState()
}

// Restore puts back the backup taken before MyMCP last wrote a client's
// configuration, then forgets the ownership record.
func (m *Manager) Restore(clientID string) error {
	client := m.clientByID(clientID)
	if client == nil {
		return fmt.Errorf("unknown client %q", clientID)
	}
	record, ok := m.state.Clients[client.ID]
	if !ok {
		return fmt.Errorf("%s is not managed", client.Label)
	}
	if record.Backup == "" {
		return fmt.Errorf("%s has no backup (the file did not exist before MyMCP wrote it); use client detach", client.Label)
	}
	raw, err := os.ReadFile(record.Backup)
	if err != nil {
		return err
	}
	if _, err := backupFile(m.dir, client.ID, record.File); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(record.File), 0o755); err != nil {
		return err
	}
	temp := record.File + ".mymcp.tmp"
	if err := os.WriteFile(temp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, record.File); err != nil {
		return err
	}
	delete(m.state.Clients, client.ID)
	return m.saveState()
}

// Status is the state of one client for `client list`.
type Status struct {
	Client  string `json:"client"`
	Label   string `json:"label"`
	File    string `json:"file"`
	Icon    string `json:"icon,omitempty"`
	Present bool   `json:"present"`
	Managed bool   `json:"managed"`
	Drifted bool   `json:"drifted,omitempty"`
	Entry   string `json:"entry,omitempty"`
}

// Statuses lists every client.
func (m *Manager) Statuses() []Status {
	var out []Status
	for _, client := range m.clients() {
		path := client.Path()
		_, err := os.Stat(path)
		present := err == nil
		icon := ""
		if client.Plugin != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			present = client.Plugin.Detect(ctx)
			cancel()
			icon = client.Plugin.IconPath()
		}
		record, managed := m.state.Clients[client.ID]
		drifted := false
		if managed {
			drifted, _ = m.Drift(client.ID)
		}
		out = append(out, Status{
			Client: client.ID, Label: client.Label, File: path, Icon: icon,
			Present: present, Managed: managed, Drifted: drifted, Entry: record.Entry,
		})
	}
	return out
}

// Drift reports whether a managed client's MyMCP entry changed or vanished
// after MyMCP wrote it.
func (m *Manager) Drift(clientID string) (bool, error) {
	client := m.clientByID(clientID)
	if client == nil {
		return false, fmt.Errorf("unknown client %q", clientID)
	}
	record, managed := m.state.Clients[client.ID]
	if !managed {
		return false, nil
	}
	if client.Plugin != nil {
		raw, err := os.ReadFile(record.File)
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return record.Hash != "" && hashAny(string(raw)) != record.Hash, nil
	}
	doc, err := readDocument(client, record.File)
	if err != nil {
		return false, err
	}
	section := sectionOf(doc, client.Root)
	if record.Mode == "direct" {
		for name, hash := range record.Entries {
			current, exists := section[name]
			if !exists || hashAny(current) != hash {
				return true, nil
			}
		}
		return false, nil
	}
	current, exists := section[record.Entry]
	return !exists || hashAny(current) != record.Hash, nil
}

func (m *Manager) saveState() error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	temp := m.statePath + ".tmp"
	if err := os.WriteFile(temp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temp, m.statePath)
}

func hasSecretRef(server DirectServer) bool {
	for _, values := range []map[string]string{server.Env, server.Headers} {
		for _, value := range values {
			if strings.HasPrefix(strings.TrimSpace(value), "secret://") {
				return true
			}
		}
	}
	return false
}

func directEntry(clientID string, server DirectServer) map[string]any {
	if server.Transport == "stdio" {
		env := map[string]string{}
		for key, value := range server.Env {
			env[key] = value
		}
		switch clientID {
		case "opencode":
			command := append([]string{server.Command}, server.Args...)
			entry := map[string]any{"type": "local", "command": command, "enabled": true}
			if len(env) > 0 {
				entry["environment"] = env
			}
			return entry
		case "cline":
			transport := map[string]any{"type": "stdio", "command": server.Command, "args": stringsOrEmpty(server.Args)}
			if len(env) > 0 {
				transport["env"] = env
			}
			return map[string]any{"transport": transport}
		case "goose":
			entry := map[string]any{"enabled": true, "name": server.Name, "type": "stdio", "cmd": server.Command, "args": stringsOrEmpty(server.Args), "timeout": 300}
			if len(env) > 0 {
				entry["envs"] = env
			}
			return entry
		case "claudecode", "copilot", "qoder", "crush", "droid", "vscode":
			entry := map[string]any{"type": "stdio", "command": server.Command, "args": stringsOrEmpty(server.Args)}
			if len(env) > 0 {
				entry["env"] = env
			}
			return entry
		default:
			entry := map[string]any{"command": server.Command, "args": stringsOrEmpty(server.Args)}
			if len(env) > 0 {
				entry["env"] = env
			}
			return entry
		}
	}
	headers := map[string]string{}
	for key, value := range server.Headers {
		headers[key] = value
	}
	if clientID == "claude" || clientID == "kiro" || clientID == "codex" || clientID == "zed" {
		return bridgeEntry(server.URL)
	}
	entry := map[string]any{}
	switch clientID {
	case "windsurf":
		entry["serverUrl"] = server.URL
	case "opencode":
		entry["type"] = "remote"
		entry["url"] = server.URL
		entry["enabled"] = true
	case "kimi":
		entry["url"] = server.URL
		entry["transport"] = server.Transport
	case "antigravity":
		entry["type"] = "sse"
		entry["serverUrl"] = server.URL
	case "goose":
		entry["enabled"] = true
		entry["name"] = server.Name
		entry["type"] = "sse"
		entry["uri"] = server.URL
		entry["timeout"] = 300
		if len(headers) > 0 {
			entry["headers"] = headers
		}
		return entry
	case "cline":
		kind := "streamableHttp"
		if server.Transport == "sse" {
			kind = "sse"
		}
		transport := map[string]any{"type": kind, "url": server.URL}
		if len(headers) > 0 {
			transport["headers"] = headers
		}
		return map[string]any{"transport": transport}
	case "vscode":
		kind := "http"
		if server.Transport == "sse" {
			kind = "sse"
		}
		entry["type"] = kind
		entry["url"] = server.URL
	case "claudecode", "copilot", "qoder", "crush", "droid":
		entry["type"] = server.Transport
		entry["url"] = server.URL
	default:
		entry["url"] = server.URL
	}
	if len(headers) > 0 {
		entry["headers"] = headers
	}
	return entry
}

func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (m *Manager) pluginPlan(client *Client, mode, endpoint string, servers []DirectServer, entry string) (*Plan, error) {
	path := client.Path()
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	current := string(raw)
	input := plugin.RenderContext{
		Mode: mode, Endpoint: endpoint, Content: current, Path: path, OS: runtime.GOOS,
	}
	for _, server := range servers {
		if hasSecretRef(server) {
			return nil, fmt.Errorf("%s: direct mode cannot carry secret references; use router mode", server.Name)
		}
		input.Servers = append(input.Servers, plugin.Server{
			Name: server.Name, Transport: server.Transport, Command: server.Command,
			Args: server.Args, URL: server.URL, Env: server.Env, Headers: server.Headers,
		})
	}
	desired, err := client.Plugin.Render(context.Background(), input)
	if err != nil {
		return nil, err
	}
	record := m.state.Clients[client.ID]
	conflict := record.Hash != "" && hashAny(current) != record.Hash && current != desired
	reason := ""
	if conflict {
		reason = "the file changed since MyMCP wrote it"
	}
	planMode := "plugin"
	if mode == "direct" {
		planMode = "plugin-direct"
	}
	return &Plan{
		Client: client.ID, File: path, Entry: entry, Mode: planMode,
		Exists: current != "", Managed: record.Mode != "", Conflict: conflict,
		Current: current, Desired: desired, Reason: reason,
	}, nil
}

func (m *Manager) pluginApply(client *Client, plan *Plan) (*Plan, error) {
	path := plan.File
	backup, err := backupFile(m.dir, client.ID, path)
	if err != nil {
		return nil, err
	}
	desired, _ := plan.Desired.(string)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	temp := path + ".mymcp.tmp"
	if err := os.WriteFile(temp, []byte(desired), 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(temp, path); err != nil {
		return nil, err
	}
	m.state.Clients[client.ID] = Record{
		Client: client.ID, File: path, Mode: plan.Mode, Hash: hashAny(desired),
		Backup: backup, AppliedAt: time.Now(),
	}
	if err := m.saveState(); err != nil {
		return nil, err
	}
	return plan, nil
}

func (m *Manager) pluginDetach(client *Client, record Record, force bool) error {
	raw, err := os.ReadFile(record.File)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	current := string(raw)
	if record.Hash != "" && hashAny(current) != record.Hash && !force {
		return fmt.Errorf("%s changed since MyMCP wrote it; use --force", record.File)
	}
	desired, err := client.Plugin.Render(context.Background(), plugin.RenderContext{
		Mode: "detach", Content: current, Path: record.File, OS: runtime.GOOS,
	})
	if err != nil {
		return err
	}
	if _, err := backupFile(m.dir, client.ID, record.File); err != nil {
		return err
	}
	temp := record.File + ".mymcp.tmp"
	if err := os.WriteFile(temp, []byte(desired), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, record.File); err != nil {
		return err
	}
	delete(m.state.Clients, client.ID)
	return m.saveState()
}

func readDocument(client *Client, path string) (map[string]any, error) {
	doc := map[string]any{}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return doc, nil
	}
	switch client.Kind {
	case CodexTOML:
		if err := toml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return doc, nil
	case YAML:
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return doc, nil
	}
	plain := jsonc.ToJSON(raw)
	if len(plain) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(plain, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

func writeDocument(client *Client, path string, doc map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var raw []byte
	var err error
	switch client.Kind {
	case CodexTOML:
		raw, err = toml.Marshal(doc)
	case YAML:
		raw, err = yaml.Marshal(doc)
	default:
		raw, err = json.MarshalIndent(doc, "", "  ")
		raw = append(raw, '\n')
	}
	if err != nil {
		return err
	}
	temp := path + ".mymcp.tmp"
	if err := os.WriteFile(temp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func backupFile(dir, clientID, path string) (string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	backupDir := filepath.Join(dir, "backups", clientID)
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", err
	}
	name := time.Now().Format("20060102-150405.000") + "-" + filepath.Base(path)
	backup := filepath.Join(backupDir, name)
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		return "", err
	}
	return backup, nil
}

func sectionOf(doc map[string]any, root string) map[string]any {
	section, _ := doc[root].(map[string]any)
	if section == nil {
		section = map[string]any{}
	}
	return section
}

func hashAny(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func expandPath(path string) string {
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
