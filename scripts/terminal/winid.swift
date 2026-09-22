// Prints "<window-number> <width> <height>" for each on-screen, layer-0
// Ghostty window, so screencapture -l can target it. Used by
// scripts/verify-terminal.sh.
import CoreGraphics
import Foundation

let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
let windows = CGWindowListCopyWindowInfo(options, kCGNullWindowID) as? [[String: Any]] ?? []
for window in windows {
  guard (window["kCGWindowOwnerName"] as? String) == "Ghostty",
        (window["kCGWindowLayer"] as? Int) == 0,
        let number = window["kCGWindowNumber"] as? Int,
        let bounds = window["kCGWindowBounds"] as? [String: Any],
        let width = bounds["Width"] as? Int,
        let height = bounds["Height"] as? Int
  else { continue }
  print(number, width, height)
}
