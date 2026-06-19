// swift-tools-version:5.9

import PackageDescription

let package = Package(
    name: "terminal-notifier-modern",
    platforms: [
        .macOS(.v10_15)
    ],
    targets: [
        .executableTarget(
            name: "terminal-notifier-modern",
            path: "Sources/terminal-notifier-modern",
            linkerSettings: [
                .linkedFramework("UserNotifications"),
                .linkedFramework("AppKit"),
            ]
        ),
        .testTarget(
            name: "terminal-notifier-modernTests",
            dependencies: ["terminal-notifier-modern"],
            path: "Tests/terminal-notifier-modernTests"
        ),
    ]
)
