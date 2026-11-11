# JavaScript 客户端适配器

MyMCP 中**所有 Agent 客户端支持都是插件**：Cursor、Claude Code、Codex、
Goose、OpenCode 等内置适配器本身就是 JavaScript 插件，编译时嵌入程序。
新增或覆盖客户端不需要修改 Go 代码，安装插件后重载即可。

插件由 [moejs](https://github.com/Calcium-Ion/moejs) 执行，纯 Go、无 cgo。

## 内置适配器

内置插件放在 `internal/agent/builtin/<id>/`，通过 `go:embed` 随程序发布：

```text
internal/agent/builtin/codex/
├── plugin.json
├── index.js
└── icon.svg
```

- 启动时自动加载，不需要安装。
- 在「插件」页面显示为「内置适配器」。
- 查看全部适配器：

```sh
bin/mymcp plugin list
```

输出中的 `BUILTIN` 列表示是否为内置插件。

用户插件放在 `~/.mymcp/plugins/<id>/`。如果用户插件与内置插件使用相同
`id`，用户插件会覆盖内置插件，可用于本地定制。

## 安装用户插件

```sh
bin/mymcp plugin install ./my-agent-plugin
bin/mymcp plugin list
```

目录至少包含：

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
  "description": "My Agent MCP configuration",
  "entry": "index.js",
  "format": "json",
  "section": "mcpServers",
  "paths": {
    "darwin": "~/.my-agent/mcp.json",
    "linux": "~/.my-agent/mcp.json",
    "windows": "%APPDATA%\\MyAgent\\mcp.json"
  },
  "icon": "icon.svg"
}
```

字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 唯一标识，也是客户端 ID |
| `label` | 界面显示名称 |
| `format` | `json`、`jsonc`、`toml` 或 `yaml`；决定 MyMCP 如何解析和写回文档 |
| `section` | 配置文件里存放服务器条目的根节点，例如 `mcpServers`、`mcp_servers`、`servers`、`extensions` |
| `key` | 可选，原生条目键模板，例如 `{name}-{id}`；默认使用服务器名 |
| `paths` | 各平台配置路径；留空表示只支持预览，不写入文件 |
| `icon` | 可选，相对插件目录的图标，支持 SVG 或位图 |

`icon` 不填时，客户端列表使用 MyMCP 自带的品牌图标资源。

## 接口

插件是 ES Module，可导出 `render`、`load`、`detect`。

### `render(ctx) -> object | string`

返回**完整的新文档对象**（推荐），或返回**完整的新文件内容字符串**（旧式插件）。
返回对象时，MyMCP 按 `format` 序列化；返回字符串时，插件自己负责文件格式。
需要完全保留注释等格式细节的插件，可以不声明 `format`，直接返回字符串。

`ctx`：

```js
{
  mode: "router" | "direct" | "detach",
  endpoint: "http://127.0.0.1:8422/mcp/default", // router
  profile: "default",
  entry: "mymcp",          // router 条目名，也是默认的插件条目名
  bridge: "/Users/me/.mymcp/bin/mcp-pipe", // 需要桥接时使用的命令
  content: "current file text, empty when missing",
  document: { /* 当前配置解析后的对象 */ },
  path: "/home/me/.my-agent/mcp.json",
  os: "darwin",
  remove: ["old-entry"],   // 本次应删除的、之前由 MyMCP 管理的条目
  servers: [               // direct
    {
      name: "github",
      key: "github",       // 原生条目键；可能包含 id，如 deepchat
      id: "gh-1",
      transport: "stdio" | "http" | "sse",
      command: "npx",
      args: ["-y", "github-mcp"],
      url: "",
      env: {},
      headers: {}
    }
  ]
}
```

- `router`：写入一条指向 `endpoint` 的条目，键为 `ctx.entry`。
- `direct`：为 `servers` 中每个服务器写入原生条目，键为 `server.key`。
- `detach`：删除 `ctx.remove` 中的条目，并清理 MyMCP 自己的路由条目。

内置适配器使用 `ctx.document` 和 `ctx.remove`，因此切换 Router/Direct、
删除服务器或断开客户端时，不会留下旧条目，也不会影响用户自己的配置。

示例（完整可安装版本见 `examples/plugin-demo/`）：

```js
export function render(ctx) {
  const doc = ctx.document || {};
  doc.mcpServers ??= {};
  const servers = doc.mcpServers;

  for (const name of ctx.remove || []) delete servers[name];

  if (ctx.mode === "detach") {
    delete servers[ctx.entry || "mymcp"];
    return doc;
  }
  if (ctx.mode === "router") {
    servers[ctx.entry || "mymcp"] = { url: ctx.endpoint };
    return doc;
  }
  for (const server of ctx.servers || []) {
    servers[server.key || server.name] =
      server.transport === "stdio"
        ? { command: server.command, args: server.args || [], env: server.env || {} }
        : { url: server.url, headers: server.headers || {} };
  }
  return doc;
}

export function load(ctx) {
  const servers = (ctx.document || {}).mcpServers || {};
  const out = [];
  for (const [name, entry] of Object.entries(servers)) {
    if (name === "mymcp") continue;
    if (entry.command) {
      out.push({ name, transport: "stdio", command: entry.command, args: entry.args || [], env: entry.env || {} });
    } else if (entry.url) {
      out.push({ name, transport: "sse", url: entry.url, headers: entry.headers || {} });
    }
  }
  return out;
}
```

### `load(ctx) -> Server[]`（可选）

从现有配置读取服务器，用于：

```sh
bin/mymcp import --client my-agent
```

`ctx`：`{ content, document, path, os }`。`document` 是 MyMCP 按 `format`
解析后的对象。

返回：

```js
[{ name, transport, command, args, url, env, headers }]
```

**不要返回 MyMCP 的 `mymcp` 路由条目或 `mcp-pipe` 桥接条目。**
导入器会再过滤一次 MyMCP 自己写入的条目。

### `detect(ctx) -> boolean`（可选）

判断客户端是否已安装；没有此函数时 MyMCP 检查配置文件是否存在。
`ctx`：`{ path, os }`。

## 合并与冲突

- MyMCP 写文件前先备份到 `~/.mymcp/backups/<client>/`。
- 文档插件按 `section` 中的条目级别检测漂移：用户改了其他配置不会被打断，
  改了 MyMCP 条目则会提示冲突。
- 旧式字符串插件按整文件哈希检测漂移。
- Direct 模式拒绝包含 `secret://` 引用的服务器，请使用 Router 模式。

## 安全

插件是本机受信任代码。moejs 运行时只暴露 JavaScript 标准库，没有文件系统、
网络、定时器等全局对象；每次调用独立运行时，限制 64 MiB 内存和 10 秒执行时间。
插件只能通过返回的新文件内容或文档对象影响 MyMCP 写入的客户端配置。
