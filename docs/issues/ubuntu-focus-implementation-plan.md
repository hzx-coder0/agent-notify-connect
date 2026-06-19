# Click-to-Focus Feature on Linux - Research Summary

## Issue Context

A user requested click-to-focus functionality for Ubuntu 24.04 Wayland, similar to what works on macOS. This document summarizes the research findings and implementation challenges.

## Current macOS Implementation

On macOS, click-to-focus works via `terminal-notifier` with the `-activate bundleID` flag:
- Auto-detects terminal from `TERM_PROGRAM` environment variable
- Activates the terminal window when user clicks the notification
- Simple, reliable, works out of the box

## Why Linux (Especially Wayland) is Different

### 1. Focus-Stealing Prevention (Security Feature)

Wayland has **built-in focus-stealing prevention** as a core security feature. Unlike X11, applications cannot arbitrarily request focus - this prevents phishing attacks where a malicious app could steal password input.

> "The compositor library mutter implements focus stealing prevention mechanisms... In GNOME Shell, activation means that the window receives focus and is placed on top of other windows."
>
> — [GNOME Shell Developer Blog](https://blogs.gnome.org/shell-dev/2024/09/20/understanding-gnome-shells-focus-stealing-prevention/)

### 2. XDG Activation Protocol

Wayland introduced `xdg_activation_v1` protocol to allow "legitimate" window activation:

1. Application A (with focus) requests an **activation token** from compositor
2. Token is passed to Application B (via environment variable or D-Bus)
3. Application B uses token to request focus
4. Compositor decides whether to grant focus based on token validity

**The catch**: Tokens must be obtained during user interaction (like a click). A background process cannot generate valid tokens.

### 3. Platform Fragmentation

Different Wayland compositors have different window management tools:

| Compositor | Tool | Protocol |
|------------|------|----------|
| wlroots (Sway, LabWC, Wayfire) | `wlrctl` | wlr-foreign-toplevel-management |
| KDE Plasma | `kdotool` | KWin D-Bus scripting |
| GNOME Shell | extension required | No standard CLI tool |

### 4. Current Notification Library Limitation

The `beeep` library currently used does not support notification action callbacks - only sending notifications. Need to use a different library.

## Available Solutions

### Go Library for D-Bus Notifications with Actions

**[github.com/esiqveland/notify](https://pkg.go.dev/github.com/esiqveland/notify)** provides:

```go
notifier, _ := notify.New(conn,
    notify.WithOnAction(func(sig *notify.ActionInvokedSignal) {
        // Called when user clicks notification
        if sig.ActionKey == "default" {
            focusTerminalWindow()
        }
    }),
)
```

### Window Focus Tools by Platform

| Platform | Tool | Command Example |
|----------|------|-----------------|
| **X11** | wmctrl | `wmctrl -a "VS Code"` |
| **X11** | xdotool | `xdotool search --name "VS Code" windowactivate` |
| **Wayland Sway** | wlrctl | `wlrctl toplevel focus app_id:code` |
| **Wayland KDE** | kdotool | `kdotool search --class code windowactivate` |
| **Wayland GNOME** | extension | Requires [activate-window-by-title](https://github.com/lucaswerkmeister/activate-window-by-title) |

## Expected Success Rates

| Platform | Success Rate | Notes |
|----------|--------------|-------|
| Linux X11 | ~90% | wmctrl/xdotool are mature, stable tools |
| Wayland Sway/wlroots | ~85% | wlrctl uses foreign-toplevel protocol (no token needed) |
| Wayland KDE | ~75% | kdotool uses KWin D-Bus API |
| Wayland GNOME (native) | ~40-50% | Focus prevention is strict, may show "is ready" notification |
| Wayland GNOME (with config) | ~80% | User can provide custom focus command |

## Implementation Plan

### Phase 1: Core D-Bus Integration

1. Replace `beeep` with `esiqveland/notify` for Linux
2. Send notifications with `"default"` action for click handling
3. Listen for `ActionInvoked` signal

### Phase 2: Focus Tool Detection

```go
func detectAndFocus(windowIdentifier string) error {
    // 1. User override
    if config.FocusCommand != "" {
        return exec.Command("sh", "-c", config.FocusCommand).Run()
    }

    // 2. Auto-detect display server
    if os.Getenv("WAYLAND_DISPLAY") != "" {
        return focusWayland(windowIdentifier)
    }
    if os.Getenv("DISPLAY") != "" {
        return focusX11(windowIdentifier)
    }
    return errors.New("no display server detected")
}
```

### Phase 3: Configuration Option

Add `focusCommand` to config for users to specify their own command:

```yaml
notifications:
  desktop:
    clickToFocus: true
    focusCommand: "wmctrl -a 'Code'"  # User-specified override
```

## GNOME Wayland Specifics

### Why GNOME is Hardest

1. **No standard CLI tool** for window activation
2. **Strict focus-stealing prevention** - even with valid token, focus may be denied
3. **"Window is ready" notification** - instead of focus, user may see a second notification

### Workarounds for GNOME Users

1. **Install extension**: [activate-window-by-title](https://github.com/lucaswerkmeister/activate-window-by-title)
   ```bash
   # Then use focusCommand:
   focusCommand: "gdbus call --session --dest de.lucaswerkmeister.ActivateWindowByTitle --object-path /de/lucaswerkmeister/ActivateWindowByTitle --method de.lucaswerkmeister.ActivateWindowByTitle.activateBySubstring 'Code'"
   ```

2. **Install Steal My Focus**: [Extension link](https://extensions.gnome.org/extension/234/steal-my-focus/) - removes "is ready" notification

3. **Use custom script** with `focusCommand`

## Windows Support

Not planned in initial scope - requires WinRT Toast API with COM callbacks, which is significantly more complex.

## References

- [GNOME Shell Focus Stealing Prevention](https://blogs.gnome.org/shell-dev/2024/09/20/understanding-gnome-shells-focus-stealing-prevention/)
- [XDG Activation Protocol](https://wayland.app/protocols/xdg-activation-v1)
- [wlrctl](https://git.sr.ht/~brocellous/wlrctl)
- [kdotool](https://github.com/jinliu/kdotool)
- [activate-window-by-title](https://github.com/lucaswerkmeister/activate-window-by-title)
- [esiqveland/notify Go library](https://pkg.go.dev/github.com/esiqveland/notify)
- [Desktop Notifications Spec](https://specifications.freedesktop.org/notification-spec/latest/)
- [XDG Desktop Portal Notification](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Notification.html)

## Summary for GitHub Issue Response

**TL;DR**: Click-to-focus on Linux Wayland is challenging due to:

1. **Wayland's focus-stealing prevention** - a security feature by design
2. **Platform fragmentation** - different compositors need different tools
3. **GNOME being hardest** - no standard CLI tool for window activation

**What we can do**:
- X11: Full support via wmctrl/xdotool
- Wayland Sway/wlroots: Full support via wlrctl
- Wayland KDE: Good support via kdotool
- Wayland GNOME: Limited native support, but `focusCommand` config allows users to configure workarounds

**Recommended user workaround for Ubuntu 24.04 Wayland (GNOME)**:
1. Install [activate-window-by-title](https://github.com/lucaswerkmeister/activate-window-by-title) extension
2. Configure custom `focusCommand` in notifications config
