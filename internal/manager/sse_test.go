package manager

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/model"
	"github.com/tk103331/mymcp/internal/proxy"
)

func TestServeMCPRoutesByPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := &proxy.ProxyServer{ID: "demo"}
	server.ServerInfo = mcp.Implementation{Name: "demo", Version: "1.0.0"}
	AddProxyRoute(server)
	defer RemoveProxyRoute(server)

	httpServer := httptest.NewServer(http.HandlerFunc(serveMCP))
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/demo/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "endpoint") {
		t.Fatalf("first event = %q, want the endpoint event", line)
	}

	unknown, err := http.Get(httpServer.URL + "/missing/sse")
	if err != nil {
		t.Fatal(err)
	}
	unknown.Body.Close()
	if unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want 404", unknown.StatusCode)
	}
}

func TestRemoveProxyRoute(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := &proxy.ProxyServer{ID: "demo"}
	server.ServerInfo = mcp.Implementation{Name: "demo", Version: "1.0.0"}
	AddProxyRoute(server)
	RemoveProxyRoute(server)

	httpServer := httptest.NewServer(http.HandlerFunc(serveMCP))
	defer httpServer.Close()
	resp, err := http.Get(httpServer.URL + "/demo/sse")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("removed route status = %d, want 404", resp.StatusCode)
	}
}

func TestAutoStartWorkspaces(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := data.SaveWorkspace(&data.Workspace{Name: "always", AutoRun: true}); err != nil {
		t.Fatal(err)
	}
	if err := data.SaveWorkspace(&data.Workspace{Name: "manual"}); err != nil {
		t.Fatal(err)
	}
	if err := data.SaveWorkspace(&data.Workspace{Name: "off", AutoRun: true, Disabled: true}); err != nil {
		t.Fatal(err)
	}
	auto := AutoStartWorkspaces()
	if len(auto) != 1 || auto[0].Name != "always" {
		t.Fatalf("AutoStartWorkspaces = %+v, want only always", auto)
	}
}

func TestDisabledWorkspaceDoesNotStart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workspace := &data.Workspace{Name: "off", Disabled: true}
	if err := data.SaveWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if err := StartWorkspace(workspace.ID); err == nil {
		t.Fatal("StartWorkspace started a disabled workspace")
	}
	config := &model.ServerConfig{Workspace: workspace.ID, Name: "demo", Transport: "stdio", Cmd: "true"}
	if err := data.SaveServerConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := StartServerInstance(config.ID); err == nil {
		t.Fatal("StartServerInstance started a service of a disabled workspace")
	}
}
