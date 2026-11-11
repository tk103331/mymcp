// Package plugin runs local JavaScript plugins that teach MyMCP how to read
// and write another agent's configuration. Plugins are trusted local code:
// moejs gives them the JavaScript standard library only, with no filesystem,
// network or timers.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Calcium-Ion/moejs"
)

// Manifest is plugin.json.
type Manifest struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Entry       string            `json:"entry,omitempty"`
	Format      string            `json:"format,omitempty"`
	Paths       map[string]string `json:"paths"`
	Icon        string            `json:"icon,omitempty"`
}

// Server is one server handed to a plugin in Direct mode.
type Server struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// RenderContext is the argument of a plugin's render function.
type RenderContext struct {
	Mode     string   `json:"mode"` // router | direct | detach
	Endpoint string   `json:"endpoint,omitempty"`
	Profile  string   `json:"profile,omitempty"`
	Content  string   `json:"content,omitempty"`
	Path     string   `json:"path"`
	OS       string   `json:"os"`
	Servers  []Server `json:"servers,omitempty"`
}

// ImportedServer is one server returned by a plugin's load function.
type ImportedServer struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// LoadContext is the argument of a plugin's load function.
type LoadContext struct {
	Content string `json:"content"`
	Path    string `json:"path"`
	OS      string `json:"os"`
}

// Plugin is a loaded plugin.
type Plugin struct {
	Dir      string
	Manifest Manifest
	module   *moejs.Module
}

// Load reads a plugin directory containing plugin.json and its entry module.
func Load(dir string) (*Plugin, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	if !validID(manifest.ID) {
		return nil, fmt.Errorf("%s: plugin.json has an invalid id %q", dir, manifest.ID)
	}
	if manifest.Label == "" {
		manifest.Label = manifest.ID
	}
	if manifest.Entry == "" {
		manifest.Entry = "index.js"
	}
	entry, err := within(dir, manifest.Entry)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	source, err := os.ReadFile(entry)
	if err != nil {
		return nil, err
	}
	module, err := moejs.Compile(manifest.ID, string(source))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", manifest.ID, err)
	}
	return &Plugin{Dir: dir, Manifest: manifest, module: module}, nil
}

// Discover loads every valid plugin in dir. Invalid plugins are returned as
// errors, so one bad plugin does not hide the others.
func Discover(dir string) ([]*Plugin, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{err}
	}
	var plugins []*Plugin
	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		plugin, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		plugins = append(plugins, plugin)
	}
	return plugins, errs
}

// IconPath returns the plugin's icon file, or "" when it has none.
func (p *Plugin) IconPath() string {
	if p.Manifest.Icon == "" {
		return ""
	}
	path, err := within(p.Dir, p.Manifest.Icon)
	if err != nil {
		return ""
	}
	return path
}

// Path returns the native configuration path on this OS.
func (p *Plugin) Path() string {
	path := p.Manifest.Paths[runtime.GOOS]
	if path == "" {
		path = p.Manifest.Paths["default"]
	}
	return expandPath(path)
}

// Detect asks the plugin whether the agent is present. A plugin without a
// detect export is present when its configuration file exists.
func (p *Plugin) Detect(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rt, hook, ok, err := p.runtime(ctx, "detect")
	if err != nil || !ok {
		_, statErr := os.Stat(p.Path())
		return statErr == nil
	}
	defer watchInterrupt(ctx, rt)()
	defer rt.ReleaseCallData()
	arg, err := rt.FromGo(map[string]any{"path": p.Path(), "os": runtime.GOOS})
	if err != nil {
		return false
	}
	res, err := rt.Call(hook, arg)
	if err != nil {
		return false
	}
	var present bool
	if err := rt.Unmarshal(res, &present); err != nil {
		return false
	}
	return present
}

// Render calls the plugin's render export and returns the new file content.
func (p *Plugin) Render(ctx context.Context, input RenderContext) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rt, hook, ok, err := p.runtime(ctx, "render")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s: render is not exported", p.Manifest.ID)
	}
	defer watchInterrupt(ctx, rt)()
	defer watchInterrupt(ctx, rt)()
	defer rt.ReleaseCallData()
	arg, err := rt.FromGo(input)
	if err != nil {
		return "", err
	}
	res, err := rt.Call(hook, arg)
	if err != nil {
		return "", fmt.Errorf("%s render: %w", p.Manifest.ID, err)
	}
	var raw any
	if err := rt.Unmarshal(res, &raw); err != nil {
		return "", err
	}
	switch value := raw.(type) {
	case string:
		return value, nil
	case map[string]any:
		if content, ok := value["content"].(string); ok {
			return content, nil
		}
	}
	return "", fmt.Errorf("%s render: want a string or {content}", p.Manifest.ID)
}

// LoadServers calls the plugin's load export.
func (p *Plugin) LoadServers(ctx context.Context, input LoadContext) ([]ImportedServer, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rt, hook, ok, err := p.runtime(ctx, "load")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	defer rt.ReleaseCallData()
	arg, err := rt.FromGo(input)
	if err != nil {
		return nil, err
	}
	res, err := rt.Call(hook, arg)
	if err != nil {
		return nil, fmt.Errorf("%s load: %w", p.Manifest.ID, err)
	}
	var servers []ImportedServer
	if err := rt.Unmarshal(res, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func (p *Plugin) runtime(ctx context.Context, export string) (*moejs.Runtime, moejs.Hook, bool, error) {
	rt := moejs.NewRuntime(moejs.Options{
		MemoryLimit:        64 << 20,
		DisableDynamicCode: true,
	})
	if err := rt.Load(p.module); err != nil {
		return nil, moejs.Hook{}, false, err
	}
	hook, err := p.module.Hook(export)
	if err != nil {
		return rt, moejs.Hook{}, false, nil
	}
	has, err := rt.Has(hook)
	if err != nil || !has {
		return rt, moejs.Hook{}, false, err
	}
	return rt, hook, true, nil
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

func watchInterrupt(ctx context.Context, rt *moejs.Runtime) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		rt.Interrupt(context.Cause(ctx))
		close(done)
	})
	return func() {
		if !stop() {
			<-done
		}
	}
}

// within resolves a plugin-relative file and keeps it inside the plugin.
func within(dir, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid plugin path %q", name)
	}
	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("plugin path %q escapes its directory", name)
	}
	return filepath.Join(dir, clean), nil
}

func validID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\`) && !strings.ContainsRune(id, 0)
}

// Install copies a plugin directory into root/<id>.
func Install(source, root string) (*Plugin, error) {
	p, err := Load(source)
	if err != nil {
		return nil, err
	}
	dest := filepath.Join(root, p.Manifest.ID)
	if _, err := os.Stat(dest); err == nil {
		return nil, fmt.Errorf("%s is already installed", p.Manifest.ID)
	}
	if err := copyDir(source, dest); err != nil {
		return nil, err
	}
	return p, nil
}

// Remove deletes an installed plugin.
func Remove(root, id string) error {
	if !validID(id) {
		return fmt.Errorf("invalid plugin id %q", id)
	}
	return os.RemoveAll(filepath.Join(root, id))
}

func copyDir(source, dest string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode())
	})
}
