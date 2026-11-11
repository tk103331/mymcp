package app

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/internal/agent"
	"github.com/tk103331/mymcp/internal/configsync"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/plugin"
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

func loadClientIcons() {
	names := make([]string, 0, len(supportedClients))
	for _, client := range supportedClients {
		names = append(names, client.Name)
	}
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

// registerClientIcon decodes a plugin-supplied icon under key.
func registerClientIcon(key string, raw []byte, name string) {
	if strings.HasSuffix(strings.ToLower(name), ".svg") {
		if svg, err := ui.ParseSVG(raw); err == nil {
			clientSVGs[key] = svg
		}
		return
	}
	if bitmap, err := ui.DecodeBitmap(raw); err == nil {
		clientBitmaps[key] = bitmap
	}
}

// SVG returns the client's vector brand icon, or nil when it has none.
func (c *clientDef) SVG() *ui.SVG { return clientSVGs[c.Name] }

// Icon returns the client's bitmap brand icon, or nil when it has none.
func (c *clientDef) Icon() *ui.Bitmap { return clientBitmaps[c.Name] }

// clientDef is one built-in adapter shown by the legacy workspace pages.
// The actual client support lives in internal/agent's embedded plugins.
type clientDef struct {
	Name  string
	Label string
	agent *agent.Agent
}

// supportedClients is the built-in plugin registry in label order.
var supportedClients = loadSupportedClients()

func loadSupportedClients() []*clientDef {
	agents, err := agent.Builtin()
	if err != nil {
		log.Println("built-in agent plugins:", err)
		return nil
	}
	clients := make([]*clientDef, 0, len(agents))
	for _, a := range agents {
		clients = append(clients, &clientDef{Name: a.ID(), Label: a.Label(), agent: a})
	}
	return clients
}

func clientByName(name string) *clientDef {
	for _, client := range supportedClients {
		if client.Name == name {
			return client
		}
	}
	return nil
}

// ConfigPath returns the client's configuration file on this platform, "" when it has none.
func (c *clientDef) ConfigPath() string {
	if c.agent == nil {
		return ""
	}
	return c.agent.Path()
}

// Config returns the configuration the client expects for the instances.
func (c *clientDef) Config(instances []*data.ServerInstance) map[string]any {
	rendered, err := c.render(instances)
	if err != nil {
		return nil
	}
	if rendered.Document != nil {
		return rendered.Document
	}
	doc := map[string]any{}
	if err := json.Unmarshal([]byte(rendered.Content), &doc); err != nil {
		return nil
	}
	return doc
}

// PreviewText is the configuration as the dialog shows it.
func (c *clientDef) PreviewText(instances []*data.ServerInstance) string {
	rendered, err := c.render(instances)
	if err != nil {
		return ""
	}
	return rendered.Content
}

func (c *clientDef) render(instances []*data.ServerInstance) (plugin.Rendered, error) {
	if c.agent == nil {
		return plugin.Rendered{}, nil
	}
	return c.agent.Preview(plugin.RenderContext{
		Mode: "direct", Entry: "mymcp", Bridge: configsync.BridgeCommand(),
		Servers: clientServers(c.agent, instances),
	})
}

func clientServers(adapter *agent.Agent, instances []*data.ServerInstance) []plugin.Server {
	servers := make([]plugin.Server, 0, len(instances))
	for _, instance := range instances {
		if instance == nil || instance.Config == nil {
			continue
		}
		name := instance.Config.Name
		if name == "" {
			name = instance.ID
		}
		servers = append(servers, plugin.Server{
			Name: name, Key: adapter.Key(name, instance.Config.ID), ID: instance.Config.ID,
			Transport: "sse", URL: instance.Endpoint,
		})
	}
	return servers
}

// saveManagedClientConfig merges the generated configuration into the client's file.
func saveManagedClientConfig(client *clientDef, instances []*data.ServerInstance) error {
	path := client.ConfigPath()
	if path == "" {
		return fmt.Errorf("%s has no configuration file", client.Label)
	}
	rendered, err := client.agent.Render(plugin.RenderContext{
		Mode: "direct", Entry: "mymcp", Bridge: configsync.BridgeCommand(),
		Servers: clientServers(client.agent, instances),
	})
	if err != nil {
		return err
	}
	return agent.WriteContent(path, rendered.Content)
}
