import Foundation

protocol NotificationSending {
    func send(config: NotificationConfig, completion: @escaping (Result<Void, Error>) -> Void)
}
