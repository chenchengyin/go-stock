import Cocoa

@MainActor
final class RightPanelController: NSObject, NSToolbarDelegate, NSMenuItemValidation {
  private static let modeKey = "mac_right_panel_enabled"
  private static let widthKey = "mac_right_panel_width"
  private static let toggleItemID = NSToolbarItem.Identifier("TradingRadarWindowMode")

  private struct NormalWindowState {
    let frame: NSRect
    let level: NSWindow.Level
    let minSize: NSSize
    let maxSize: NSSize
    let movable: Bool
    let title: String
    let collectionBehavior: NSWindow.CollectionBehavior
  }

  private weak var window: NSWindow?
  private let defaults: UserDefaults
  private let visibleFrameProvider: () -> NSRect?
  private var normalState: NormalWindowState?
  private var modeMenuItem: NSMenuItem?
  private var modeButton: NSButton?
  private(set) var isPanelMode = false

  init(window: NSWindow, defaults: UserDefaults = .standard,
       visibleFrameProvider: (() -> NSRect?)? = nil) {
    self.window = window
    self.defaults = defaults
    self.visibleFrameProvider = visibleFrameProvider ?? { [weak window] in
      (window?.screen ?? NSScreen.main)?.visibleFrame
    }
    super.init()
    NotificationCenter.default.addObserver(
      self, selector: #selector(didEndLiveResize(_:)),
      name: NSWindow.didEndLiveResizeNotification, object: window
    )
    NotificationCenter.default.addObserver(
      self, selector: #selector(screenParametersDidChange(_:)),
      name: NSApplication.didChangeScreenParametersNotification, object: nil
    )
  }

  deinit {
    NotificationCenter.default.removeObserver(self)
  }

  @objc private func didEndLiveResize(_ notification: Notification) {
    guard isPanelMode, let window = window else { return }
    setPanelWidth(window.frame.width)
  }

  @objc private func screenParametersDidChange(_ notification: Notification) {
    if isPanelMode { positionPanel() }
  }

  static func panelFrame(visibleFrame: NSRect, preferredWidth: CGFloat) -> NSRect {
    let maximumWidth = min(640, max(0, visibleFrame.width))
    let minimumWidth = min(320, maximumWidth)
    let requestedWidth = preferredWidth.isFinite ? preferredWidth : 400
    let width = min(maximumWidth, max(minimumWidth, requestedWidth))
    return NSRect(x: visibleFrame.maxX - width, y: visibleFrame.minY,
                  width: width, height: max(0, visibleFrame.height))
  }

  func restorePreferredMode() {
    setPanelMode(defaults.object(forKey: Self.modeKey) == nil || defaults.bool(forKey: Self.modeKey))
  }

  func setPanelMode(_ enabled: Bool) {
    guard let window = window, !window.styleMask.contains(.fullScreen) else { return }
    if enabled {
      guard let visibleFrame = visibleFrameProvider(), visibleFrame.width > 0,
            visibleFrame.height > 0 else { return }
      if !isPanelMode {
        normalState = NormalWindowState(
          frame: window.frame, level: window.level, minSize: window.minSize,
          maxSize: window.maxSize, movable: window.isMovable, title: window.title,
          collectionBehavior: window.collectionBehavior
        )
      }
      isPanelMode = true
      window.level = .floating
      window.isMovable = false
      window.collectionBehavior = window.collectionBehavior
        .subtracting([.fullScreenPrimary, .fullScreenAuxiliary]).union(.fullScreenNone)
      window.title = "交易雷达 · 右侧面板"
      positionPanel()
    } else if let state = normalState {
      isPanelMode = false
      window.minSize = state.minSize
      window.maxSize = state.maxSize
      window.level = state.level
      window.isMovable = state.movable
      window.collectionBehavior = state.collectionBehavior
      window.title = state.title
      window.setFrame(state.frame, display: true)
      normalState = nil
    }
    defaults.set(enabled, forKey: Self.modeKey)
    modeMenuItem?.state = isPanelMode ? .on : .off
    modeButton?.title = isPanelMode ? "普通窗口" : "右侧面板"
    NSLog("TradingRadar window mode=%@ frame=%@", isPanelMode ? "right-panel" : "normal",
          NSStringFromRect(window.frame))
  }

  func setPanelWidth(_ width: CGFloat) {
    guard width.isFinite, let visibleFrame = visibleFrameProvider() else { return }
    let clampedWidth = Self.panelFrame(visibleFrame: visibleFrame, preferredWidth: width).width
    defaults.set(Double(clampedWidth), forKey: Self.widthKey)
    if isPanelMode { positionPanel() }
  }

  private func positionPanel() {
    guard let window = window, let visibleFrame = visibleFrameProvider() else { return }
    let preferredWidth = defaults.object(forKey: Self.widthKey) == nil
      ? 400 : defaults.double(forKey: Self.widthKey)
    let frame = Self.panelFrame(visibleFrame: visibleFrame, preferredWidth: preferredWidth)
    window.minSize = NSSize(width: min(320, frame.width), height: min(300, frame.height))
    window.maxSize = NSSize(width: min(640, visibleFrame.width), height: visibleFrame.height)
    window.setFrame(frame, display: true)
  }

  func installControls() {
    guard let window = window else { return }
    let toolbar = NSToolbar(identifier: "TradingRadarWindowToolbar")
    toolbar.delegate = self
    toolbar.allowsUserCustomization = false
    toolbar.displayMode = .iconOnly
    window.toolbarStyle = .unifiedCompact
    window.toolbar = toolbar

    guard let menu = NSApp.windowsMenu else { return }
    menu.addItem(.separator())
    let toggle = NSMenuItem(title: "右侧面板模式", action: #selector(togglePanelMode(_:)),
                            keyEquivalent: "p")
    toggle.keyEquivalentModifierMask = [.command, .control]
    toggle.target = self
    menu.addItem(toggle)
    modeMenuItem = toggle
    let widthItem = NSMenuItem(title: "面板宽度", action: nil, keyEquivalent: "")
    let widthMenu = NSMenu(title: "面板宽度")
    for width in [360, 400, 480, 580] {
      let item = NSMenuItem(title: "\(width) 点", action: #selector(selectPanelWidth(_:)),
                            keyEquivalent: "")
      item.tag = width
      item.target = self
      widthMenu.addItem(item)
    }
    widthItem.submenu = widthMenu
    menu.addItem(widthItem)
  }

  @objc private func togglePanelMode(_ sender: Any?) { setPanelMode(!isPanelMode) }
  @objc private func selectPanelWidth(_ sender: NSMenuItem) { setPanelWidth(CGFloat(sender.tag)) }

  func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
    if menuItem.action == #selector(selectPanelWidth(_:)) { return isPanelMode }
    return !(window?.styleMask.contains(.fullScreen) ?? true)
  }

  func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
    [.flexibleSpace, Self.toggleItemID]
  }

  func toolbarAllowedItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
    toolbarDefaultItemIdentifiers(toolbar)
  }

  func toolbar(_ toolbar: NSToolbar, itemForItemIdentifier itemIdentifier: NSToolbarItem.Identifier,
               willBeInsertedIntoToolbar flag: Bool) -> NSToolbarItem? {
    guard itemIdentifier == Self.toggleItemID else { return nil }
    let item = NSToolbarItem(itemIdentifier: itemIdentifier)
    let button = NSButton(title: isPanelMode ? "普通窗口" : "右侧面板", target: self,
                          action: #selector(togglePanelMode(_:)))
    button.bezelStyle = .rounded
    button.toolTip = "切换普通窗口与右侧面板（⌃⌘P）"
    item.view = button
    item.label = "窗口模式"
    modeButton = button
    return item
  }
}
