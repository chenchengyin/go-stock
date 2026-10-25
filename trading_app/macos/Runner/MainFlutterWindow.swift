import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow, NSWindowDelegate {
  private var rightPanelController: RightPanelController?

  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
    rightPanelController = RightPanelController(window: self)
    delegate = self
    isReleasedWhenClosed = false
  }

  func configureWindowPresentation() {
    rightPanelController?.installControls()
    rightPanelController?.restorePreferredMode()
  }

  func prepareMenuBarPanel() {
    rightPanelController?.setPanelMode(true)
  }

  func windowShouldClose(_ sender: NSWindow) -> Bool {
    orderOut(nil)
    return false
  }
}
