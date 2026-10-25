import Cocoa
import XCTest

@MainActor
class RunnerTests: XCTestCase {
  private func makeWindow() -> NSWindow {
    _ = NSApplication.shared
    let window = NSWindow(
      contentRect: NSRect(x: 100, y: 160, width: 800, height: 500),
      styleMask: [.titled, .closable, .miniaturizable, .resizable],
      backing: .buffered,
      defer: false
    )
    window.isReleasedWhenClosed = false
    return window
  }

  private func makeDefaults() -> UserDefaults {
    let name = "com.colin.tradingApp.panel-tests.\(UUID().uuidString)"
    let defaults = UserDefaults(suiteName: name)!
    addTeardownBlock { defaults.removePersistentDomain(forName: name) }
    return defaults
  }

  func testPanelUsesVisibleScreenIncludingNegativeDisplayOrigin() {
    let frame = RightPanelController.panelFrame(
      visibleFrame: NSRect(x: -1920, y: 120, width: 1440, height: 780),
      preferredWidth: 400
    )
    XCTAssertEqual(frame, NSRect(x: -880, y: 120, width: 400, height: 780))
  }

  func testPanelFitsDisplayNarrowerThanMinimumWidth() {
    let frame = RightPanelController.panelFrame(
      visibleFrame: NSRect(x: 20, y: 40, width: 250, height: 500),
      preferredWidth: 900
    )
    XCTAssertEqual(frame, NSRect(x: 20, y: 40, width: 250, height: 500))
  }

  func testMenuBarTogglePreservesWindowContentAndShowsAgainAfterHiding() {
    let window = makeWindow()
    let content = NSViewController()
    window.contentViewController = content
    var showCount = 0
    let controller = MenuBarController(window: window) {
      showCount += 1
    }
    controller.hideWindow()
    XCTAssertFalse(window.isVisible)
    controller.toggleWindow()
    XCTAssertTrue(window.isVisible)
    XCTAssertEqual(showCount, 1)
    controller.toggleWindow()
    XCTAssertFalse(window.isVisible)
    controller.showWindow()
    XCTAssertTrue(window.isVisible)
    XCTAssertEqual(showCount, 2)
    XCTAssertTrue(window.contentViewController === content)
    window.orderOut(nil)
  }

  func testMenuBarShowRestoresMinimizedWindow() {
    let window = makeWindow()
    let controller = MenuBarController(window: window) {}
    window.makeKeyAndOrderFront(nil)
    let minimized = expectation(forNotification: NSWindow.didMiniaturizeNotification, object: window)
    window.miniaturize(nil)
    wait(for: [minimized], timeout: 3)
    XCTAssertTrue(window.isMiniaturized)
    let restored = expectation(forNotification: NSWindow.didDeminiaturizeNotification, object: window)
    controller.toggleWindow()
    wait(for: [restored], timeout: 3)
    XCTAssertFalse(window.isMiniaturized)
    XCTAssertTrue(window.isVisible)
    window.orderOut(nil)
  }

  func testActualMenuBarButtonActionShowsAndHidesSameWindow() throws {
    let window = makeWindow()
    let content = NSViewController()
    window.contentViewController = content
    let controller = MenuBarController(window: window) {}
    controller.install()
    let item = try XCTUnwrap(controller.statusItem)
    defer {
      NSStatusBar.system.removeStatusItem(item)
      window.orderOut(nil)
    }
    let button = try XCTUnwrap(item.button)
    XCTAssertEqual(button.title, "盘")
    controller.hideWindow()
    button.performClick(nil)
    XCTAssertTrue(window.isVisible)
    button.performClick(nil)
    XCTAssertFalse(window.isVisible)
    XCTAssertTrue(window.contentViewController === content)
  }

  func testReturningToNormalRestoresWindowWithoutReplacingContent() {
    let window = makeWindow()
    let content = NSViewController()
    window.contentViewController = content
    window.title = "Existing screen"
    let originalFrame = window.frame
    let originalLevel = window.level
    let originalMovable = window.isMovable
    let controller = RightPanelController(window: window, defaults: makeDefaults())

    controller.setPanelMode(true)
    controller.setPanelMode(true)
    XCTAssertEqual(window.level, .floating)
    XCTAssertTrue(window.contentViewController === content)

    controller.setPanelMode(false)
    XCTAssertFalse(controller.isPanelMode)
    XCTAssertEqual(window.frame, originalFrame)
    XCTAssertEqual(window.level, originalLevel)
    XCTAssertEqual(window.isMovable, originalMovable)
    XCTAssertEqual(window.title, "Existing screen")
    XCTAssertTrue(window.contentViewController === content)
  }

  func testPanelWidthAndModeSurviveControllerRecreation() {
    let defaults = makeDefaults()
    let firstWindow = makeWindow()
    let first = RightPanelController(window: firstWindow, defaults: defaults)
    first.setPanelMode(true)
    first.setPanelWidth(580)

    let secondWindow = makeWindow()
    let second = RightPanelController(window: secondWindow, defaults: defaults)
    second.restorePreferredMode()
    XCTAssertTrue(second.isPanelMode)
    XCTAssertEqual(secondWindow.frame.width, 580, accuracy: 1)
  }

  func testFirstLaunchUsesPanelButExplicitNormalChoiceIsRemembered() {
    let defaults = makeDefaults()
    let window = makeWindow()
    let controller = RightPanelController(window: window, defaults: defaults)
    controller.restorePreferredMode()
    XCTAssertTrue(controller.isPanelMode)
    controller.setPanelMode(false)

    let nextWindow = makeWindow()
    let next = RightPanelController(window: nextWindow, defaults: defaults)
    next.restorePreferredMode()
    XCTAssertFalse(next.isPanelMode)
    XCTAssertEqual(nextWindow.level, .normal)
  }
}
