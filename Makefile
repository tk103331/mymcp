
mcp-debug:
	go build -o bin/mcp-debug debug/main.go

mcp-pipe:
	go build -o bin/mcp-pipe pipe/main.go

mcp-router:
	go build -o bin/mcp-router router/main.go

mcp-manager:
	cd manager && wails build

