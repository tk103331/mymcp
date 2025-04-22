# mymcp

## 项目简介

`mymcp` 是一个基于 Go 语言开发的 MCP Servers 相关的一系列工具。

- `mcp-pipe`  MCP 管道，支持一对一管道，支持 sse2sse、sse2stdio、stdio2sse、stdio2stdio
- `mcp-router`  MCP 路由，支持多个 MCP Servers 的统一路由
- `mcp-debug`  MCP 调试，命令行界面调试 MCP Server
- `mcp-mananger`  桌面MCP管理应用，基于Wails构建

## 目录结构

```
.
├── Makefile                # 项目构建与常用命令
├── go.mod / go.sum         # Go Module 依赖管理
├── debug/                  # MCP Server 调试
│   └── main.go
├── manager/                # 桌面MCP管理应用，基于Wails构建
│   ├── app.go
│   ├── main.go
│   ├── bind/               # Go 与前端/数据绑定
│   ├── data/               # 数据模型与文件处理
│   ├── frontend/           # 前端源码（Vue3 + Vite）
│   └── manager/            # 管理端核心逻辑
├── pipe/                   # MCP 管道，支持 sse2sse、sse2stdio、stdio2sse、stdio2stdio
│   └── main.go
├── router/                 # MCP 路由，支持多个 MCP Servers 的统一路由
│   ├── main.go
│   └── router.json         # 路由配置文件
└── README.md               # 项目说明文档
```


## 贡献与开发

欢迎提交 Issue 和 PR，请确保遵循 Go 及 Vue3 相关最佳实践。
