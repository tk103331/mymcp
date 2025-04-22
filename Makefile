
mcp-debug:
	go build -o bin/mcpdebug debug/main.go

mcp-pipe:
	go build -o bin/mcppipe pipe/main.go

mcp-router:
	go build -o bin/mcp-router router/maing.go

mcp-manager:
	cd manager && wails build

