// webshot.swift —— 用 WKWebView 在无头模式给网页截图（与 Wails 同一渲染引擎）
//
// 用途：给桌面端前端做视觉验收。用法：
//   webshot <url> <out.png> [宽] [高] [--dark] [--wait=毫秒] [--scale=2]
// 例：
//   webshot http://127.0.0.1:8123/dev/ /tmp/ui.png 1000 720 --dark
//
// 说明：
// * 窗口不显示在屏幕上（移到屏幕外且透明），不会打扰用户；
// * 等页面 load 完成后再等 --wait 毫秒（默认 900）让字体/动画稳定；
// * 截图像素 = 逻辑尺寸 × backingScaleFactor（Retina 自动 2x）。

import AppKit
import Foundation
import WebKit

final class Shooter: NSObject, WKNavigationDelegate {
    var loaded = false
    var failure: Error?
    let done = DispatchSemaphore(value: 0)

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        loaded = true
    }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        failure = error
        done.signal()
    }
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        failure = error
        done.signal()
    }
}

func fail(_ message: String, code: Int32 = 2) -> Never {
    FileHandle.standardError.write(("webshot: " + message + "\n").data(using: .utf8)!)
    exit(code)
}

// ---- 参数 ----
var positional: [String] = []
var dark = false
var waitMs = 900
var scaleOverride: CGFloat?
for arg in CommandLine.arguments.dropFirst() {
    switch arg {
    case "--dark":
        dark = true
    case let s where s.hasPrefix("--wait="):
        waitMs = Int(s.dropFirst("--wait=".count)) ?? waitMs
    case let s where s.hasPrefix("--scale="):
        scaleOverride = CGFloat(Double(s.dropFirst("--scale=".count)) ?? 0)
    case let s where s.hasPrefix("--"):
        fail("未知参数 \(s)")
    default:
        positional.append(arg)
    }
}
guard positional.count >= 2 else {
    fail("用法：webshot <url> <out.png> [宽] [高] [--dark] [--wait=ms]")
}
guard let url = URL(string: positional[0]) else { fail("URL 不合法：\(positional[0])") }
let outPath = positional[1]
let width = CGFloat(Double(positional.count > 2 ? positional[2] : "1000") ?? 1000)
let height = CGFloat(Double(positional.count > 3 ? positional[3] : "720") ?? 720)

// ---- 应用与离屏窗口 ----
let app = NSApplication.shared
app.setActivationPolicy(.prohibited) // 不在 Dock/菜单栏出现

let rect = NSRect(x: 0, y: 0, width: width, height: height)
let window = NSWindow(contentRect: rect, styleMask: [.borderless], backing: .buffered, defer: false)
window.isReleasedWhenClosed = false
window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
window.alphaValue = 0
window.setFrameOrigin(NSPoint(x: -20000, y: -20000))
window.orderFrontRegardless()

let config = WKWebViewConfiguration()
if let scale = scaleOverride {
    config.setValue(1, forKey: "deprecatedBackingScale")
    _ = scale
}
let webView = WKWebView(frame: rect, configuration: config)
window.contentView = webView

let shooter = Shooter()
webView.navigationDelegate = shooter
webView.load(URLRequest(url: url))

// ---- 等加载 ----
let deadline = Date().addingTimeInterval(20)
while !shooter.loaded && shooter.failure == nil && Date() < deadline {
    RunLoop.main.run(until: Date().addingTimeInterval(0.05))
}
if let err = shooter.failure {
    fail("页面加载失败：\(err.localizedDescription)")
}
if !shooter.loaded {
    fail("页面加载超时（20s）：\(url)")
}
RunLoop.main.run(until: Date().addingTimeInterval(Double(waitMs) / 1000.0))

// ---- 截图 ----
var image: NSImage?
var snapshotError: Error?
let flags = DispatchSemaphore(value: 0)
webView.takeSnapshot(with: nil) { snap, err in
    image = snap
    snapshotError = err
    flags.signal()
}
while flags.wait(timeout: .now() + 0.05) == .timedOut {
    RunLoop.main.run(until: Date().addingTimeInterval(0.02))
}
if let err = snapshotError { fail("截图失败：\(err.localizedDescription)") }
guard let image = image else { fail("截图返回空图像") }

guard let tiff = image.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else {
    fail("PNG 编码失败")
}
do {
    try png.write(to: URL(fileURLWithPath: outPath))
} catch {
    fail("写入失败 \(outPath)：\(error.localizedDescription)")
}
print("\(outPath) \(bitmap.pixelsWide)x\(bitmap.pixelsHigh)")
