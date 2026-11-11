.PHONY: all mcp-debug mcp-pipe mcp-router mymcp mcp-pipe-bundle mcp-manager dev test clean

VERSION ?= $(shell sed -n 's/.*"version": "\([^"]*\)".*/\1/p' mygo.json)

all: mymcp mcp-debug mcp-pipe mcp-router mcp-manager

mcp-debug:
	go build -o bin/mcp-debug ./cmd/mcp-debug

mcp-pipe:
	go build -o bin/mcp-pipe ./cmd/mcp-pipe

mcp-router:
	go build -o bin/mcp-router ./cmd/mcp-router

# Local MCP gateway daemon and CLI.
mymcp:
	go build -ldflags "-X github.com/tk103331/mymcp/internal/gateway.Version=$(VERSION)" -o bin/mymcp ./cmd/mymcp

# Build the mcp-pipe helper the desktop app bundles for bridged clients.
mcp-pipe-bundle:
	./scripts/bundle-mcp-pipe.sh

# Desktop app, packaged with MyGo.
mcp-manager: mcp-pipe-bundle
	go tool mygo build

# Desktop app, rebuilt on changes.
dev: mcp-pipe-bundle
	go tool mygo dev

test:
	go test ./...

clean:
	rm -rf bin build resources/*/bin
