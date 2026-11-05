package main

// version.go 是桌面端版本号的唯一来源。
//
// 发版流程（GitHub Releases，macOS）：
//  1. 改这里的 appVersion 与 wails.json 的 info.productVersion（保持一致）；
//  2. 本地构建自测后提交；
//  3. 打 tag desktop-v<版本号> 并推送，CI 会自动构建 zip 并发布 Release；
//  4. 已安装的 App 启动时检查更新，用户点「下载并安装」即可升级。
//
// 用 var 而不是 const，方便 CI 通过 -ldflags "-X main.appVersion=..." 覆盖。
var appVersion = "0.3.0"
