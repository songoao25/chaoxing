// makeicon.swift —— 生成桌面端应用图标（1024×1024 PNG）
//
// 用法：swiftc -O -o tools/bin/makeicon tools/makeicon.swift -framework AppKit
//       tools/bin/makeicon build/appicon.png
//
// 家族语言（与「雨课助手」同一套）：
//   暖白纸面圆角方块 + 1px 墨线内边 + 墨蓝单字 + 下方细墨线 + 小圆点。
//   不用渐变、不用发光、不用玻璃拟态。

import AppKit
import Foundation

let outPath = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "appicon.png"
let size: CGFloat = 1024
let canvas = NSSize(width: size, height: size)

func color(_ r: CGFloat, _ g: CGFloat, _ b: CGFloat, _ a: CGFloat = 1) -> NSColor {
    NSColor(srgbRed: r / 255, green: g / 255, blue: b / 255, alpha: a)
}

let paper = color(247, 244, 236)      // 暖白纸
let ink = color(31, 58, 95)           // 墨蓝
let hairline = color(31, 58, 95, 0.22)

guard let rep = NSBitmapImageRep(
    bitmapDataPlanes: nil, pixelsWide: 1024, pixelsHigh: 1024,
    bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
    colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0
) else { exit(1) }
rep.size = canvas
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

NSColor.clear.setFill()
NSRect(origin: .zero, size: canvas).fill()

// 纸面方块（macOS 图标网格：内容 824，四周 100）
let content = NSRect(x: 100, y: 100, width: 824, height: 824)
let radius: CGFloat = 186
let tile = NSBezierPath(roundedRect: content, xRadius: radius, yRadius: radius)
paper.setFill()
tile.fill()

// 1px 墨线内边（纸的边界，不是发光描边）
let inner = NSBezierPath(roundedRect: content.insetBy(dx: 1.5, dy: 1.5),
                         xRadius: radius - 1.5, yRadius: radius - 1.5)
inner.lineWidth = 3
hairline.setStroke()
inner.stroke()

// 墨蓝「超」
let fontSize: CGFloat = 470
let font = NSFont(name: "PingFang SC", size: fontSize)
    ?? NSFont.systemFont(ofSize: fontSize, weight: .semibold)
let text = "超" as NSString
let attrs: [NSAttributedString.Key: Any] = [.font: font, .foregroundColor: ink]
let textSize = text.size(withAttributes: attrs)
text.draw(in: NSRect(x: (size - textSize.width) / 2, y: 330,
                     width: textSize.width, height: textSize.height),
          withAttributes: attrs)

// 细墨线 + 圆点（家族签名）
let lineY: CGFloat = 292
let line = NSBezierPath()
line.move(to: NSPoint(x: 356, y: lineY))
line.line(to: NSPoint(x: 620, y: lineY))
line.lineWidth = 7
ink.setStroke()
line.stroke()

let dot = NSBezierPath(ovalIn: NSRect(x: 646, y: lineY - 6, width: 12, height: 12))
ink.setFill()
dot.fill()

NSGraphicsContext.restoreGraphicsState()

guard let png = rep.representation(using: .png, properties: [:]) else {
    FileHandle.standardError.write("makeicon: PNG 编码失败\n".data(using: .utf8)!)
    exit(1)
}
do {
    try png.write(to: URL(fileURLWithPath: outPath))
} catch {
    FileHandle.standardError.write("makeicon: 写入失败 \(error)\n".data(using: .utf8)!)
    exit(1)
}
print("\(outPath) \(rep.pixelsWide)x\(rep.pixelsHigh)")
