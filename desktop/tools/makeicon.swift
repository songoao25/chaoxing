import AppKit
import Foundation
let outPath = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "appicon.png"
let size: CGFloat = 1024
func color(_ r: CGFloat, _ g: CGFloat, _ b: CGFloat, _ a: CGFloat = 1) -> NSColor {
    NSColor(srgbRed: r/255, green: g/255, blue: b/255, alpha: a)
}
// 与「雨课堂刷课助手」图标同构：系统蓝 squircle + 白色主图形 + 右下角绿色对勾徽标
let blue = color(10, 132, 255)
let green = color(52, 199, 89)
guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 1024, pixelsHigh: 1024,
    bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
    colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { exit(1) }
rep.size = NSSize(width: size, height: size)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
NSColor.clear.setFill(); NSRect(x: 0, y: 0, width: size, height: size).fill()
let content = NSRect(x: 100, y: 100, width: 824, height: 824)
let radius: CGFloat = 186
let tile = NSBezierPath(roundedRect: content, xRadius: radius, yRadius: radius)
blue.setFill(); tile.fill()
// 主图形：白色「超」
let font = NSFont(name: "PingFang SC", size: 460) ?? NSFont.systemFont(ofSize: 460, weight: .semibold)
let text = "超" as NSString
let attrs: [NSAttributedString.Key: Any] = [.font: font, .foregroundColor: NSColor.white]
let ts = text.size(withAttributes: attrs)
text.draw(in: NSRect(x: (size - ts.width)/2, y: 330, width: ts.width, height: ts.height), withAttributes: attrs)
// 右下角绿色对勾徽标（与 ykt 图标同一语言）
let badgeR: CGFloat = 122
let badgeRect = NSRect(x: 612, y: 190, width: badgeR*2, height: badgeR*2)
green.setFill(); NSBezierPath(ovalIn: badgeRect).fill()
let check = NSBezierPath()
check.move(to: NSPoint(x: 668, y: 300))
check.line(to: NSPoint(x: 720, y: 248))
check.line(to: NSPoint(x: 800, y: 336))
check.lineWidth = 26
check.lineCapStyle = .round
check.lineJoinStyle = .round
NSColor.white.setStroke(); check.stroke()
NSGraphicsContext.restoreGraphicsState()
guard let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try! png.write(to: URL(fileURLWithPath: outPath))
print("\(outPath) \(rep.pixelsWide)x\(rep.pixelsHigh)")
