# Demo agent plugin

A minimal MyMCP JavaScript plugin. Install it with:

```sh
bin/mymcp plugin install examples/plugin-demo
bin/mymcp plugin list
bin/mymcp client list
bin/mymcp client apply demo-agent --addr 127.0.0.1:8422
```

`render` receives `ctx.document` (the parsed configuration) and returns the next
document. MyMCP serializes it as JSON, backs up the file and detects drift per
entry. `load` returns servers for `mymcp import`. MyMCP entries (`mymcp`) are
skipped on import.

The built-in adapters under `internal/agent/builtin/` are the best reference for
JSON, TOML and YAML clients.
