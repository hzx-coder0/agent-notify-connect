# v1.0.1

Patch release for `agent-notify-connect`.

## Fixed

- Terminal installer now shows the Feishu/Lark QR code directly in the same terminal.
- `claude-notifications feishu bind` now prints a terminal QR code by default.
- Feishu/Lark binding no longer asks the user to run a second command manually during terminal install.

## Changed

- Repository renamed to `agent-notify-connect`.
- Plugin metadata and install docs now use the new repository name.

## Install

Windows:

Download `agent-notify-connect-windows-amd64.zip`, unzip it, then run:

```text
bin/notification-installer-gui-windows-amd64.exe
```

Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/hzx-coder0/agent-notify-connect/main/scripts/install-linux.sh | bash
```

Linux with Feishu/Lark binding:

```bash
curl -fsSL https://raw.githubusercontent.com/hzx-coder0/agent-notify-connect/main/scripts/install-linux.sh | bash -s -- --bind-feishu
```
