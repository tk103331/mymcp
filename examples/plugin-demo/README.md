# Demo agent plugin

A minimal MyMCP JavaScript plugin. Install it with:

```sh
bin/mymcp plugin install examples/plugin-demo
bin/mymcp client list
bin/mymcp client apply demo-agent --addr 127.0.0.1:8422
```

`render` owns the whole configuration file: it receives the current content
and returns the next content. `load` returns servers for `mymcp import`.
MyMCP entries (`mymcp`) are skipped on import.
