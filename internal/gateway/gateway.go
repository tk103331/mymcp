// Package gateway exposes the servers of a profile as one MCP endpoint and
// records every tool call in the local usage log.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tk103331/mymcp/internal/oauth"
	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

// Version is reported to MCP clients as the gateway's version. Release
// builds set it with -ldflags -X, so keep it a variable.
var Version = "0.1.0"

const (
	defaultToolThreshold = 10
	reapEvery            = 30 * time.Second
)

// Options builds one profile's gateway.
type Options struct {
	Profile string
	Store   *store.DB
	Secrets *secrets.Store
	BaseURL string
	// ToolSearch exposes find_tools/call_tool instead of every tool once a
	// profile has more than ToolSearchThreshold tools.
	ToolSearch          bool
	ToolSearchThreshold int
}

// Gateway is the MCP server of one profile.
type Gateway struct {
	profile   string
	store     *store.DB
	secrets   *secrets.Store
	baseURL   string
	mcp       *server.MCPServer
	upstreams map[string]*upstream
	routes    map[string]route
	errors    map[string]string
	search    bool
	threshold int
	stop      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
}

type upstream struct {
	mu       sync.Mutex
	server   *store.Server
	client   client.MCPClient
	info     mcp.Implementation
	tools    []mcp.Tool
	lastUsed time.Time
	inFlight int
}

type route struct {
	upstream *upstream
	tool     string
	name     string
}

// ToolInfo describes one aggregated tool for inspection and debugging.
type ToolInfo struct {
	Name        string          `json:"name"`
	Server      string          `json:"server"`
	Tool        string          `json:"tool"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// New starts every enabled server of the profile, discovers its tools and
// registers one aggregated MCP surface.
func New(opts Options) (*Gateway, error) {
	threshold := opts.ToolSearchThreshold
	if threshold <= 0 {
		threshold = defaultToolThreshold
	}
	g := &Gateway{
		profile:   opts.Profile,
		store:     opts.Store,
		secrets:   opts.Secrets,
		baseURL:   opts.BaseURL,
		mcp:       server.NewMCPServer("mymcp", Version),
		upstreams: map[string]*upstream{},
		routes:    map[string]route{},
		errors:    map[string]string{},
		threshold: threshold,
		stop:      make(chan struct{}),
	}
	servers, err := opts.Store.ServersForProfile(context.Background(), opts.Profile)
	if err != nil {
		return nil, err
	}
	// Open servers in parallel, with a small cap, so one slow or broken
	// upstream does not delay the whole gateway by its timeout.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, cfg := range servers {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			up := &upstream{server: cfg, lastUsed: time.Now()}
			if err := g.open(up); err != nil {
				g.mu.Lock()
				g.errors[cfg.Name] = err.Error()
				g.mu.Unlock()
				log.Printf("server %s: %v", cfg.Name, err)
				return
			}
			g.mu.Lock()
			g.upstreams[cfg.Name] = up
			g.mu.Unlock()
			if err := g.discover(up); err != nil {
				g.mu.Lock()
				g.errors[cfg.Name] = err.Error()
				g.mu.Unlock()
				log.Printf("server %s tools: %v", cfg.Name, err)
			}
		}()
	}
	wg.Wait()
	g.search = opts.ToolSearch && len(g.routes) > threshold
	if g.search {
		g.registerSearch()
	} else {
		g.registerAll()
	}
	go g.reapLoop()
	return g, nil
}

// open dials and initializes an upstream.
func (g *Gateway) open(up *upstream) error {
	up.client = nil
	cfg := up.server
	env, err := g.secrets.ResolveMap(cfg.Env)
	if err != nil {
		return err
	}
	headers, err := g.secrets.ResolveMap(cfg.Headers)
	if err != nil {
		return err
	}
	if cfg.OAuth != nil && cfg.Transport != "stdio" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		token, err := oauth.AccessToken(ctx, cfg.OAuth, cfg.Name, g.secrets, http.DefaultClient)
		cancel()
		if err != nil {
			return err
		}
		if headers == nil {
			headers = map[string]string{}
		}
		headers["Authorization"] = "Bearer " + token
	}
	var mcpClient client.MCPClient
	switch cfg.Transport {
	case "stdio":
		command, args := stdioCommand(cfg, env)
		mcpClient, err = client.NewStdioMCPClient(command, mergeEnv(os.Environ(), env), args...)
		if err != nil {
			return err
		}
	case "http":
		transportClient, err := transport.NewStreamableHTTP(cfg.URL, transport.WithHTTPHeaders(headers))
		if err != nil {
			return err
		}
		mcpClient = client.NewClient(transportClient)
	case "sse":
		transportClient, err := transport.NewSSE(cfg.URL, transport.WithHeaders(headers))
		if err != nil {
			return err
		}
		mcpClient = client.NewClient(transportClient)
	default:
		return fmt.Errorf("unknown transport %q", cfg.Transport)
	}
	starter, ok := mcpClient.(interface{ Start(context.Context) error })
	if !ok {
		mcpClient.Close()
		return fmt.Errorf("%s: client does not start", cfg.Name)
	}
	// The SSE/HTTP stream lives as long as the client, not just this handshake.
	if err := starter.Start(context.Background()); err != nil {
		mcpClient.Close()
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "mymcp", Version: Version}
	result, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		mcpClient.Close()
		return err
	}
	up.client, up.info, up.lastUsed = mcpClient, result.ServerInfo, time.Now()
	g.mu.Lock()
	delete(g.errors, cfg.Name)
	g.mu.Unlock()
	return nil
}

func (g *Gateway) discover(up *upstream) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	list, err := up.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	up.tools = list.Tools
	delete(g.errors, up.server.Name)
	for _, tool := range list.Tools {
		namespaced := g.uniqueToolName(up.server.Name, tool.Name)
		g.routes[namespaced] = route{upstream: up, tool: tool.Name, name: namespaced}
	}
	return nil
}

func (g *Gateway) uniqueToolName(serverName, toolName string) string {
	base := sanitizeName(serverName) + "__" + sanitizeName(toolName)
	if len(base) > 64 {
		base = base[:64]
	}
	name := base
	for i := 2; ; i++ {
		if _, exists := g.routes[name]; !exists {
			return name
		}
		suffix := fmt.Sprintf("_%d", i)
		if len(base)+len(suffix) > 64 {
			name = base[:64-len(suffix)] + suffix
		} else {
			name = base + suffix
		}
	}
}

func sanitizeName(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "server"
	}
	return b.String()
}

// Tools lists the aggregated tools for the debug page.
func (g *Gateway) Tools() []ToolInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]ToolInfo, 0, len(g.routes))
	for name, r := range g.routes {
		var tool mcp.Tool
		for _, candidate := range r.upstream.tools {
			if candidate.Name == r.tool {
				tool = candidate
				break
			}
		}
		schema, _ := json.Marshal(tool.InputSchema)
		out = append(out, ToolInfo{
			Name: name, Server: r.upstream.server.Name, Tool: r.tool,
			Description: tool.Description, InputSchema: schema,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Server != out[j].Server {
			return out[i].Server < out[j].Server
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// CallTool invokes an aggregated tool by name, as the gateway does for MCP
// clients, and records the call in the usage log.
func (g *Gateway) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	r, err := g.route(name)
	if err != nil {
		return nil, err
	}
	request := mcp.CallToolRequest{}
	request.Params.Name = r.tool
	request.Params.Arguments = args
	return g.invoke(ctx, r, request)
}

// route resolves a namespaced name, or a unique original tool name.
func (g *Gateway) route(name string) (route, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if r, ok := g.routes[name]; ok {
		return r, nil
	}
	var matches []route
	for _, candidate := range g.routes {
		if candidate.tool == name {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return route{}, fmt.Errorf("tool %q not found; use mymcp_find_tools", name)
	default:
		return route{}, fmt.Errorf("tool %q is ambiguous; use its namespaced name from mymcp_find_tools", name)
	}
}

func (g *Gateway) registerAll() {
	names := make([]string, 0, len(g.routes))
	for name := range g.routes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r := g.routes[name]
		var tool mcp.Tool
		for _, candidate := range r.upstream.tools {
			if candidate.Name == r.tool {
				tool = candidate
				break
			}
		}
		tool.Name = name
		g.mcp.AddTool(tool, g.routeHandler(r))
	}
}

func (g *Gateway) registerSearch() {
	findSchema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Words to search for"},"server":{"type":"string","description":"Only this server"}},"required":["query"]}`)
	callSchema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"A tool name from find_tools, or its original name"},"arguments":{"type":"object","description":"The tool arguments"}},"required":["name"]}`)
	g.mcp.AddTool(mcp.NewToolWithRawSchema("mymcp_find_tools",
		"Find MCP tools by name, server or description. MyMCP exposes tools on demand.", findSchema),
		g.findHandler)
	g.mcp.AddTool(mcp.NewToolWithRawSchema("mymcp_call_tool",
		"Call a tool returned by mymcp_find_tools.", callSchema),
		g.callToolHandler)
}

func (g *Gateway) findHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	query, _ := args["query"].(string)
	server, _ := args["server"].(string)
	words := strings.Fields(strings.ToLower(query))
	type result struct {
		Name        string `json:"name"`
		Server      string `json:"server"`
		Tool        string `json:"tool"`
		Description string `json:"description,omitempty"`
		InputSchema any    `json:"inputSchema,omitempty"`
		Score       int    `json:"-"`
	}
	var results []result
	for _, r := range g.routes {
		if server != "" && r.upstream.server.Name != server {
			continue
		}
		var tool mcp.Tool
		for _, candidate := range r.upstream.tools {
			if candidate.Name == r.tool {
				tool = candidate
				break
			}
		}
		score := 0
		name := strings.ToLower(r.name)
		description := strings.ToLower(tool.Description)
		for _, word := range words {
			if strings.Contains(name, word) {
				score += 5
			}
			if strings.Contains(strings.ToLower(r.upstream.server.Name), word) {
				score += 3
			}
			if strings.Contains(description, word) {
				score++
			}
		}
		if query == "" {
			score = 1
		}
		if score == 0 {
			continue
		}
		results = append(results, result{Name: r.name, Server: r.upstream.server.Name, Tool: r.tool, Description: tool.Description, InputSchema: tool.InputSchema, Score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Name < results[j].Name
	})
	if len(results) > 20 {
		results = results[:20]
	}
	raw, _ := json.MarshalIndent(results, "", "  ")
	return mcp.NewToolResultText(string(raw)), nil
}

func (g *Gateway) callToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	name, _ := args["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	r, err := g.route(name)
	if err != nil {
		return nil, err
	}
	call := mcp.CallToolRequest{}
	call.Params.Name = r.tool
	call.Params.Arguments = args["arguments"]
	return g.invoke(ctx, r, call)
}

func (g *Gateway) routeHandler(r route) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return g.invoke(ctx, r, request)
	}
}

func (g *Gateway) invoke(ctx context.Context, r route, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	started := time.Now()
	up := r.upstream
	up.mu.Lock()
	if up.client == nil {
		if err := g.open(up); err != nil {
			up.mu.Unlock()
			g.record(r, "error", err.Error(), started, 0, 0)
			return nil, err
		}
	}
	up.inFlight++
	up.lastUsed = time.Now()
	upstreamClient := up.client
	up.mu.Unlock()
	request.Params.Name = r.tool
	requestBytes, _ := json.Marshal(request)
	result, err := upstreamClient.CallTool(ctx, request)
	up.mu.Lock()
	up.inFlight--
	up.mu.Unlock()
	status := "ok"
	errText := ""
	if err != nil {
		status = "error"
		errText = err.Error()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	var responseBytes int64
	if result != nil {
		raw, _ := json.Marshal(result)
		responseBytes = int64(len(raw))
	}
	g.record(r, status, errText, started, int64(len(requestBytes)), responseBytes)
	return result, err
}

func (g *Gateway) record(r route, status, errText string, started time.Time, requestBytes, responseBytes int64) {
	call := &store.Call{
		Profile:       g.profile,
		Server:        r.upstream.server.Name,
		Tool:          r.tool,
		Method:        "tools/call",
		Status:        status,
		Error:         errText,
		LatencyMS:     time.Since(started).Milliseconds(),
		RequestBytes:  requestBytes,
		ResponseBytes: responseBytes,
	}
	if err := g.store.RecordCall(context.Background(), call); err != nil {
		log.Printf("record call: %v", err)
	}
}

func (g *Gateway) reapLoop() {
	ticker := time.NewTicker(reapEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			g.reapIdle(time.Now())
		case <-g.stop:
			return
		}
	}
}

func (g *Gateway) reapIdle(now time.Time) {
	for _, up := range g.upstreams {
		up.mu.Lock()
		idle := time.Duration(up.server.IdleSeconds) * time.Second
		if idle <= 0 {
			idle = 10 * time.Minute
		}
		if up.client != nil && up.inFlight == 0 && !up.lastUsed.IsZero() && now.Sub(up.lastUsed) > idle {
			if err := up.client.Close(); err != nil {
				log.Printf("idle close %s: %v", up.server.Name, err)
			}
			up.client = nil
		}
		up.mu.Unlock()
	}
}

// MCP returns the aggregated MCP server.
func (g *Gateway) MCP() *server.MCPServer { return g.mcp }

// Profile returns the profile name.
func (g *Gateway) Profile() string { return g.profile }

// Errors returns why servers failed to start, by name.
func (g *Gateway) Errors() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[string]string, len(g.errors))
	for k, v := range g.errors {
		out[k] = v
	}
	return out
}

// Status is what /healthz reports.
type Status struct {
	Profile    string            `json:"profile"`
	Version    string            `json:"version"`
	Servers    int               `json:"servers"`
	Running    int               `json:"running"`
	Tools      int               `json:"tools"`
	ToolSearch bool              `json:"toolSearch"`
	Errors     map[string]string `json:"errors,omitempty"`
	Endpoints  Endpoints         `json:"endpoints"`
}

// Endpoints are the profile's MCP URLs.
type Endpoints struct {
	Streamable string `json:"streamable"`
	SSE        string `json:"sse"`
}

// Status describes the gateway.
func (g *Gateway) Status() Status {
	running := 0
	names := map[string]bool{}
	for name, up := range g.upstreams {
		names[name] = true
		up.mu.Lock()
		if up.client != nil {
			running++
		}
		up.mu.Unlock()
	}
	g.mu.RLock()
	for name := range g.errors {
		names[name] = true
	}
	g.mu.RUnlock()
	base := strings.TrimSuffix(g.baseURL, "/")
	return Status{
		Profile:    g.profile,
		Version:    Version,
		Servers:    len(names),
		Running:    running,
		Tools:      len(g.routes),
		ToolSearch: g.search,
		Errors:     g.Errors(),
		Endpoints: Endpoints{
			Streamable: base + "/mcp/" + g.profile,
			SSE:        base + "/mcp/" + g.profile + "/sse",
		},
	}
}

// Close ends every upstream and stops the reaper.
func (g *Gateway) Close() {
	g.closeOnce.Do(func() { close(g.stop) })
	for _, up := range g.upstreams {
		up.mu.Lock()
		if up.client != nil {
			if err := up.client.Close(); err != nil {
				log.Printf("close %s: %v", up.server.Name, err)
			}
			up.client = nil
		}
		up.mu.Unlock()
	}
}

// Handler serves the profile over Streamable HTTP and SSE.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	streamable := server.NewStreamableHTTPServer(g.mcp,
		server.WithEndpointPath("/mcp/"+g.profile))
	sse := server.NewSSEServer(g.mcp,
		server.WithBaseURL(g.baseURL), server.WithBasePath("/mcp/"+g.profile))

	mux.Handle("/mcp/"+g.profile, streamable)
	mux.Handle("/mcp/"+g.profile+"/", streamable)
	mux.Handle(sse.CompleteSsePath(), sse)
	mux.Handle(sse.CompleteMessagePath(), sse)
	return mux
}

// stdioCommand is the local command or its docker sandbox wrapper.
func stdioCommand(cfg *store.Server, env map[string]string) (string, []string) {
	if cfg.Executor != "docker" {
		return cfg.Command, cfg.Args
	}
	args := []string{"run", "--rm", "-i", "--init"}
	network := cfg.Network
	if network == "" {
		network = "bridge"
	}
	args = append(args, "--network", network)
	for _, volume := range cfg.Volumes {
		args = append(args, "--volume", volume)
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--env", key)
	}
	args = append(args, cfg.Image)
	if cfg.Command != "" {
		args = append(args, cfg.Command)
		args = append(args, cfg.Args...)
	}
	return "docker", args
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	values := make(map[string]string, len(base)+len(extra))
	order := make([]string, 0, len(base)+len(extra))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = value
	}
	for key, value := range extra {
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = value
	}
	out := make([]string, 0, len(order))
	for _, key := range order {
		out = append(out, key+"="+values[key])
	}
	return out
}
