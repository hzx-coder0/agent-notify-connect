#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-hzx-coder0/claude-codex-notifications}"
APP_NAME="claude-codex-notifications"
INSTALL_ROOT="${INSTALL_ROOT:-${XDG_DATA_HOME:-$HOME/.local/share}/${APP_NAME}}"
VERSION=""
INSTALL_CLAUDE=true
INSTALL_CODEX=true
BIND_FEISHU=false
RUN_TEST=false
LOCAL_SOURCE=""

usage() {
  cat <<USAGE
Usage: install-linux.sh [options]

Options:
  --bind-feishu       Run Feishu/Lark QR binding after install
  --test              Send a Codex Stop test notification after install
  --no-claude         Do not write Claude Code hooks
  --no-codex          Do not write Codex hooks
  --version <tag>     Install a specific release tag, for example v1.0.0
  --install-root <p>  Install directory
  --local <dir>       Build and install from a local source checkout
  -h, --help          Show this help

Environment:
  REPO                GitHub repo, default: hzx-coder0/claude-codex-notifications
  INSTALL_ROOT        Install directory
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --bind-feishu)
      BIND_FEISHU=true
      ;;
    --test)
      RUN_TEST=true
      ;;
    --no-claude)
      INSTALL_CLAUDE=false
      ;;
    --no-codex)
      INSTALL_CODEX=false
      ;;
    --version)
      shift
      [ "$#" -gt 0 ] || { echo "--version requires a value" >&2; exit 2; }
      VERSION="$1"
      ;;
    --install-root)
      shift
      [ "$#" -gt 0 ] || { echo "--install-root requires a path" >&2; exit 2; }
      INSTALL_ROOT="$1"
      ;;
    --local)
      shift
      [ "$#" -gt 0 ] || { echo "--local requires a source directory" >&2; exit 2; }
      LOCAL_SOURCE="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  if [ "$os" != "linux" ]; then
    echo "This installer is Linux-only. Detected: $os" >&2
    exit 1
  fi

  case "$arch" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
      echo "Unsupported Linux architecture: $arch" >&2
      exit 1
      ;;
  esac
}

download() {
  local url="$1"
  local dest="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --connect-timeout 20 --max-time 120 "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$dest" "$url"
  else
    echo "curl or wget is required" >&2
    exit 1
  fi
}

release_download_base() {
  if [ -n "$VERSION" ]; then
    printf 'https://github.com/%s/releases/download/%s' "$REPO" "$VERSION"
  else
    printf 'https://github.com/%s/releases/latest/download' "$REPO"
  fi
}

raw_base() {
  local ref="main"
  if [ -n "$VERSION" ]; then
    ref="$VERSION"
  fi
  printf 'https://raw.githubusercontent.com/%s/%s' "$REPO" "$ref"
}

install_release_binary() {
  local asset="claude-notifications-linux-${ARCH}"
  local tmp
  tmp="$(mktemp)"

  echo "Downloading ${asset}..."
  download "$(release_download_base)/${asset}" "$tmp"
  install -m 0755 "$tmp" "${BIN_PATH}"
  rm -f "$tmp"
}

install_local_binary() {
  local source="$LOCAL_SOURCE"
  if [ -z "$source" ]; then
    source="$(pwd)"
  fi
  if [ ! -d "$source/cmd/claude-notifications" ]; then
    echo "Local source does not look like this repository: $source" >&2
    exit 1
  fi
  need_cmd go
  echo "Building from local source: $source"
  (cd "$source" && go build -trimpath -ldflags="-s -w" -o "$BIN_PATH" ./cmd/claude-notifications)
}

install_assets_from_release() {
  local base
  base="$(raw_base)"

  mkdir -p "$SOUNDS_DIR"
  for sound in task-complete.mp3 review-complete.mp3 question.mp3 plan-ready.mp3 error.mp3; do
    echo "Downloading sounds/${sound}..."
    download "${base}/sounds/${sound}" "${SOUNDS_DIR}/${sound}"
    chmod 0644 "${SOUNDS_DIR}/${sound}"
  done

  echo "Downloading claude_icon.png..."
  download "${base}/claude_icon.png" "${INSTALL_ROOT}/claude_icon.png"
  chmod 0644 "${INSTALL_ROOT}/claude_icon.png"
}

install_assets_from_local() {
  local source="$LOCAL_SOURCE"
  if [ -z "$source" ]; then
    source="$(pwd)"
  fi
  mkdir -p "$SOUNDS_DIR"
  cp "$source"/sounds/*.mp3 "$SOUNDS_DIR"/
  cp "$source/claude_icon.png" "$INSTALL_ROOT/claude_icon.png"
  chmod 0644 "$SOUNDS_DIR"/*.mp3 "$INSTALL_ROOT/claude_icon.png"
}

install_symlink() {
  local link_dir="$HOME/.local/bin"
  mkdir -p "$link_dir"
  ln -sfn "$BIN_PATH" "$link_dir/claude-notifications"
}

write_hooks() {
  if [ "$INSTALL_CLAUDE" != "true" ] && [ "$INSTALL_CODEX" != "true" ]; then
    echo "Hook writing skipped."
    return
  fi

  CLAUDE_PLUGIN_ROOT="$INSTALL_ROOT" "$BIN_PATH" install-hooks \
    --exe "$BIN_PATH" \
    "--claude=${INSTALL_CLAUDE}" \
    "--codex=${INSTALL_CODEX}"
}

bind_feishu() {
  CLAUDE_PLUGIN_ROOT="$INSTALL_ROOT" "$BIN_PATH" feishu bind
}

send_test() {
  local cwd payload
  cwd="$(pwd)"
  payload='{"session_id":"linux-installer-test","turn_id":"test","cwd":"'"$cwd"'","hook_event_name":"Stop","last_assistant_message":"Linux installer test: Codex and Feishu notifications are connected."}'
  printf '%s\n' "$payload" | CLAUDE_PLUGIN_ROOT="$INSTALL_ROOT" "$BIN_PATH" handle-codex-hook Stop
}

main() {
  detect_platform
  mkdir -p "$BIN_DIR" "$SOUNDS_DIR"

  BIN_PATH="${BIN_DIR}/claude-notifications"

  if [ -n "$LOCAL_SOURCE" ]; then
    install_local_binary
    install_assets_from_local
  else
    install_release_binary
    install_assets_from_release
  fi

  install_symlink
  write_hooks

  if [ "$BIND_FEISHU" = "true" ]; then
    bind_feishu
  fi

  if [ "$RUN_TEST" = "true" ]; then
    send_test
  fi

  cat <<DONE

Installed.
  Binary: ${BIN_PATH}
  Install root: ${INSTALL_ROOT}
  Config: ${HOME}/.claude/claude-notifications-go/config.json
  Claude hooks: ${HOME}/.claude/settings.json
  Codex hooks: ${HOME}/.codex/hooks.json

Restart Claude Code/Codex after installing hooks.
DONE
}

BIN_DIR="${INSTALL_ROOT}/bin"
SOUNDS_DIR="${INSTALL_ROOT}/sounds"
main
