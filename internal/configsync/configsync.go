// Package configsync writes one router entry into the native configuration
// files of MCP clients, and removes only the entries it wrote. Every client
// is a plugin adapter loaded from internal/agent.
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
	"strings"
	"time"

	"github.com/tk103331/mymcp/internal/agent"
	"github.com/tk103331/mymcp/internal/plugin"
)

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
	Key       string            `json:"key,omitempty"`
	ID        string            `json:"id,omitempty"`
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
	Preview  string         `json:"preview,omitempty"`
	Entries  map[string]any `json:"entries,omitempty"`
	Reason   string         `json:"reason,omitempty"`
}

// Manager applies plans and keeps the ownership manifest.
type Manager struct {
	dir       string
	statePath string
	state     State
	agents    *agent.Registry
}

// New opens the sync state in dir and loads built-in and user adapters.
func New(dir string) (*Manager, error) {
	agents, err := agent.New(dir)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		dir: dir, statePath: filepath.Join(dir, "sync.json"),
		state: State{Clients: map[string]Record{}}, agents: agents,
	}
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

// Agents lists every adapter, built-in and installed.
func (m *Manager) Agents() []*agent.Agent { return m.agents.Agents() }

// Plugins returns the user-installed adapters.
func (m *Manager) Plugins() []*plugin.Plugin { return m.agents.UserPlugins() }

// BuiltinPlugins returns the adapters shipped with MyMCP.
func (m *Manager) BuiltinPlugins() []*plugin.Plugin { return m.agents.BuiltinPlugins() }

// ReloadPlugins rescans the plugins directory after an install or removal.
func (m *Manager) ReloadPlugins() []error { return m.agents.Reload() }

// PluginErrors are plugins that failed to load.
func (m *Manager) PluginErrors() []error { return m.agents.Errors() }

func (m *Manager) clientByID(id string) *agent.Agent { return m.agents.Get(id) }

// Plan computes what applying endpoint as entry would change.
func (m *Manager) Plan(clientID, endpoint, entry string) (*Plan, error) {
	client := m.clientByID(clientID)
	if client == nil {
		return nil, fmt.Errorf("unknown client %q", clientID)
	}
	path := client.Path()
	if path == "" {
		return nil, fmt.Errorf("%s has no configuration file", client.Label())
	}
	doc, err := agent.ReadDocument(path, client.Format())
	if err != nil {
		return nil, err
	}
	record, managed := m.state.Clients[client.ID()]
	remove := removeForRouter(record, entry)
	rendered, err := client.Render(plugin.RenderContext{
		Mode: "router", Endpoint: endpoint, Entry: entry, Bridge: bridgeCommand,
		Document: doc.Data, Content: doc.Raw, Remove: remove,
	})
	if err != nil {
		return nil, err
	}
	plan := &Plan{
		Client: client.ID(), File: path, Entry: entry, Mode: planMode(client, "router"),
		Managed: managed, Preview: rendered.Content,
	}
	if client.Section() != "" {
		current := sectionOf(doc.Data, client.Section())[entry]
		plan.Exists = current != nil
		plan.Current = current
		plan.Desired = sectionOf(rendered.Document, client.Section())[entry]
		plan.Conflict, plan.Reason = routerConflict(client, doc, record, managed, entry)
	} else {
		plan.Exists = doc.Exists
		plan.Current = doc.Raw
		plan.Desired = rendered.Content
		if record.Hash != "" && hashAny(doc.Raw) != record.Hash && doc.Raw != rendered.Content {
			plan.Conflict = true
			plan.Reason = "the file changed since MyMCP wrote it"
		}
	}
	return plan, nil
}

// Apply writes the router entry, backing up the file first.
func (m *Manager) Apply(clientID, endpoint, entry string, force bool) (*Plan, error) {
	plan, err := m.Plan(clientID, endpoint, entry)
	if err != nil {
		return nil, err
	}
	if plan.Conflict && !force {
		return plan, fmt.Errorf("%s: %s; use --force to overwrite", plan.File, plan.Reason)
	}
	client := m.clientByID(clientID)
	if client == nil {
		return nil, fmt.Errorf("unknown client %q", clientID)
	}
	desired := plan.Preview
	backup, err := backupFile(m.dir, client.ID(), plan.File)
	if err != nil {
		return nil, err
	}
	if err := agent.WriteContent(plan.File, desired); err != nil {
		return nil, err
	}
	record := Record{
		Client: client.ID(), File: plan.File, Mode: plan.Mode, Entry: entry,
		Backup: backup, AppliedAt: time.Now(),
	}
	if client.Section() != "" {
		doc, err := agent.Unmarshal(client.Format(), []byte(plan.Preview))
		if err != nil {
			return nil, err
		}
		record.Hash = hashAny(sectionOf(doc, client.Section())[entry])
	} else {
		record.Hash = hashAny(plan.Preview)
	}
	m.state.Clients[client.ID()] = record
	if err := m.saveState(); err != nil {
		return nil, err
	}
	return plan, nil
}

// PlanDirect compares the client's file with every entry Direct mode would
// write for servers.
func (m *Manager) PlanDirect(clientID string, servers []DirectServer, force bool) (*Plan, error) {
	client := m.clientByID(clientID)
	if client == nil {
		return nil, fmt.Errorf("unknown client %q", clientID)
	}
	path := client.Path()
	if path == "" {
		return nil, fmt.Errorf("%s has no configuration file", client.Label())
	}
	doc, err := agent.ReadDocument(path, client.Format())
	if err != nil {
		return nil, err
	}
	record, managed := m.state.Clients[client.ID()]
	desiredKeys := map[string]bool{}
	input := plugin.RenderContext{
		Mode: "direct", Entry: "mymcp", Bridge: bridgeCommand,
		Document: doc.Data, Content: doc.Raw,
	}
	for _, server := range servers {
		if hasSecretRef(server) {
			return nil, fmt.Errorf("%s: direct mode cannot carry secret references; use router mode", server.Name)
		}
		key := server.Key
		if key == "" {
			key = client.Key(server.Name, server.ID)
		}
		desiredKeys[key] = true
		input.Servers = append(input.Servers, plugin.Server{
			Name: server.Name, Key: key, ID: server.ID, Transport: server.Transport,
			Command: server.Command, Args: server.Args, URL: server.URL,
			Env: server.Env, Headers: server.Headers,
		})
	}
	input.Remove = removeForDirect(record, desiredKeys)
	rendered, err := client.Render(input)
	if err != nil {
		return nil, err
	}
	plan := &Plan{
		Client: client.ID(), File: path, Mode: planMode(client, "direct"),
		Managed: managed, Preview: rendered.Content,
	}
	if client.Section() != "" {
		desiredSection := sectionOf(rendered.Document, client.Section())
		entries := map[string]any{}
		for key := range desiredKeys {
			if value, ok := desiredSection[key]; ok {
				entries[key] = value
			}
		}
		plan.Entries = entries
		plan.Desired = entries
		plan.Conflict, plan.Reason = directConflict(client, doc, record, managed, desiredKeys, entries)
	} else {
		plan.Current = doc.Raw
		plan.Desired = rendered.Content
		if record.Hash != "" && hashAny(doc.Raw) != record.Hash && doc.Raw != rendered.Content {
			plan.Conflict = true
			plan.Reason = "the file changed since MyMCP wrote it"
		}
	}
	return plan, nil
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
	if client == nil {
		return nil, fmt.Errorf("unknown client %q", clientID)
	}
	desired := plan.Preview
	backup, err := backupFile(m.dir, client.ID(), plan.File)
	if err != nil {
		return nil, err
	}
	if err := agent.WriteContent(plan.File, desired); err != nil {
		return nil, err
	}
	record := Record{
		Client: client.ID(), File: plan.File, Mode: plan.Mode,
		Backup: backup, AppliedAt: time.Now(),
	}
	if client.Section() != "" {
		hashes := map[string]string{}
		for name, entry := range plan.Entries {
			hashes[name] = hashAny(entry)
		}
		record.Entries = hashes
	} else {
		record.Hash = hashAny(plan.Preview)
	}
	m.state.Clients[client.ID()] = record
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
	record, ok := m.state.Clients[client.ID()]
	if !ok {
		return fmt.Errorf("%s is not managed", client.Label())
	}
	doc, err := agent.ReadDocument(record.File, client.Format())
	if err != nil {
		return err
	}
	if client.Section() != "" {
		section := sectionOf(doc.Data, client.Section())
		for name, hash := range record.Entries {
			if current, exists := section[name]; exists && hashAny(current) != hash && !force {
				return fmt.Errorf("%s changed since MyMCP wrote it; use --force", record.File)
			}
		}
		if record.Entry != "" {
			if current, exists := section[record.Entry]; exists && hashAny(current) != record.Hash && !force {
				return fmt.Errorf("%s changed since MyMCP wrote it; use --force", record.File)
			}
		}
	} else if record.Hash != "" && hashAny(doc.Raw) != record.Hash && !force {
		return fmt.Errorf("%s changed since MyMCP wrote it; use --force", record.File)
	}
	remove := removeForDetach(record)
	rendered, err := client.Render(plugin.RenderContext{
		Mode: "detach", Entry: record.Entry, Bridge: bridgeCommand,
		Document: doc.Data, Content: doc.Raw, Remove: remove,
	})
	if err != nil {
		return err
	}
	if _, err := backupFile(m.dir, client.ID(), record.File); err != nil {
		return err
	}
	if err := agent.WriteContent(record.File, rendered.Content); err != nil {
		return err
	}
	delete(m.state.Clients, client.ID())
	return m.saveState()
}

// Restore puts back the backup taken before MyMCP last wrote a client's
// configuration, then forgets the ownership record.
func (m *Manager) Restore(clientID string) error {
	client := m.clientByID(clientID)
	if client == nil {
		return fmt.Errorf("unknown client %q", clientID)
	}
	record, ok := m.state.Clients[client.ID()]
	if !ok {
		return fmt.Errorf("%s is not managed", client.Label())
	}
	if record.Backup == "" {
		return fmt.Errorf("%s has no backup (the file did not exist before MyMCP wrote it); use client detach", client.Label())
	}
	raw, err := os.ReadFile(record.Backup)
	if err != nil {
		return err
	}
	if _, err := backupFile(m.dir, client.ID(), record.File); err != nil {
		return err
	}
	if err := agent.WriteContent(record.File, string(raw)); err != nil {
		return err
	}
	delete(m.state.Clients, client.ID())
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

// Statuses lists every writable client.
func (m *Manager) Statuses() []Status {
	var out []Status
	for _, client := range m.agents.Agents() {
		path := client.Path()
		if path == "" {
			continue
		}
		_, err := os.Stat(path)
		present := err == nil
		icon := ""
		if !client.Builtin {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			present = client.Detect(ctx)
			cancel()
			icon = client.Plugin.IconPath()
		}
		record, managed := m.state.Clients[client.ID()]
		drifted := false
		if managed {
			drifted, _ = m.Drift(client.ID())
		}
		out = append(out, Status{
			Client: client.ID(), Label: client.Label(), File: path, Icon: icon,
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
	record, managed := m.state.Clients[client.ID()]
	if !managed {
		return false, nil
	}
	doc, err := agent.ReadDocument(record.File, client.Format())
	if err != nil {
		return false, err
	}
	if !doc.Exists {
		return true, nil
	}
	if record.Mode == "plugin" || record.Mode == "plugin-direct" {
		return record.Hash != "" && hashAny(doc.Raw) != record.Hash, nil
	}
	section := sectionOf(doc.Data, client.Section())
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

func planMode(client *agent.Agent, mode string) string {
	if client.Section() == "" {
		if mode == "direct" {
			return "plugin-direct"
		}
		return "plugin"
	}
	return mode
}

func removeForRouter(record Record, entry string) []string {
	var remove []string
	for name := range record.Entries {
		if name != entry {
			remove = append(remove, name)
		}
	}
	if record.Entry != "" && record.Entry != entry {
		remove = append(remove, record.Entry)
	}
	return remove
}

func removeForDirect(record Record, desired map[string]bool) []string {
	var remove []string
	for name := range record.Entries {
		if !desired[name] {
			remove = append(remove, name)
		}
	}
	if record.Entry != "" && !desired[record.Entry] {
		remove = append(remove, record.Entry)
	}
	return remove
}

func removeForDetach(record Record) []string {
	var remove []string
	for name := range record.Entries {
		remove = append(remove, name)
	}
	if record.Entry != "" {
		remove = append(remove, record.Entry)
	}
	return remove
}

func routerConflict(client *agent.Agent, doc agent.Document, record Record, managed bool, entry string) (bool, string) {
	if !managed {
		return false, ""
	}
	section := sectionOf(doc.Data, client.Section())
	if current, exists := section[entry]; exists && hashAny(current) != record.Hash {
		return true, "the entry changed since MyMCP wrote it"
	}
	for name, hash := range record.Entries {
		if name == entry {
			continue
		}
		if current, exists := section[name]; exists && hashAny(current) != hash {
			return true, fmt.Sprintf("entry %q changed since MyMCP wrote it", name)
		}
	}
	if record.Entry != "" && record.Entry != entry {
		if current, exists := section[record.Entry]; exists && hashAny(current) != record.Hash {
			return true, "the previous router entry changed since MyMCP wrote it"
		}
	}
	return false, ""
}

func directConflict(client *agent.Agent, doc agent.Document, record Record, managed bool, desiredKeys map[string]bool, desired map[string]any) (bool, string) {
	if !managed {
		if client.Section() == "" {
			return false, ""
		}
	}
	section := sectionOf(doc.Data, client.Section())
	for key := range desiredKeys {
		current, exists := section[key]
		if !exists {
			continue
		}
		if hash, known := record.Entries[key]; known {
			if hashAny(current) != hash && hashAny(current) != hashAny(desired[key]) {
				return true, fmt.Sprintf("entry %q changed since MyMCP wrote it", key)
			}
			continue
		}
		if key == record.Entry {
			if hashAny(current) != record.Hash && hashAny(current) != hashAny(desired[key]) {
				return true, "the router entry changed since MyMCP wrote it"
			}
			continue
		}
		return true, fmt.Sprintf("entry %q already exists and is not managed by MyMCP", key)
	}
	for name, hash := range record.Entries {
		if desiredKeys[name] {
			continue
		}
		if current, exists := section[name]; exists && hashAny(current) != hash {
			return true, fmt.Sprintf("entry %q changed since MyMCP wrote it", name)
		}
	}
	if record.Entry != "" && !desiredKeys[record.Entry] {
		if current, exists := section[record.Entry]; exists && hashAny(current) != record.Hash {
			return true, "the router entry changed since MyMCP wrote it"
		}
	}
	return false, ""
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
