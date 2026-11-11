package app

import (
	"testing"

	"github.com/tk103331/mymcp/internal/gateway"
)

func TestGroupDebugTools(t *testing.T) {
	tools := []gateway.ToolInfo{
		{Server: "alpha", Tool: "one"},
		{Server: "alpha", Tool: "two"},
		{Server: "beta", Tool: "three"},
	}
	groups := groupDebugTools(tools)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	if groups[0].Server != "alpha" || len(groups[0].Tools) != 2 {
		t.Fatalf("first group = %+v", groups[0])
	}
	if groups[1].Server != "beta" || len(groups[1].Tools) != 1 {
		t.Fatalf("second group = %+v", groups[1])
	}
	if got := groupDebugTools(nil); len(got) != 0 {
		t.Fatalf("empty tools produced %d groups", len(got))
	}
}
