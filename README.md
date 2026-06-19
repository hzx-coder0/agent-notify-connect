# Claude/Codex Notifications

Claude Code 和 Codex 的本地通知 + 飞书/Lark 通知工具。

这个项目基于 `777genius/claude-notifications-go` 改造，保留 GPL-3.0 许可。当前版本重点解决三件事：

- Claude Code hooks 通知。
- Codex hooks 通知。
- 飞书/Lark 扫码绑定，通知内容可发送模型回复正文。

## 功能

- Windows 图形化安装器。
- Linux 一行安装脚本。
- 自动写入 Claude Code 和 Codex hooks。
- 桌面通知、声音通知、飞书/Lark 通知可配置。
- 飞书/Lark 支持扫码绑定，不需要手填 webhook。
- Codex `Stop` / `SubagentStop` 可把 `last_assistant_message` 发到飞书。
- Claude Code `Stop` / `SubagentStop` 会读取 transcript，提取最后一次 assistant 回复发到飞书。

## Windows 安装

从 [Releases](https://github.com/hzx-coder0/claude-codex-notifications/releases/latest) 下载 Windows zip：

```text
claude-codex-notifications-windows-amd64.zip
```

解压后双击：

```text
bin/notification-installer-gui-windows-amd64.exe
```

安装器会把运行文件复制到：

```text
%LOCALAPPDATA%\ClaudeNotificationsGo
```

然后按界面选择：

- 哪些状态通知。
- 是否桌面通知。
- 是否声音通知。
- 是否连接飞书/Lark。
- 是否写入 Claude Code hooks。
- 是否写入 Codex hooks。

安装完成后，可以删除解压目录。运行时使用的是 `%LOCALAPPDATA%\ClaudeNotificationsGo` 里的文件。

## Linux 安装

默认同时安装 Claude Code 和 Codex hooks：

```bash
curl -fsSL https://raw.githubusercontent.com/hzx-coder0/claude-codex-notifications/main/scripts/install-linux.sh | bash
```

安装并立刻扫码绑定飞书/Lark：

```bash
curl -fsSL https://raw.githubusercontent.com/hzx-coder0/claude-codex-notifications/main/scripts/install-linux.sh | bash -s -- --bind-feishu
```

安装后发送一次测试通知：

```bash
curl -fsSL https://raw.githubusercontent.com/hzx-coder0/claude-codex-notifications/main/scripts/install-linux.sh | bash -s -- --bind-feishu --test
```

默认安装目录：

```text
${XDG_DATA_HOME:-$HOME/.local/share}/claude-codex-notifications
```

安装完成后，可以删除 clone 目录。hooks 写入的是安装目录里的绝对路径。

## 手动命令

扫码绑定飞书/Lark：

```bash
claude-notifications feishu bind
```

手动写入 hooks：

```bash
claude-notifications install-hooks --exe /path/to/claude-notifications --claude=true --codex=true
```

只写 Claude Code：

```bash
claude-notifications install-hooks --exe /path/to/claude-notifications --claude=true --codex=false
```

只写 Codex：

```bash
claude-notifications install-hooks --exe /path/to/claude-notifications --claude=false --codex=true
```

## 配置路径

通知配置：

```text
~/.claude/claude-notifications-go/config.json
```

Claude Code hooks：

```text
~/.claude/settings.json
```

Codex hooks：

```text
~/.codex/hooks.json
```

## 触发时机

Claude Code：

| Hook | 触发通知 |
| --- | --- |
| `PreToolUse` | `ExitPlanMode` 计划完成、`AskUserQuestion` 需要用户回答 |
| `Notification` | 权限请求 |
| `Stop` | 主会话停止后，根据 transcript 判断完成、审阅、错误、额度等状态 |
| `SubagentStop` | 子任务停止后通知，受配置控制 |
| `TeammateIdle` | team mode 下队友空闲通知 |

Codex：

| Hook | 触发通知 |
| --- | --- |
| `Stop` | 一轮回复结束后通知 |
| `SubagentStop` | 子任务结束后通知 |
| `PermissionRequest` | Codex 请求执行权限时通知 |

飞书/Lark 内容：

- Codex：优先发送 hook payload 里的 `last_assistant_message` 完整正文。
- Claude Code：从 `transcript_path` 指向的 JSONL transcript 中提取最后一次 assistant 回复。
- 桌面通知会做短文本展示，飞书/Lark webhook 使用完整正文。

## 本地构建

```bash
go test ./internal/codexhooks ./internal/webhook ./internal/hooks ./internal/feishu ./internal/installer ./internal/config
go build -o bin/claude-notifications ./cmd/claude-notifications
```

Windows release 构建需要在 Windows 上执行：

```powershell
go build -trimpath -ldflags="-s -w" -o bin\claude-notifications-windows-amd64.exe .\cmd\claude-notifications
go build -trimpath -ldflags="-s -w" -o bin\notification-installer-gui-windows-amd64.exe .\cmd\notification-installer-gui
```

## License

GPL-3.0。

Derived from `777genius/claude-notifications-go`.
