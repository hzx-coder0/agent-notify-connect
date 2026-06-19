import Foundation
import UserNotifications

enum PermissionError: Error, CustomStringConvertible {
    case denied
    case notDetermined
    case unknown(UNAuthorizationStatus)

    var description: String {
        switch self {
        case .denied:
            return "Notification permission denied. Enable in System Settings > Notifications."
        case .notDetermined:
            return "Notification permission not yet determined."
        case .unknown(let status):
            return "Unknown notification authorization status: \(status.rawValue)"
        }
    }
}

enum PermissionManager {

    static func ensurePermission(completion: @escaping (Result<Void, Error>) -> Void) {
        let center = UNUserNotificationCenter.current()

        center.getNotificationSettings { settings in
            switch settings.authorizationStatus {
            case .authorized, .provisional:
                completion(.success(()))

            case .notDetermined:
                center.requestAuthorization(options: [.alert, .sound, .badge]) { granted, error in
                    if let error = error {
                        completion(.failure(error))
                    } else if granted {
                        completion(.success(()))
                    } else {
                        completion(.failure(PermissionError.denied))
                    }
                }

            case .denied:
                completion(.failure(PermissionError.denied))

            @unknown default:
                completion(.failure(PermissionError.unknown(settings.authorizationStatus)))
            }
        }
    }
}
