# Managed client icons

Icons are bundled so the app shows them offline.

## From Magpie

Most icons come from [yetone/magpie](https://github.com/yetone/magpie),
`internal/gui/assets/icons/`, renamed to the client's name here:

`cursor.svg`, `windsurf.svg`, `trae.svg`, `claude.svg` (Claude Desktop,
`claude-color.svg`), `claudecode.svg` (`claudecode-color.svg`),
`codex.svg` (`codex-color.svg`), `gemini.svg` (`geminicli-color.svg`),
`opencode.svg`, `copilot.svg` (`githubcopilot.svg`), `cline.svg`,
`kiro.svg` (`kiro-color.svg`), `qoder.svg`, `crush.png`, `vscode.svg`,
`lmstudio.svg`, `droid.svg` (`factory.svg`), and `cherrystudio.svg`
(`cherrystudio-color.svg`).

Magpie's MCP layer (`internal/library/mcp.go`, `targets.go`) also defines the
entry shapes used here: `fmtCursor`, `fmtClaude` (Claude Code), `fmtDesktop`
(Claude Desktop), `fmtCodex` (TOML), `fmtGemini`, `fmtOpenCode`,
`fmtCopilot`, `fmtCline`, `fmtKiro`, `fmtOmp` (Qoder) and `fmtCrush`. Claude
Desktop, Codex and Kiro take commands or HTTP only, so MyMCP bridges a remote
endpoint through `mcp-pipe`.

Magpie does not define Windsurf, Trae, VS Code, LM Studio, Factory Droid,
ChatWise, CherryStudio or DeepChat MCP files; those keep their vendor
formats.

## From the vendors

Magpie has no icon for ChatWise or DeepChat, so those are fetched from their
own sites and resized to 128×128 (2026-11-10):

| Client | Source |
|---|---|
| ChatWise | https://chatwise.app/_app/immutable/assets/icon.BlYxxTy5.png |
| DeepChat | https://deepchat.thinkinai.xyz/logo.png |

## Rendering notes

Most icons are drawn in their own colors with `ui.Image`. Icons that use only
`currentColor` follow the surrounding text color, as Cursor, Windsurf, Trae,
OpenCode, Cline, Qoder, VS Code, LM Studio, Factory Droid, Zed, Kimi Code and
Goose do.

MyGo's SVG renderer has no gradient, mask or filter support. The Codex, Gemini
CLI, CodeBuddy and Antigravity icons were therefore flattened to solid brand
colors; their original gradient versions were not changed upstream.

The icons and trademarks belong to their respective owners and identify the
clients they configure.
