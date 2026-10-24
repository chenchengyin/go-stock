import AppKit
import Foundation

// Short-lived UI only: the Python runner owns scheduling and API access.
struct AlertInput: Decodable {
    let title: String
    let body: String
    let url: String
    let receipt: String
    let date: String
    let count: Int
    let browserProfile: String?
}

do {
    let input = try JSONDecoder().decode(AlertInput.self, from: FileHandle.standardInput.readDataToEndOfFile())
    guard let url = URL(string: input.url), url.scheme == "http" || url.scheme == "https",
          url.host == "127.0.0.1", url.port == 8080 else {
        throw NSError(domain: "RedAuction", code: 1, userInfo: [NSLocalizedDescriptionKey: "Invalid local red-list URL"])
    }
    let app = NSApplication.shared
    app.setActivationPolicy(.accessory)
    let alert = NSAlert()
    alert.messageText = input.title
    alert.informativeText = input.body
    alert.alertStyle = .informational
    alert.addButton(withTitle: "确定")
    alert.addButton(withTitle: "稍后")
    alert.window.level = .floating
    alert.window.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]

    var acknowledged = false
    func receipt(_ action: String) {
        let record: [String: Any] = ["date": input.date, "count": input.count, "status": action,
                                     "shown_at": ISO8601DateFormatter().string(from: Date())]
        do {
            let data = try JSONSerialization.data(withJSONObject: record)
            let file = URL(fileURLWithPath: input.receipt)
            try data.write(to: file, options: .atomic)
            try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: file.path)
        } catch {
            fputs("Cannot save notification receipt\n", stderr)
        }
    }
    // Runs inside the modal loop, after AppKit has actually shown the window.
    let observer = NotificationCenter.default.addObserver(
        forName: NSWindow.didBecomeKeyNotification, object: alert.window, queue: .main
    ) { _ in
        if alert.window.isVisible && !acknowledged {
            acknowledged = true
            receipt("shown")
        }
    }
    app.activate(ignoringOtherApps: true)
    let answer = alert.runModal()
    NotificationCenter.default.removeObserver(observer)
    if !acknowledged { receipt("shown") }
    if answer == .alertFirstButtonReturn {
        let chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
        if let profile = input.browserProfile,
           profile.range(of: "^[A-Za-z0-9 _-]+$", options: .regularExpression) != nil,
           FileManager.default.isExecutableFile(atPath: chrome) {
            // Open the normal profile, rather than an unrelated debug Chrome instance.
            let browser = Process()
            browser.executableURL = URL(fileURLWithPath: chrome)
            browser.arguments = ["--profile-directory=" + profile, url.absoluteString]
            browser.standardOutput = FileHandle.nullDevice
            browser.standardError = FileHandle.nullDevice
            do {
                try browser.run()
                receipt("open_requested")
            } catch {
                receipt("open_failed")
            }
        } else {
            receipt(NSWorkspace.shared.open(url) ? "open_requested" : "open_failed")
        }
    } else {
        receipt("dismissed")
    }
} catch {
    fputs("Cannot display red-auction dialog: \(error.localizedDescription)\n", stderr)
    exit(1)
}
