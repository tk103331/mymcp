<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/logo-dark.svg">
    <img src="docs/logo.svg" alt="MyMCP — 本地 MCP 网关" width="420">
  </picture>
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README_zh.md">简体中文</a>
</p>

# MyMCP

## 项目简介

`mymcp` 是一套用 Go 编写的本机优先 MCP 网关。所有 MCP 客户端共用一个本地端点，
并提供配置同步、加密密钥、OAuth、调用统计和 JavaScript 客户端适配器。

仓库包含：

- `mcp-manager`：基于 [MyGo](https://github.com/egoist/mygo) 的原生桌面应用。
  常驻菜单栏，关闭窗口后网关继续运行。
- `mymcp`：本地网关守护进程和命令行工具。
- `mcp-pipe`、`mcp-router`、`mcp-debug`：原有的 MCP 管道、路由和调试工具。

## 主要特性

- **每个配置组一个端点**：客户端连接
  `http://127.0.0.1:8422/mcp/<profile>`，由 MyMCP 聚合该配置组中的
  stdio、HTTP 和 SSE 服务器。
- **Router / Direct 同步**：可以只写入一条路由条目，也可以把每个服务器直接写入
  客户端配置。Direct 模式拒绝 `secret://` 引用。
- **加密密钥**：密钥以 AES-256-GCM 保存在 `secrets.enc`，配置中使用
  `secret://name` 引用。
- **OAuth**：授权码 + PKCE、令牌刷新、RFC 9728/8414 自动发现和动态客户端注册。
- **用量与访问控制**：SQLite 调用记录、客户端网关 Key、频率限制、HTTPS/mTLS
  和 Docker 沙箱执行器。
- **可移植配置组**：导出/导入带签名的配置包，不携带密钥明文。
- **JavaScript 适配器**：通过 `render`、`load`、`detect` 钩子扩展新客户端，
  运行在 [moejs](https://github.com/Calcium-Ion/moejs) 中。
- **原生界面**：中英文、浅色/深色主题，包含概览、服务器、配置组、服务目录、客户端、
  调试、密钥、插件、用量和设置页面。

## 方向

MyMCP 是一个 **本机优先的 MCP 网关**：所有客户端共用一个端点，统一配置同步、
密钥管理和调用统计。相关调研、竞品对比和架构设计见
[docs/mcp-gateway-design.md](docs/mcp-gateway-design.md)；JavaScript 客户端适配器见
[docs/plugins.md](docs/plugins.md)。MVP 范围、验证记录和已知限制见
[docs/mvp-readiness.md](docs/mvp-readiness.md)。

## 下载

预构建的桌面应用和命令行压缩包发布在
[Releases](https://github.com/tk103331/mymcp/releases) 页面。

## 目录结构

```
.
├── cmd/                    # 命令行与桌面端入口
│   ├── mcp-debug/          # MCP Server 调试
│   ├── mcp-manager/        # 桌面应用入口（MyGo）
│   ├── mcp-pipe/           # MCP 管道，支持 sse2sse、sse2stdio、stdio2sse、stdio2stdio
│   ├── mcp-router/         # MCP 路由，含 router.json
│   └── mymcp/              # 本地网关守护进程与命令行
├── internal/
│   ├── app/                # 原生 UI：页面、对话框、客户端配置、服务目录
│   ├── configsync/         # 客户端适配器、plan/apply/detach、导入
│   ├── data/               # 本地数据与 JSON 设置
│   ├── gateway/            # MCP 聚合、鉴权、用量接口
│   ├── importer/           # 从现有 Agent 导入服务器
│   ├── manager/            # 旧工作空间运行时
│   ├── model/              # 各工具共用的服务配置
│   ├── oauth/              # OAuth 发现、登录与令牌刷新
│   ├── plugin/             # 基于 moejs 的 JavaScript 适配器
│   ├── portable/           # 带签名的配置包导出/导入
│   ├── proxy/              # MCP 代理与 SSE 传输
│   ├── secrets/            # 加密密钥库
│   └── store/              # SQLite 注册表与用量存储
├── docs/                   # 架构、插件文档与 Logo
├── examples/plugin-demo/   # JavaScript 客户端适配器示例
├── locales/                # 中英文界面文案
├── resources/              # 应用图标与生成的平台 helper
├── scripts/                # 构建脚本（含内置 mcp-pipe）
├── .github/workflows/      # CI 与发布自动化
├── mygo.json               # 桌面应用名称、版本与构建配置
├── Makefile                # 构建脚本与常用命令
└── go.mod / go.sum         # Go Module 依赖管理
```

## 环境要求

- Go 1.27 或更高版本；MyGo 构建桌面端，缺少工具链时会自动下载。
- 桌面端需要 macOS 12 及以上、Windows 10/11，或带 GTK 3 的 Linux。
- 桌面端和命令行数据保存在 `~/.mymcp`：`settings.json`、`mymcp.db`、
  `secrets.enc`、`master.key`、`sync.json`、`backups/` 和 `plugins/`。

## 本地网关

```sh
make mymcp
bin/mymcp add demo -- python3 ./fake-mcp.py
bin/mymcp add github --url https://api.githubcopilot.com/mcp/
bin/mymcp secret set github/token          # 从 stdin 读取
bin/mymcp serve                            # http://127.0.0.1:8422
bin/mymcp client apply cursor              # 写入路由条目
bin/mymcp client apply cursor --direct     # 或直接写入每个服务器
bin/mymcp client plan cursor               # 先预览再应用
bin/mymcp client restore cursor            # 恢复上一次备份
bin/mymcp usage --stats
bin/mymcp key create cursor --profile default --rate 60
bin/mymcp oauth discover github            # RFC 9728/8414 + 动态客户端注册
bin/mymcp oauth login github               # 授权码 + PKCE
bin/mymcp oauth status github
bin/mymcp import --client cursor
bin/mymcp import --only github,postgres  # 选择服务器
bin/mymcp plugin install ./examples/plugin-demo
bin/mymcp profile export default --output work.mymcp.json --sign-key bundle.key
bin/mymcp profile import work.mymcp.json --name shared --verify-key bundle.pub
bin/mymcp profile keygen --private bundle.key --public bundle.pub
bin/mymcp add docker-demo --executor docker --image mcp/demo --network none
bin/mymcp doctor
```

配置包只携带 `secret://` 引用，不携带密钥值；如果 Token/Password/Authorization
等敏感字段仍是字面值，导出会被拒绝，需要先保存为密钥。

修改服务器或配置组后需要重启 `mymcp serve`；桌面应用在概览页面提供重启按钮。

桌面应用包含：

- **概览**：网关健康状态、端点、服务器/工具/客户端数量、最近调用和快捷操作。
- **服务器**：新增、导入、编辑、启停、删除上游服务器，查看所属配置组和启动错误。
- **配置组**：创建配置组、挂载服务器、复制配置组端点、管理客户端 Key。
- **服务目录**：从内置目录把服务器添加到配置组。
- **调试**：查看聚合后的工具列表，编辑 JSON 参数，通过运行中的网关调用工具，
  并查看返回结果和延迟。
- **客户端**：检测已安装客户端，预览并应用路由条目，或断开托管配置；MyMCP
  写入后被修改的条目会标记为“已修改”。
- **密钥**：保存通过 `secret://名称` 引用的加密 API Key 和 Token。
- **插件**：安装、重载、移除 JavaScript 客户端适配器。
- **用量**：从同一个 SQLite 数据读取调用次数、错误、延迟和最近活动。
- **设置**：语言、主题、开机启动和网关地址。

守护进程还提供 `/dashboard` 本地用量面板、`/api/usage/*` JSON 接口和
`/metrics` Prometheus 指标。`--tool-search` 会用
`mymcp_find_tools`/`mymcp_call_tool` 替代过长的工具列表。
`serve --tls-cert/--tls-key` 启用 HTTPS，`--client-ca` 要求客户端证书。
`key create` 生成客户端网关 Key；远程请求必须带 Key，本机请求在
`--require-key` 下也必须带。

客户端导入支持 JSON、JSONC、TOML 和 YAML，自动去重，并跳过 MyMCP 自己写入的
路由条目和 `mcp-pipe` 桥接条目。桌面端可以按 Agent 过滤、选择要导入的服务器和
目标配置组；命令行使用 `--client ID` 与 `--only name1,name2`。

Claude Desktop、Codex、Kiro 和 Zed 使用 `mcp-pipe` 桥接。桌面端会内置
`mcp-pipe`，首次启动时复制到 `~/.mymcp/bin/mcp-pipe`，客户端配置直接写入该
绝对路径，因此不依赖 `PATH`。OAuth 服务器可以在服务器编辑对话框中直接登录，
令牌保存在密钥库中。

## 构建

```sh
make mymcp                          # 本地网关命令行，输出到 bin/
make mcp-debug mcp-pipe mcp-router  # 其他命令行工具，输出到 bin/
make mcp-manager                    # MyGo 桌面应用，输出到 build/，内置 mcp-pipe
make dev                            # 启动桌面应用，修改后自动重新构建
make test                           # go test ./...
```

`make mcp-manager` 会先构建当前平台的 `mcp-pipe` 并打包；直接运行
`go tool mygo build` 时使用已有 helper，或回退到 `PATH` 中的 `mcp-pipe`。

`mcp-manager` 使用 [MyGo](https://github.com/egoist/mygo) 原生绘制界面，不再依赖
WebView 或前端工程。应用配置在 `mygo.json`，界面文案在 `locales/en.json` 与
`locales/zh-Hans.json`。界面支持中英文切换，并可跟随系统或强制浅色/深色，
主题切换后立即生效。

## 发布

1. 修改 `mygo.json` 中的 `version`。
2. 提交并推送。
3. 打标签并推送：

```sh
git tag v0.3.1
git push origin v0.3.1
```

[`.github/workflows/release.yml`](.github/workflows/release.yml) 会校验标签与
`mygo.json` 的版本一致，运行测试，构建桌面应用和命令行压缩包，并在 GitHub
Release 中发布，附带 SHA-256 校验和。在配置 Developer ID 签名和公证之前，
macOS 应用使用 ad-hoc 签名。

## 许可证

MIT，见 [LICENSE](LICENSE)。

## 贡献与开发

欢迎提交 Issue 和 PR，请确保遵循 Go 相关最佳实践。
