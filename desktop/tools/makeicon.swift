// makeicon.swift —— 生成桌面端应用图标（1024×1024 PNG）
//
// 用法：swiftc -O -o tools/bin/makeicon tools/makeicon.swift -framework AppKit
//       tools/bin/makeicon build/appicon.png [paper|ink] [SemiBold|Bold|Regular]
//
// 视觉方向：「文隐」——暖白纸底 + 思源宋体单字「超」，去掉线条、圆点、渐变与任何装饰。
// 字体优先级：思源宋体 SC → Noto Serif SC → 宋体-简；都没有时退回系统字体。
// 注意：图标里的字形是栅格化的，仓库只需要提交 PNG，不提交字体文件。

import AppKit
import Foundation

let args = CommandLine.arguments
let outPath = args.count > 1 ? args[1] : "appicon.png"
let variant = args.count > 2 ? args[2] : "paper"   // paper | ink
let weight = args.count > 3 ? args[3] : "SemiBold"
let size: CGFloat = 1024

func color(_ r: CGFloat, _ g: CGFloat, _ b: CGFloat, _ a: CGFloat = 1) -> NSColor {
    NSColor(srgbRed: r / 255, green: g / 255, blue: b / 255, alpha: a)
}

let paper = color(246, 241, 231)     // 温润纸色
let ink = color(27, 38, 48)          // 墨色（近黑，微蓝）
let hairline = color(27, 38, 48, 0.16)
let bg = variant == "ink" ? ink : paper
let glyph = variant == "ink" ? paper : ink

guard let rep = NSBitmapImageRep(
    bitmapDataPlanes: nil, pixelsWide: 1024, pixelsHigh: 1024,
    bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
    colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0
) else { exit(1) }
rep.size = NSSize(width: size, height: size)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

NSColor.clear.setFill()
NSRect(x: 0, y: 0, width: size, height: size).fill()

// macOS 图标网格：内容 824，四周 100
let content = NSRect(x: 100, y: 100, width: 824, height: 824)
let radius: CGFloat = 186
let tile = NSBezierPath(roundedRect: content, xRadius: radius, yRadius: radius)
bg.setFill()
tile.fill()

// 纸边：一条极细墨线，仅用于在浅色 Dock 上界定纸面
if variant == "paper" {
    let inner = NSBezierPath(roundedRect: content.insetBy(dx: 1.5, dy: 1.5),
                             xRadius: radius - 1.5, yRadius: radius - 1.5)
    inner.lineWidth = 3
    hairline.setStroke()
    inner.stroke()
}

// 思源宋体单字「超」
var font: NSFont?
let candidates = [
    "SourceHanSerifSC-" + weight,
    "SourceHanSerifSC-Regular",
    "NotoSerifSC-" + weight,
    "NotoSerifSC-Regular",
    "STSongti-SC-Bold",
]
for name in candidates {
    if let f = NSFont(name: name, size: 500) {
        font = f
        break
    }
}
let useFont = font ?? NSFont.systemFont(ofSize: 500, weight: .semibold)
let text = "超" as NSString
let attrs: [NSAttributedString.Key: Any] = [.font: useFont, .foregroundColor: glyph]
let textSize = text.size(withAttributes: attrs)
// 宋体字面重心略偏下，向上抬 8px 做视觉居中
text.draw(in: NSRect(x: (size - textSize.width) / 2,
                     y: (size - textSize.height) / 2 - 8,
                     width: textSize.width, height: textSize.height),
          withAttributes: attrs)

NSGraphicsContext.restoreGraphicsState()
guard let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
do {
    try png.write(to: URL(fileURLWithPath: outPath))
} catch {
    FileHandle.standardError.write("makeicon: 写入失败 \(error)\n".data(using: .utf8)!)
    exit(1)
}
print("\(outPath) \(rep.pixelsWide)x\(rep.pixelsHigh) font=\(useFont.fontName)")
