import Cocoa
import FlutterMacOS

@main
class AppDelegate: FlutterAppDelegate {
  private var menuBarController: MenuBarController?

  override func applicationDidFinishLaunching(_ notification: Notification) {
    // FlutterAppDelegate does not implement this optional delegate callback.
    guard let window = mainFlutterWindow as? MainFlutterWindow else { return }
    window.configureWindowPresentation()
    let controller = MenuBarController(window: window) { [weak window] in
      window?.prepareMenuBarPanel()
    }
    controller.install()
    controller.hideWindow()
    menuBarController = controller
  }

  override func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    return false
  }

  override func applicationShouldHandleReopen(_ sender: NSApplication,
                                             hasVisibleWindows flag: Bool) -> Bool {
    menuBarController?.showWindow()
    return false
  }

  override func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
    return true
  }
}
