# Windows Installer

This branch includes a local Windows installer executable:

```powershell
.\bin\notification-installer-gui-windows-amd64.exe
```

There is also a console installer:

```powershell
.\bin\notification-installer-windows-amd64.exe
```

The GUI installer copies the runtime files into a stable user directory:

```text
%LOCALAPPDATA%\ClaudeNotificationsGo
```

Hooks point to:

```text
%LOCALAPPDATA%\ClaudeNotificationsGo\bin\claude-notifications-windows-amd64.exe
```

After installation, the cloned repository directory can be deleted without breaking notifications, as long as `%LOCALAPPDATA%\ClaudeNotificationsGo` remains in place.

The installer does not modify Claude Code, Codex, or repository source code. It writes user-level config files only:

```text
~/.claude/claude-notifications-go/config.json
~/.claude/settings.json
~/.codex/hooks.json
```

## What It Configures

- desktop notifications on/off
- sound on/off
- webhook/Feishu notifications on/off
- per-status notifications:
  - `task_complete`
  - `review_complete`
  - `question`
  - `plan_ready`
  - `session_limit_reached`
  - `api_error`
  - `api_error_overloaded`
- per-status desktop channel on/off
- per-status webhook channel on/off
- Claude Code hooks
- Codex hooks
- Feishu/Lark connection status
- optional webhook test notification

## Hook Installation

For non-interactive hook installation:

```powershell
.\bin\claude-notifications-windows-amd64.exe install-hooks --exe "C:\Tools\claude-notifications-windows-amd64.exe"
```

Claude Code hooks are merged into:

```text
~/.claude/settings.json
```

Codex hooks are merged into:

```text
~/.codex/hooks.json
```

Existing non-plugin hooks are preserved. Existing `claude-notifications` managed hooks are replaced with the new executable path.

## Feishu/Lark

The installer can check whether a `feishu_app` binding exists. To bind:

```powershell
.\bin\claude-notifications-windows-amd64.exe feishu bind
```

After scanning, the app credentials and `receive_id` are written to:

```text
~/.claude/claude-notifications-go/config.json
```

The installer can then send a test webhook notification. Failures are reported as real API or config errors.
