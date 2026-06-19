import XCTest
@testable import terminal_notifier_modern

final class NotificationServiceTests: XCTestCase {

    func testNotificationConfigCreation() {
        let config = NotificationConfig(
            title: "Test Title",
            message: "Test Message",
            subtitle: "main · my-project",
            action: .activate(bundleID: "com.apple.Terminal"),
            group: "test-group",
            threadID: "session-123",
            timeSensitive: true,
            silent: false
        )

        XCTAssertEqual(config.title, "Test Title")
        XCTAssertEqual(config.message, "Test Message")
        XCTAssertEqual(config.subtitle, "main · my-project")
        XCTAssertEqual(config.action, .activate(bundleID: "com.apple.Terminal"))
        XCTAssertEqual(config.group, "test-group")
        XCTAssertEqual(config.threadID, "session-123")
        XCTAssertTrue(config.timeSensitive)
        XCTAssertFalse(config.silent)
    }

    func testNotificationConfigWithNilGroup() {
        let config = NotificationConfig(
            title: "Test",
            message: "Body",
            subtitle: nil,
            action: .none,
            group: nil,
            threadID: nil,
            timeSensitive: false,
            silent: true
        )

        XCTAssertNil(config.group)
        XCTAssertNil(config.subtitle)
        XCTAssertNil(config.threadID)
        XCTAssertEqual(config.action, .none)
        XCTAssertFalse(config.timeSensitive)
        XCTAssertTrue(config.silent)
    }

    func testExitCodes() {
        XCTAssertEqual(ExitCode.success, 0)
        XCTAssertEqual(ExitCode.invalidArgs, 1)
        XCTAssertEqual(ExitCode.permissionDenied, 2)
        XCTAssertEqual(ExitCode.failed, 3)
    }

    func testActionToJSONForUserInfo() {
        let action = ClickAction.activate(bundleID: "com.apple.Terminal")
        let json = action.toJSON()

        XCTAssertNotNil(json, "Action should produce valid JSON for userInfo storage")

        let decoded = ClickAction.fromJSON(json!)
        XCTAssertEqual(decoded, action, "Round-trip through JSON should preserve the action")
    }
}
