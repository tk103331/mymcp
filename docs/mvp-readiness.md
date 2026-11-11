# MVP 发布检查

> 状态：2026-11-11 · 对应 `v0.3.2` 候选版本 · MIT License

## 结论

核心链路已经闭环，可以作为本机开发者工具发布 MVP：

1. 首次启动自动创建 `~/.mymcp` 数据目录并启动网关。
2. 添加服务器、选择 Profile、连接客户端。
3. 通过 `secret://` 保存上游密钥。
4. 客户端调用进入本地网关，产生用量记录。
5. 客户端配置可预览、应用、强制断开和恢复备份。
6. 网关可重启、查看上游错误和工具数，退出时关闭上游进程。

发布前仍建议完成 macOS Developer ID 签名和公证。桌面端已内置桥接所需的
`mcp-pipe`；只有单独使用 CLI 时才回退到 `PATH`。

## 已覆盖并验证的流程

### 首次启动

- 创建 `~/.mymcp`（0700）。
- 创建 `mymcp.db`（0600）、`secrets.enc`、`master.key`（0600）。
- 自动启动本地网关，默认 `127.0.0.1:8422`。
- 端口被占用时显示红色网关状态和具体错误，可在设置中修改地址后重试。
- 单实例锁：再次启动会唤起已有窗口。

### 页面结构

- **概览**：网关状态、端点、统计、最近调用和快捷操作。
- **服务器**：服务器注册表、搜索、启停、所属配置组和上游错误。
- **配置组**：创建、挂载服务器、复制端点、管理客户端 Key。
- **服务目录**：从内置目录安装服务器到配置组。
- **客户端**、**调试**、**密钥**、**插件**、**用量**、**设置** 保持独立入口。

### 服务器与 Profile

- 添加 stdio / HTTP / SSE 服务器。
- 命令参数支持引号和转义；参数在编辑时可往返。
- Docker 执行器、网络、挂载卷和环境变量。
- 启用/禁用、编辑、删除、重启网关生效。
- Profile 新建、描述、服务器挂载/卸载。
- 网关状态显示运行服务器数、工具数和上游错误。
- 服务器、Profile 名称重复时阻止保存。
- URL 必须是 `http://` 或 `https://`。

### 客户端配置

- Router 模式写入客户端原生配置。
- 桌面构建内置 `mcp-pipe`；首次启动复制到 `~/.mymcp/bin/mcp-pipe`，桥接客户端
  使用该绝对路径，不依赖 `PATH`。
- SSE 客户端写入 `/mcp/<profile>/sse`。
- Claude Desktop、Codex、Kiro、Zed 使用 `mcp-pipe` 桥接。
- 应用前预览，冲突时明确显示“覆盖写入”。
- 托管条目被用户修改后标记“已修改”。
- 强制断开需要二次确认。
- `mymcp client restore` 可恢复上一次备份。

### 密钥与 OAuth

- Secrets 页面新增/更新/删除加密值。
- 服务器 Env/Headers 使用 `secret://name` 引用。
- OAuth 服务器可以在编辑对话框中登录，令牌写入密钥库。
- OAuth 令牌刷新失败会显示在网关健康信息中。

### MCP 调试

- Debug 页面列出当前 Profile 聚合后的工具。
- 支持按名称、服务器和描述搜索工具。
- 展示工具的 JSON Input Schema。
- 编辑 JSON 参数并通过运行中的网关实际调用。
- 显示结果 JSON 和调用延迟，调用会进入用量记录。
- 网关未运行时可从调试页面启动网关。

### 用量与访问控制

- 每次 `tools/call` 记录调用、错误、延迟和字节数。
- 用量页按 24h / 7d / 30d 过滤，最近调用同样受时间范围限制。
- 客户端 Key 创建、吊销、频率限制和过期时间。
- 远程请求必须带 Key；远程管理页面被拒绝。
- HTTPS、mTLS 和 Docker 沙箱由 CLI 提供。

### 插件与导入

- 安装/重载/移除 JavaScript 客户端适配器。
- 插件运行在 moejs，64 MiB、10 秒限制，无文件/网络/定时器。
- 导入现有客户端配置，过滤 MyMCP 路由和 `mcp-pipe` 桥接条目。
- 导入前扫描并列出检测到的服务器，可按 Agent 过滤，选择单个服务器、全选/全不选，
  并选择目标配置组。
- CLI `mymcp import --only name1,name2` 支持同样的选择。
- 重复服务器会附加到目标 Profile，而不是丢弃。
- 导入失败会在界面显示服务器名和错误。

### 退出与恢复

- 退出时关闭网关上游、工作空间服务和数据库。
- 网关地址或客户端配置变化不会静默写入。
- 客户端文件写入前备份到 `~/.mymcp/backups/<client>/`。
- 配置包导出拒绝 Token/Password/Authorization 等敏感字段的字面值，要求先改为
  `secret://` 引用。

## 验证记录

```text
go test ./...                    通过
go test -race ./...              通过
go vet ./...                     通过
go tool mygo vet                 通过
go tool mygo i18n check          通过（416 条中英文文案）
go tool mygo build               通过
```

CI：`.github/workflows/ci.yml` 在 push / PR 上运行同一组测试、vet、文案检查和构建。

手工验证：

- 启动桌面 App，网关自动监听配置端口。
- 退出 App 后 8421/8422 监听关闭。
- 端口冲突时日志出现 `bind: address already in use`，界面显示错误。
- 启动 fake stdio MCP server，`tools/list` 和 `tools/call` 正常。
- 调用后 `mymcp usage --stats` 记录 1 次成功调用。
- `mymcp client apply cursor` 写入正确端点，`detach` 移除托管条目。
- SSE 桥接 `mcp-pipe --mode sse2stdio --url .../sse` 可完成 initialize 和 tools/list。
- DMG 内含 `Contents/Resources/bin/mcp-pipe`；启动后复制到
  `~/.mymcp/bin/mcp-pipe`，客户端配置写入该绝对路径。
- macOS universal 构建会把 arm64/amd64 helper 合并为 universal binary。
- `mymcp key create` + `serve --require-key`：无 Key 返回 401，有 Key 返回 200。
- 不存在的 Profile 不会留下孤儿服务器。

## 发布前已知限制

1. **macOS 签名**：当前为 ad-hoc 签名。发布给其他 Mac 前需要 Developer ID
   和 notarization，否则 Gatekeeper 会提示。
2. **Direct 模式**：仅 CLI 提供，桌面 UI 只提供 Router 模式。
3. **格式保真**：JSONC/TOML/YAML 写入会重新序列化，可能丢失注释和原始排版；
   写入前会备份，可用 `client restore` 恢复。
4. **导入密钥**：从现有客户端导入的 Env/Headers 字面值仍保存在 SQLite 中，
   不会自动迁移到密钥库；建议后续用 `secret://` 替换。
5. **密钥后端**：当前只有 AES-256-GCM 文件后端，尚未接入 macOS Keychain /
   Windows Credential Manager。
6. **Profile 删除/重命名**：暂未提供，避免破坏已写入客户端的 URL 和 Key 归属。
7. **自动更新**：未启用 MyGo 更新包和签名更新通道。
8. **OAuth 取消**：浏览器登录没有取消按钮，只能等待 5 分钟超时或关闭对话框。
9. **备份恢复 UI**：目前通过 `mymcp client restore` 完成，客户端页面暂未提供按钮。
10. **Windows 签名**：安装包未签名，SmartScreen 会提示未知发布者。

## 发布清单

- [ ] `go test ./...`、`go vet ./...`、`go tool mygo vet`、`go tool mygo i18n check`
- [ ] `go tool mygo build`
- [ ] 更新 `mygo.json` 版本并提交
- [x] 添加 MIT `LICENSE`
- [ ] 打 `v0.3.2` 标签，触发 Release workflow
- [ ] 检查 GitHub Release 中的 DMG、Linux 包、Windows 安装包、CLI 压缩包和
      `SHA256SUMS.txt`
- [ ] 确认安装包中包含 `bin/mcp-pipe`，首次启动生成 `~/.mymcp/bin/mcp-pipe`
- [ ] 在干净机器上安装 DMG/EXE/DEB，完成一次首次启动和客户端连接
- [ ] 配置 macOS Developer ID 签名和公证（正式对普通用户发布时）
