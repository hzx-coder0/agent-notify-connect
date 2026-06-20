# Codex Notifications

This plugin can also receive OpenAI Codex hook events.

## 1. Configure notifications

Codex and Claude Code share the same plugin config:

```text
~/.claude/claude-notifications-go/config.json
```

Run the normal settings command from Claude Code:

```text
/agent-notify-connect:settings
```

For Feishu/Lark QR binding, run:

```bash
claude-notifications feishu bind
```

Scan the printed URL with the Feishu/Lark mobile app. The command writes a `feishu_app` config that can send app robot messages to the scanned user's `open_id` by default.

For custom Feishu/Lark group bots, configure:

```json
{
  "notifications": {
    "webhook": {
      "enabled": true,
      "preset": "lark",
      "url": "https://open.feishu.cn/open-apis/bot/v2/hook/..."
    },
    "feishu": {
      "mode": "custom_webhook",
      "sign_secret_env": "CLAUDE_NOTIFICATIONS_FEISHU_SIGN_SECRET"
    }
  }
}
```

## 2. Generate Codex hook config

Windows:

```powershell
.\bin\claude-notifications-windows-amd64.exe codex-hooks
```

macOS/Linux:

```bash
./bin/claude-notifications codex-hooks
```

Write the output to:

```text
~/.codex/hooks.json
```

If `~/.codex/hooks.json` already exists, merge the generated `"Stop"`, `"PermissionRequest"`, and `"SubagentStop"` entries into the existing top-level `"hooks"` object.

Example:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "C:\\Tools\\claude-notifications-windows-amd64.exe handle-codex-hook Stop",
            "timeout": 30,
            "statusMessage": "Sending Codex completion notification"
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "C:\\Tools\\claude-notifications-windows-amd64.exe handle-codex-hook PermissionRequest",
            "timeout": 30,
            "statusMessage": "Sending Codex approval notification"
          }
        ]
      }
    ]
  }
}
```

## 3. Trust the hooks

After editing `hooks.json`, restart Codex and run:

```text
/hooks
```

Trust the command hooks shown by Codex.

## Events

Supported Codex events:

| Codex event | Notification |
| --- | --- |
| `Stop` | task completed |
| `PermissionRequest` | approval needed |
| `SubagentStop` | subagent completed |

`PreToolUse` is parsed but intentionally silent by default to avoid noisy notifications.

## Manual handler commands

The generated hooks call these commands:

```text
claude-notifications handle-codex-hook Stop
claude-notifications handle-codex-hook PermissionRequest
claude-notifications handle-codex-hook SubagentStop
```

Codex sends the hook JSON on stdin. The adapter maps Codex payloads into the shared notification service, so desktop notifications and all configured webhooks work the same way as Claude Code notifications.

## Manual test

```powershell
'{"session_id":"test","turn_id":"turn1","cwd":"E:\temp\repo","hook_event_name":"Stop","model":"gpt-5-codex","permission_mode":"never","last_assistant_message":"done"}' |
  .\bin\claude-notifications-windows-amd64.exe handle-codex-hook Stop
```
