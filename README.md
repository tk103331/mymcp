# mymcp

## Project Overview

`mymcp` is a set of tools related to MCP Servers developed in Go.

- `mcp-pipe`: MCP pipe tool, supports one-to-one pipe with sse2sse, sse2stdio, stdio2sse, and stdio2stdio modes.
- `mcp-router`: MCP router, provides unified routing for multiple MCP Servers.
- `mcp-debug`: MCP debugging tool, a command-line interface for debugging MCP Servers.
- `mcp-manager`: Desktop MCP management application, built with Wails.

## Directory Structure

```
.
├── Makefile                # Project build scripts and common commands
├── go.mod / go.sum         # Go module dependency management
├── debug/                  # MCP Server debugging
│   └── main.go
├── manager/                # Desktop MCP management app, built with Wails
│   ├── app.go
│   ├── main.go
│   ├── bind/               # Go bindings for frontend/data
│   ├── data/               # Data models and file handling
│   ├── frontend/           # Frontend source code (Vue3 + Vite)
│   └── manager/            # Core management logic
├── pipe/                   # MCP pipeline, supports sse2sse, sse2stdio, stdio2sse, stdio2stdio
│   └── main.go
├── router/                 # MCP router, unified routing for multiple MCP Servers
│   ├── main.go
│   └── router.json         # Routing configuration file
└── README.md               # Project documentation
```

## Contribution & Development

Contributions via Issues and PRs are welcome. Please follow best practices for Go and Vue3 projects.
