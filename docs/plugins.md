# JavaScript 插件

MyMCP 可以用本地 JavaScript 插件扩展客户端支持，而不需要重新编译程序。
插件由 [moejs](https://github.com/Calcium-Ion/moejs) 执行，纯 Go、无 cgo。

## 安装

```sh
bin/mymcp plugin install ./my-agent-plugin
bin/mymcp plugin list
```

插件放在 `~/.mymcp/plugins/<id>/`，至少包含：

```text
my-agent-plugin/
├── plugin.json
└── index.js
```

`plugin.json`：

```json
{
  "id": "my-agent",
  "label": "My Agent",
  "version": "1.0.0",
  "entry": "index.js",
  "format": "json",
  "paths": {
    "darwin": "~/.my-agent/mcp.json",
    "linux": "~/.my-agent/mcp.json",
    "windows": "%APPDATA%\\MyAgent\\mcp.json"
  },
  "icon": "icon.svg"
}
```

`icon` 可选；相对路径相对于插件目录。支持 SVG 或位图。

## 接口

插件是 ES Module，导出：

### `render(ctx) -> string`

返回**完整的新文件内容**。MyMCP 负责备份、原子写入、冲突检测和 JSONC/TOML/YAML
之外的格式由插件自己处理。

`ctx`：

```js
{
  mode: "router" | "direct" | "detach",
  endpoint: "http://127.0.0.1:8422/mcp/default", // router
  profile: "default",
  content: "current file text, empty when missing",
  path: "/home/me/.my-agent/mcp.json",
  os: "darwin",
  servers: [
    {
      name: "github",
      transport: "stdio" | "http" | "sse",
      command: "npx",
      args: ["-y", "github-mcp"],
      url: "",
      env: {},
      headers: {}
    }
  ] // direct
}
```

- `router`：写入一条指向 `endpoint` 的条目。
- `direct`：为 `servers` 中每个服务器写入原生条目。
- `detach`：只移除 MyMCP 写入的条目。

### `load(ctx) -> Server[]`（可选）

从现有配置读取服务器，用于：

```sh
bin/mymcp import --client my-agent
```

`ctx`：`{ content, path, os }`。

返回：

```js
[{ name, transport, command, args, url, env, headers }]
```

**不要返回 MyMCP 的 `mymcp` 路由条目或 `mcp-pipe` 桥接条目。**

### `detect(ctx) -> boolean`（可选）

判断客户端是否已安装；没有此函数时 MyMCP 检查配置文件是否存在。

## 示例

`examples/plugin-demo/` 是一个可安装的示例。

## 安全

插件是本机受信任代码。moejs 运行时只暴露 JavaScript 标准库，没有文件系统、
网络、定时器等全局对象；每次调用独立运行时，限制 64 MiB 内存和 10 秒执行时间。
插件只能通过返回的新文件内容影响 MyMCP 写入的客户端配置。
