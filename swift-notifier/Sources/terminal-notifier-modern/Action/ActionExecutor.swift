import AppKit
import Foundation

protocol ActionExecuting {
    func execute(_ action: ClickAction)
}

final class ActionExecutor: ActionExecuting {

    func execute(_ action: ClickAction) {
        switch action {
        case .activate(let bundleID):
            activateApp(bundleID: bundleID)
        case .execute(let command):
            executeCommand(command)
        case .executeAndActivate(let command, let bundleID):
            executeCommand(command)
            activateApp(bundleID: bundleID)
        case .none:
            break
        }
    }

    private func activateApp(bundleID: String) {
        // Try NSRunningApplication.activate() first — it brings the app to
        // front without creating a new window (unlike NSWorkspace.openApplication
        // which causes Terminal.app to open a new window).
        let runningApps = NSRunningApplication.runningApplications(
            withBundleIdentifier: bundleID
        )
        if let app = runningApps.first {
            var activated = false
            if #available(macOS 14.0, *) {
                activated = app.activate(from: NSRunningApplication.current, options: [.activateIgnoringOtherApps])
            } else {
                activated = app.activate(options: [.activateIgnoringOtherApps])
            }
            if activated {
                return
            }
            // activate() failed (e.g. target is in fullscreen on another Space) —
            // fall through to openApplication which is stronger but may create a new window
        }

        guard let url = NSWorkspace.shared.urlForApplication(
            withBundleIdentifier: bundleID
        ) else {
            fputs("Warning: application not found for bundle ID: \(bundleID)\n", stderr)
            return
        }

        let configuration = NSWorkspace.OpenConfiguration()
        configuration.activates = true

        NSWorkspace.shared.openApplication(
            at: url,
            configuration: configuration
        ) { _, error in
            if let error = error {
                fputs("Warning: failed to activate application: \(error.localizedDescription)\n", stderr)
            }
        }
    }

    private func executeCommand(_ command: String) {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/sh")
        process.arguments = ["-c", command]

        do {
            try process.run()
            process.waitUntilExit()
        } catch {
            fputs("Warning: failed to execute command: \(error.localizedDescription)\n", stderr)
        }
    }
}
