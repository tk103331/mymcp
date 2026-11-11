package app

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

//go:embed assets/servers.json
var catalogFS embed.FS

// CatalogServer is an MCP server of the bundled catalog.
type CatalogServer struct {
	Name          string                         `json:"name"`
	DisplayName   string                         `json:"display_name"`
	Description   string                         `json:"description"`
	Repository    CatalogRepository              `json:"repository"`
	Homepage      string                         `json:"homepage"`
	Author        CatalogAuthor                  `json:"author"`
	License       string                         `json:"license"`
	Categories    []string                       `json:"categories"`
	Tags          []string                       `json:"tags"`
	Examples      []CatalogExample               `json:"examples"`
	Tools         []CatalogTool                  `json:"tools"`
	Installations map[string]CatalogInstallation `json:"installations"`
	Arguments     map[string]CatalogArgument     `json:"arguments"`
	Params        map[string]any                 `json:"params"`
	Official      bool                           `json:"official"`
}

type CatalogRepository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type CatalogAuthor struct {
	Name string `json:"name"`
}

type CatalogExample struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

type CatalogTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type CatalogInstallation struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type CatalogArgument struct {
	Description string `json:"description"`
	Example     string `json:"example"`
	Required    bool   `json:"required"`
}

type catalogData struct {
	servers    []*CatalogServer
	byName     map[string]*CatalogServer
	categories []string
}

var (
	catalogOnce sync.Once
	catalog     catalogData
)

func loadCatalog() *catalogData {
	catalogOnce.Do(func() {
		raw, err := catalogFS.ReadFile("assets/servers.json")
		if err != nil {
			panic(err)
		}
		var entries map[string]*CatalogServer
		if err := json.Unmarshal(raw, &entries); err != nil {
			panic(err)
		}
		catalog.byName = make(map[string]*CatalogServer, len(entries))
		seen := map[string]bool{}
		for name, server := range entries {
			server.Name = name
			if server.DisplayName == "" {
				server.DisplayName = name
			}
			catalog.servers = append(catalog.servers, server)
			catalog.byName[name] = server
			for _, category := range server.Categories {
				if !seen[category] {
					seen[category] = true
					catalog.categories = append(catalog.categories, category)
				}
			}
		}
		sort.Slice(catalog.servers, func(i, j int) bool {
			return strings.ToLower(catalog.servers[i].DisplayName) < strings.ToLower(catalog.servers[j].DisplayName)
		})
		sort.Strings(catalog.categories)
	})
	return &catalog
}

// Servers returns the catalog in display order.
func (c *catalogData) Servers() []*CatalogServer { return c.servers }

// Server looks a server up by its catalog name.
func (c *catalogData) Server(name string) *CatalogServer { return c.byName[name] }

// Categories lists every category of the catalog.
func (c *catalogData) Categories() []string { return c.categories }

// Filter returns the servers matching a query and, when it is not empty, a category.
func (c *catalogData) Filter(query, category string) []*CatalogServer {
	query = strings.ToLower(strings.TrimSpace(query))
	var results []*CatalogServer
	for _, server := range c.servers {
		if category != "" && !contains(server.Categories, category) {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(server.Name), query) &&
			!strings.Contains(strings.ToLower(server.DisplayName), query) &&
			!strings.Contains(strings.ToLower(server.Description), query) {
			continue
		}
		results = append(results, server)
	}
	return results
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// InstallTypes lists the installation keys of a server, sorted.
func (s *CatalogServer) InstallTypes() []string {
	types := make([]string, 0, len(s.Installations))
	for name := range s.Installations {
		types = append(types, name)
	}
	sort.Strings(types)
	return types
}

// InstallCommand is the command line of an installation.
func (s *CatalogServer) InstallCommand(kind string) string {
	installation, ok := s.Installations[kind]
	if !ok {
		return ""
	}
	return strings.TrimSpace(strings.Join(append([]string{installation.Command}, installation.Args...), " "))
}

// ArgumentKeys lists the documented parameters of a server, sorted.
func (s *CatalogServer) ArgumentKeys() []string {
	keys := make([]string, 0, len(s.Arguments))
	for key := range s.Arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
