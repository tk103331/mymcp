package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

func TestGatewayAggregatesAndRecords(t *testing.T) {
	// An upstream MCP server with one tool.
	upstream := server.NewMCPServer("demo", "1.0.0")
	upstream.AddTool(mcp.NewTool("echo",
		mcp.WithDescription("Echo text"),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to echo")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := request.Params.Arguments.(map[string]any)
		text, _ := args["text"].(string)
		return mcp.NewToolResultText(text), nil
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + listener.Addr().String()
	sse := server.NewSSEServer(upstream, server.WithBaseURL(base))
	upstreamHTTP := httptest.NewUnstartedServer(sse)
	upstreamHTTP.Listener = listener
	upstreamHTTP.Start()
	defer upstreamHTTP.Close()

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "mymcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.EnsureDefaultProfile(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertServer(ctx, &store.Server{
		Name:      "demo",
		Transport: "sse",
		URL:       base + "/sse",
		Enabled:   true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.AttachServer(ctx, "default", "demo", true); err != nil {
		t.Fatal(err)
	}
	secretStore, err := secrets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	g, err := New(Options{Profile: "default", Store: db, Secrets: secretStore, BaseURL: "http://gateway"})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if status := g.Status(); status.Servers != 1 || status.Tools != 1 {
		t.Fatalf("status = %+v", status)
	}

	gatewayHTTP := httptest.NewServer(g.Handler())
	defer gatewayHTTP.Close()

	transportClient, err := transport.NewStreamableHTTP(gatewayHTTP.URL + "/mcp/default")
	if err != nil {
		t.Fatal(err)
	}
	mcpClient := client.NewClient(transportClient)
	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := mcpClient.Start(timeout); err != nil {
		t.Fatal(err)
	}
	defer mcpClient.Close()
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1.0.0"}
	if _, err := mcpClient.Initialize(timeout, initRequest); err != nil {
		t.Fatal(err)
	}
	tools, err := mcpClient.ListTools(timeout, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "demo__echo" {
		t.Fatalf("tools = %+v", tools.Tools)
	}
	call := mcp.CallToolRequest{}
	call.Params.Name = "demo__echo"
	call.Params.Arguments = map[string]any{"text": "hello"}
	result, err := mcpClient.CallTool(timeout, call)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("result = %+v", result)
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok || text.Text != "hello" {
		t.Fatalf("content = %+v", result.Content[0])
	}

	calls, err := db.RecentCalls(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].Tool != "echo" || calls[0].Status != "ok" {
		t.Fatalf("usage calls = %+v", calls)
	}

	// The idle reaper stops the upstream but keeps the tool surface; the next
	// call starts it again.
	g.reapIdle(time.Now().Add(24 * time.Hour))
	if g.Status().Running != 0 {
		t.Fatalf("upstream still running after reap: %+v", g.Status())
	}
	result, err = mcpClient.CallTool(timeout, call)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("restarted result = %+v", result)
	}

	// The debug API exposes the same route and records its calls.
	infos := g.Tools()
	if len(infos) != 1 || infos[0].Name != "demo__echo" || len(infos[0].InputSchema) == 0 {
		t.Fatalf("debug tools = %+v", infos)
	}
	debugResult, err := g.CallTool(ctx, infos[0].Name, map[string]any{"text": "debug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(debugResult.Content) != 1 {
		t.Fatalf("debug result = %+v", debugResult)
	}
}

func TestToolSearch(t *testing.T) {
	upstream := server.NewMCPServer("demo", "1.0.0")
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("tool%d", i)
		upstream.AddTool(mcp.NewTool(name, mcp.WithDescription("Tool "+name),
			mcp.WithString("text", mcp.Required())),
			func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				args, _ := request.Params.Arguments.(map[string]any)
				text, _ := args["text"].(string)
				return mcp.NewToolResultText(request.Params.Name + ":" + text), nil
			})
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + listener.Addr().String()
	sse := server.NewSSEServer(upstream, server.WithBaseURL(base))
	httpServer := httptest.NewUnstartedServer(sse)
	httpServer.Listener = listener
	httpServer.Start()
	defer httpServer.Close()

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "mymcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.EnsureDefaultProfile(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertServer(ctx, &store.Server{Name: "demo", Transport: "sse", URL: base + "/sse", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.AttachServer(ctx, "default", "demo", true); err != nil {
		t.Fatal(err)
	}
	secretStore, err := secrets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(Options{Profile: "default", Store: db, Secrets: secretStore, BaseURL: "http://gateway",
		ToolSearch: true, ToolSearchThreshold: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	gatewayHTTP := httptest.NewServer(g.Handler())
	defer gatewayHTTP.Close()
	transportClient, err := transport.NewStreamableHTTP(gatewayHTTP.URL + "/mcp/default")
	if err != nil {
		t.Fatal(err)
	}
	mcpClient := client.NewClient(transportClient)
	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := mcpClient.Start(timeout); err != nil {
		t.Fatal(err)
	}
	defer mcpClient.Close()
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1.0.0"}
	if _, err := mcpClient.Initialize(timeout, initRequest); err != nil {
		t.Fatal(err)
	}
	tools, err := mcpClient.ListTools(timeout, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 2 {
		t.Fatalf("search surface = %+v", tools.Tools)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	if !names["mymcp_find_tools"] || !names["mymcp_call_tool"] {
		t.Fatalf("search surface = %+v", tools.Tools)
	}
	find := mcp.CallToolRequest{}
	find.Params.Name = "mymcp_find_tools"
	find.Params.Arguments = map[string]any{"query": "tool3"}
	found, err := mcpClient.CallTool(timeout, find)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Content) != 1 {
		t.Fatalf("find result = %+v", found)
	}
	text, ok := found.Content[0].(mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "demo__tool3") {
		t.Fatalf("find content = %+v", found.Content[0])
	}
	call := mcp.CallToolRequest{}
	call.Params.Name = "mymcp_call_tool"
	call.Params.Arguments = map[string]any{"name": "demo__tool3", "arguments": map[string]any{"text": "hi"}}
	result, err := mcpClient.CallTool(timeout, call)
	if err != nil {
		t.Fatal(err)
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok || content.Text != "tool3:hi" {
		t.Fatalf("call content = %+v", result.Content[0])
	}
}

func TestManagerUsageEndpoints(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "mymcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.EnsureDefaultProfile(ctx); err != nil {
		t.Fatal(err)
	}
	secretStore, err := secrets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ctx, db, secretStore, ManagerOptions{BaseURL: "http://gateway"})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ts := httptest.NewServer(manager.Handler())
	defer ts.Close()

	for path, want := range map[string]string{
		"/api/usage/summary": "calls",
		"/api/usage/series":  "[",
		"/metrics":           "mymcp_calls_total",
		"/dashboard":         "MyMCP usage",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), want) {
			t.Fatalf("%s = %d %q", path, resp.StatusCode, string(body))
		}
	}
}

func TestDockerStdioCommand(t *testing.T) {
	cfg := &store.Server{
		Executor: "docker", Image: "mcp/demo", Network: "none",
		Volumes: []string{"/tmp/data:/data"}, Command: "node", Args: []string{"server.js"},
	}
	command, args := stdioCommand(cfg, map[string]string{"TOKEN": "x"})
	if command != "docker" {
		t.Fatalf("command = %q", command)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"run --rm -i --init", "--network none", "--volume /tmp/data:/data", "--env TOKEN", "mcp/demo node server.js"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
	local, localArgs := stdioCommand(&store.Server{Command: "npx", Args: []string{"-y", "demo"}}, nil)
	if local != "npx" || len(localArgs) != 2 {
		t.Fatalf("local = %q %v", local, localArgs)
	}
}
