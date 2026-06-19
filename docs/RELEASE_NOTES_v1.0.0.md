# v1.0.0

Initial release of `claude-codex-notifications`.

## Included

- Windows GUI installer.
- Linux install script: `scripts/install-linux.sh`.
- Claude Code hook installation.
- Codex hook installation.
- Feishu/Lark QR binding.
- Feishu/Lark connection test.
- Full assistant reply sent to Feishu/Lark for Codex `Stop` and `SubagentStop`.
- Claude Code transcript parsing for final assistant reply.
- UTF-8 handling for Chinese notification text.

## Install

Windows:

Download `claude-codex-notifications-windows-amd64.zip`, unzip it, then run:

```text
bin/notification-installer-gui-windows-amd64.exe
```

Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/hzx-coder0/claude-codex-notifications/main/scripts/install-linux.sh | bash
```

Linux with Feishu/Lark binding:

```bash
curl -fsSL https://raw.githubusercontent.com/hzx-coder0/claude-codex-notifications/main/scripts/install-linux.sh | bash -s -- --bind-feishu
```

## Notes

This project is derived from `777genius/claude-notifications-go` and remains GPL-3.0 licensed.
