import Cocoa

@MainActor
final class MenuBarController: NSObject {
  private let window: NSWindow
  private let prepareToShow: () -> Void
  private(set) var statusItem: NSStatusItem?

  init(window: NSWindow, prepareToShow: @escaping () -> Void) {
    self.window = window
    self.prepareToShow = prepareToShow
    super.init()
  }

  func install() {
    guard statusItem == nil else { return }
    // Seed only this item's first position near the system icons. AppKit then
    // keeps the position chosen by the user when they Command-drag the item.
    let positionKey = "NSStatusItem Preferred Position TradingRadarMenuBar"
    if UserDefaults.standard.object(forKey: positionKey) == nil {
      UserDefaults.standard.set(0, forKey: positionKey)
    }
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    item.autosaveName = "TradingRadarMenuBar"
    if let button = item.button {
      button.title = "盘"
      button.font = .systemFont(ofSize: 14, weight: .semibold)
      button.toolTip = "交易雷达：点击展开／收起，右键退出"
      button.setAccessibilityLabel("交易雷达")
      button.target = self
      button.action = #selector(statusItemClicked(_:))
      button.sendAction(on: [.leftMouseUp, .rightMouseUp])
    }
    statusItem = item
    NSLog("TradingRadar menu-bar installed visible=%d", item.isVisible)
    DispatchQueue.main.asyncAfter(deadline: .now() + 1) { [weak self] in
      guard let button = self?.statusItem?.button else { return }
      NSLog("TradingRadar menu-bar layout frame=%@ windowVisible=%d screen=%@ safeRight=%@",
            NSStringFromRect(button.window?.frame ?? .zero), button.window?.isVisible ?? false,
            NSStringFromRect(button.window?.screen?.frame ?? .zero),
            String(describing: button.window?.screen?.auxiliaryTopRightArea))
    }
  }

  func toggleWindow() {
    if window.isVisible && !window.isMiniaturized { hideWindow() }
    else { showWindow() }
  }

  func showWindow() {
    if window.isMiniaturized { window.deminiaturize(nil) }
    prepareToShow()
    NSApp.unhide(nil)
    window.makeKeyAndOrderFront(nil)
    NSApp.activate(ignoringOtherApps: true)
    NSLog("TradingRadar menu-bar window=shown")
  }

  func hideWindow() {
    window.orderOut(nil)
    NSLog("TradingRadar menu-bar window=hidden")
  }

  @objc private func statusItemClicked(_ sender: NSStatusBarButton) {
    if let event = NSApp.currentEvent,
       event.type == .rightMouseUp || event.modifierFlags.contains(.control) {
      let menu = NSMenu()
      let show = NSMenuItem(title: "显示交易雷达", action: #selector(showFromMenu(_:)), keyEquivalent: "")
      show.target = self
      menu.addItem(show)
      menu.addItem(.separator())
      let quit = NSMenuItem(title: "退出交易雷达", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "")
      quit.target = NSApp
      menu.addItem(quit)
      NSMenu.popUpContextMenu(menu, with: event, for: sender)
    } else {
      toggleWindow()
    }
  }

  @objc private func showFromMenu(_ sender: Any?) { showWindow() }
}
