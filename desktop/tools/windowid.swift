// windowid.swift —— 列出（或查找）某个应用的窗口 id 与几何信息
//
// 用法：
//   windowid                    列出所有可见窗口（owner / 标题 / id /  bounds）
//   windowid <owner 关键字>     只列匹配的窗口，输出：<windowid>\t<owner>\t<title>\t<w>x<h>@<x>,<y>
//
// 用途：配合 screencapture -l <windowid> 只截应用窗口，不截整个屏幕（隐私）。
// 只读取窗口元数据，不需要屏幕录制权限；截图那一步才可能触发系统授权。

import CoreGraphics
import Foundation

let target = CommandLine.arguments.count > 1 ? CommandLine.arguments[1].lowercased() : ""
let opts = CGWindowListOption(arrayLiteral: .optionOnScreenOnly, .excludeDesktopElements)
guard let raw = CGWindowListCopyWindowInfo(opts, kCGNullWindowID) as? [[String: Any]] else {
    FileHandle.standardError.write("windowid: 无法读取窗口列表\n".data(using: .utf8)!)
    exit(1)
}

for win in raw {
    let owner = win[kCGWindowOwnerName as String] as? String ?? ""
    let title = win[kCGWindowName as String] as? String ?? ""
    let number = win[kCGWindowNumber as String] as? Int ?? 0
    let bounds = win[kCGWindowBounds as String] as? [String: Any] ?? [:]
    let width = (bounds["Width"] as? NSNumber)?.intValue ?? 0
    let height = (bounds["Height"] as? NSNumber)?.intValue ?? 0
    let x = (bounds["X"] as? NSNumber)?.intValue ?? 0
    let y = (bounds["Y"] as? NSNumber)?.intValue ?? 0
    if target.isEmpty {
        print("\(number)\t\(owner)\t\(title)\t\(width)x\(height)@\(x),\(y)")
    } else if owner.lowercased().contains(target) || title.lowercased().contains(target) {
        print("\(number)\t\(owner)\t\(title)\t\(width)x\(height)@\(x),\(y)")
    }
}
