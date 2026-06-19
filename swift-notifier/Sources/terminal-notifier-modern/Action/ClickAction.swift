import Foundation

enum ClickAction: Codable, Equatable {
    case activate(bundleID: String)
    case execute(command: String)
    case executeAndActivate(command: String, bundleID: String)
    case none

    private enum CodingKeys: String, CodingKey {
        case type
        case value
        case bundleID
    }

    private enum ActionType: String, Codable {
        case activate
        case execute
        case executeAndActivate
        case none
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        switch self {
        case .activate(let bundleID):
            try container.encode(ActionType.activate, forKey: .type)
            try container.encode(bundleID, forKey: .value)
        case .execute(let command):
            try container.encode(ActionType.execute, forKey: .type)
            try container.encode(command, forKey: .value)
        case .executeAndActivate(let command, let bundleID):
            try container.encode(ActionType.executeAndActivate, forKey: .type)
            try container.encode(command, forKey: .value)
            try container.encode(bundleID, forKey: .bundleID)
        case .none:
            try container.encode(ActionType.none, forKey: .type)
        }
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let type = try container.decode(ActionType.self, forKey: .type)
        switch type {
        case .activate:
            let bundleID = try container.decode(String.self, forKey: .value)
            self = .activate(bundleID: bundleID)
        case .execute:
            let command = try container.decode(String.self, forKey: .value)
            self = .execute(command: command)
        case .executeAndActivate:
            let command = try container.decode(String.self, forKey: .value)
            let bundleID = try container.decode(String.self, forKey: .bundleID)
            self = .executeAndActivate(command: command, bundleID: bundleID)
        case .none:
            self = .none
        }
    }

    func toJSON() -> String? {
        guard let data = try? JSONEncoder().encode(self),
              let json = String(data: data, encoding: .utf8) else {
            return nil
        }
        return json
    }

    static func fromJSON(_ json: String) -> ClickAction? {
        guard let data = json.data(using: .utf8),
              let action = try? JSONDecoder().decode(ClickAction.self, from: data) else {
            return nil
        }
        return action
    }
}
