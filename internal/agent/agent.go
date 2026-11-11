// Package agent turns every supported MCP client into a plugin adapter.
// Built-in adapters are JavaScript plugins embedded in the binary; user
// plugins in ~/.mymcp/plugins can add agents or override a built-in one.
package agent

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tidwall/jsonc"
	"github.com/tk103331/mymcp/internal/plugin"
	"gopkg.in/yaml.v3"
)

//go:embed builtin
var builtinFS embed.FS

// Agent is one plugin-backed MCP client adapter.
type Agent struct {
	Plugin  *plugin.Plugin
	Builtin bool
}

// ID is the adapter's stable identifier.
func (a *Agent) ID() string { return a.Plugin.Manifest.ID }

// Label is the name the UI shows.
func (a *Agent) Label() string { return a.Plugin.Manifest.Label }

// Description explains what the adapter writes.
func (a *Agent) Description() string { return a.Plugin.Manifest.Description }

// Format is the configuration file format: json, jsonc, toml or yaml.
func (a *Agent) Format() string { return a.Plugin.Manifest.Format }

// Section is the document key that holds server entries.
func (a *Agent) Section() string { return a.Plugin.Manifest.Section }

// Path is the native configuration path on this platform.
func (a *Agent) Path() string { return a.Plugin.Path() }

// Icon returns the adapter's icon bytes and extension.
func (a *Agent) Icon() ([]byte, string, bool) { return a.Plugin.Icon() }

// Key returns the native entry key for a server. Adapters that need a
// composite key declare it in plugin.json, for example "{name}-{id}".
func (a *Agent) Key(name, id string) string {
	template := a.Plugin.Manifest.Key
	if template == "" || id == "" {
		return name
	}
	return strings.NewReplacer("{name}", name, "{id}", id).Replace(template)
}

// Render runs the adapter's render export. A document adapter receives the
// parsed configuration and returns the complete document; a raw adapter
// receives the file content and returns the next content.
func (a *Agent) Render(in plugin.RenderContext) (plugin.Rendered, error) {
	format := a.Format()
	if in.Path == "" {
		in.Path = a.Path()
	}
	in.OS = runtime.GOOS
	if format != "" {
		if in.Document == nil {
			if strings.TrimSpace(in.Content) != "" {
				doc, err := Unmarshal(format, []byte(in.Content))
				if err != nil {
					return plugin.Rendered{}, err
				}
				in.Document = doc
			} else {
				raw, err := os.ReadFile(in.Path)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return plugin.Rendered{}, err
				}
				if len(strings.TrimSpace(string(raw))) > 0 {
					doc, err := Unmarshal(format, raw)
					if err != nil {
						return plugin.Rendered{}, err
					}
					in.Document = doc
				} else {
					in.Document = map[string]any{}
				}
				in.Content = string(raw)
			}
		}
	} else if in.Content == "" {
		raw, err := os.ReadFile(in.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return plugin.Rendered{}, err
		}
		in.Content = string(raw)
	}
	rendered, err := a.Plugin.Render(context.Background(), in)
	if err != nil {
		return plugin.Rendered{}, err
	}
	if format != "" {
		if rendered.Document == nil {
			if strings.TrimSpace(rendered.Content) != "" {
				doc, err := Unmarshal(format, []byte(rendered.Content))
				if err != nil {
					return plugin.Rendered{}, err
				}
				rendered.Document = doc
			} else {
				rendered.Document = map[string]any{}
			}
		}
		raw, err := Marshal(format, rendered.Document)
		if err != nil {
			return plugin.Rendered{}, err
		}
		rendered.Content = string(raw)
	}
	return rendered, nil
}

// Preview renders a new configuration from an empty document, without
// touching the client's existing file.
func (a *Agent) Preview(in plugin.RenderContext) (plugin.Rendered, error) {
	in.Document = map[string]any{}
	in.Content = ""
	return a.Render(in)
}

// Load reads the adapter's native servers for import.
func (a *Agent) Load(ctx context.Context) ([]plugin.ImportedServer, error) {
	aPath := a.Path()
	var raw []byte
	var doc map[string]any
	if aPath != "" {
		data, err := os.ReadFile(aPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		raw = data
		if a.Format() != "" && len(strings.TrimSpace(string(raw))) > 0 {
			parsed, err := Unmarshal(a.Format(), raw)
			if err != nil {
				return nil, err
			}
			doc = parsed
		}
	}
	return a.Plugin.LoadServers(ctx, plugin.LoadContext{
		Content: string(raw), Document: doc, Path: aPath, OS: runtime.GOOS,
	})
}

// Detect reports whether the agent is installed.
func (a *Agent) Detect(ctx context.Context) bool { return a.Plugin.Detect(ctx) }

// Registry holds the built-in adapters and the user's installed plugins.
type Registry struct {
	dir      string
	builtins []*Agent
	users    []*Agent
	agents   []*Agent
	errs     []error
}

// New loads the built-in adapters and every valid plugin in dir/plugins.
func New(dir string) (*Registry, error) {
	builtins, err := Builtin()
	if err != nil {
		return nil, err
	}
	r := &Registry{dir: dir, builtins: builtins}
	r.Reload()
	return r, nil
}

// Reload rescans the user plugin directory.
func (r *Registry) Reload() []error {
	plugins, errs := plugin.Discover(filepath.Join(r.dir, "plugins"))
	r.errs = errs
	r.users = make([]*Agent, 0, len(plugins))
	for _, p := range plugins {
		r.users = append(r.users, &Agent{Plugin: p})
	}
	shadowed := map[string]bool{}
	for _, user := range r.users {
		shadowed[user.ID()] = true
	}
	r.builtins = make([]*Agent, 0, len(r.builtins))
	if all, err := Builtin(); err == nil {
		for _, builtin := range all {
			if !shadowed[builtin.ID()] {
				r.builtins = append(r.builtins, builtin)
			}
		}
	}
	r.agents = append(append([]*Agent{}, r.users...), r.builtins...)
	sort.Slice(r.agents, func(i, j int) bool {
		left, right := strings.ToLower(r.agents[i].Label()), strings.ToLower(r.agents[j].Label())
		if left != right {
			return left < right
		}
		return r.agents[i].ID() < r.agents[j].ID()
	})
	return errs
}

// Agents lists built-in and user adapters, sorted by label.
func (r *Registry) Agents() []*Agent { return r.agents }

// Get finds an adapter by ID.
func (r *Registry) Get(id string) *Agent {
	for _, a := range r.agents {
		if a.ID() == id {
			return a
		}
	}
	return nil
}

// UserPlugins returns the adapters installed in ~/.mymcp/plugins.
func (r *Registry) UserPlugins() []*plugin.Plugin {
	out := make([]*plugin.Plugin, 0, len(r.users))
	for _, a := range r.users {
		out = append(out, a.Plugin)
	}
	return out
}

// BuiltinPlugins returns the adapters shipped with MyMCP, except those
// overridden by a user plugin with the same ID.
func (r *Registry) BuiltinPlugins() []*plugin.Plugin {
	out := make([]*plugin.Plugin, 0, len(r.builtins))
	for _, a := range r.builtins {
		out = append(out, a.Plugin)
	}
	return out
}

// Errors are user plugins that failed to load.
func (r *Registry) Errors() []error { return r.errs }

var (
	builtinOnce sync.Once
	builtinList []*Agent
	builtinErr  error
)

// Builtin loads the adapters embedded in the binary once.
func Builtin() ([]*Agent, error) {
	builtinOnce.Do(func() {
		entries, err := fs.ReadDir(builtinFS, "builtin")
		if err != nil {
			builtinErr = err
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			p, err := plugin.LoadFS(builtinFS, "builtin/"+entry.Name())
			if err != nil {
				builtinErr = err
				return
			}
			p.Manifest.Builtin = true
			builtinList = append(builtinList, &Agent{Plugin: p, Builtin: true})
		}
		sort.Slice(builtinList, func(i, j int) bool {
			return strings.ToLower(builtinList[i].Label()) < strings.ToLower(builtinList[j].Label())
		})
	})
	if builtinErr != nil {
		return nil, builtinErr
	}
	return builtinList, nil
}

// Document is one parsed client configuration.
type Document struct {
	Data   map[string]any
	Raw    string
	Exists bool
}

// ReadDocument reads and parses a client's configuration.
func ReadDocument(path, format string) (Document, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Document{Data: map[string]any{}}, nil
	}
	if err != nil {
		return Document{}, err
	}
	doc := Document{Raw: string(raw), Exists: true}
	if len(strings.TrimSpace(doc.Raw)) == 0 {
		doc.Data = map[string]any{}
		return doc, nil
	}
	parsed, err := Unmarshal(format, raw)
	if err != nil {
		return Document{}, err
	}
	doc.Data = parsed
	return doc, nil
}

// Unmarshal parses json, jsonc, toml or yaml into a document.
func Unmarshal(format string, raw []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	doc := map[string]any{}
	switch normalizeFormat(format) {
	case "toml":
		if err := toml.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
	case "yaml":
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
	default:
		plain := jsonc.ToJSON(raw)
		if len(strings.TrimSpace(string(plain))) == 0 {
			return map[string]any{}, nil
		}
		if err := json.Unmarshal(plain, &doc); err != nil {
			return nil, err
		}
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

// Marshal serializes a document in the requested format.
func Marshal(format string, doc map[string]any) ([]byte, error) {
	switch normalizeFormat(format) {
	case "toml":
		return toml.Marshal(doc)
	case "yaml":
		return yaml.Marshal(doc)
	default:
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(raw, '\n'), nil
	}
}

// WriteDocument writes a document atomically with owner-only permissions.
func WriteDocument(path, format string, doc map[string]any) error {
	raw, err := Marshal(format, doc)
	if err != nil {
		return err
	}
	return WriteContent(path, string(raw))
}

// WriteContent writes file content atomically with owner-only permissions.
func WriteContent(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".mymcp.tmp"
	if err := os.WriteFile(temp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func normalizeFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "jsonc" {
		return "json"
	}
	return format
}
