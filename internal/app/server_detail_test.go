package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tk103331/mymcp/internal/gateway"
	"github.com/tk103331/mymcp/internal/store"
)

func TestServerDetailDialog(t *testing.T) {
	a := newTestApp()
	a.router.Transition = ui.TransitionNone
	a.gwDetailOpen = true
	a.gwDetail = &store.Server{
		Name: "github", Transport: "stdio", Command: "npx",
		Args: []string{"-y", "server-github"}, Enabled: true,
		Env: map[string]string{"GITHUB_TOKEN": "secret://github/token"},
	}
	a.gwDetailProfile = "default"
	a.gwServerProfiles = map[string][]string{"github": {"default"}}
	a.gwDetailData = &gateway.ServerDetail{
		Server: "github", Profile: "default",
		Info:               mcp.Implementation{Name: "github-mcp", Version: "1.2.0"},
		ToolsSupported:     true,
		PromptsSupported:   true,
		ResourcesSupported: true,
		Tools: []mcp.Tool{
			mcp.NewTool("search", mcp.WithDescription("Search repositories"),
				mcp.WithString("query", mcp.Required())),
			mcp.NewTool("issue", mcp.WithDescription("Read an issue")),
		},
		Prompts: []mcp.Prompt{mcp.NewPrompt("review", mcp.WithPromptDescription("Review code"))},
		Resources: []mcp.Resource{
			mcp.NewResource("repo://readme", "README", mcp.WithMIMEType("text/markdown")),
		},
		ResourceTemplates: []mcp.ResourceTemplate{
			mcp.NewResourceTemplate("repo://{path}", "File"),
		},
	}
	a.router.Push("/servers")
	tt := ui.NewTester(a.view, 1100, 900)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	for _, text := range []string{"github", "环境变量", "GITHUB_TOKEN=secret://github/token", "工具 (2)", "search", "提示词 (1)", "资源 (2)"} {
		if !tt.HasText(text) {
			t.Errorf("server detail is missing %q: %q", text, tt.Texts())
		}
	}
	if tt.HasText("请求头") {
		t.Error("stdio details show headers")
	}
}

func TestServerDetailHTTPHeaders(t *testing.T) {
	a := newTestApp()
	a.router.Transition = ui.TransitionNone
	a.gwDetailOpen = true
	a.gwDetail = &store.Server{
		Name: "remote", Transport: "http", URL: "https://example.com/mcp", Enabled: true,
		Headers: map[string]string{"Authorization": "secret://remote/token"},
	}
	a.gwDetailProfile = "default"
	a.gwServerProfiles = map[string][]string{"remote": {"default"}}
	a.router.Push("/servers")
	tt := ui.NewTester(a.view, 1100, 800)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	for _, text := range []string{"remote", "https://example.com/mcp", "请求头", "Authorization=secret://remote/token"} {
		if !tt.HasText(text) {
			t.Errorf("server detail is missing %q: %q", text, tt.Texts())
		}
	}
	if tt.HasText("环境变量") {
		t.Error("HTTP details show environment variables")
	}
}
