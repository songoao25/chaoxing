package main

// dockicon.go：按系统外观切换 Dock / ⌘Tab 图标。
//
// macOS 15 的 .icns 不支持深浅色外观变体，所以 .app 里的 bundle 图标固定用亮色纸版；
// 运行时用 NSApplication.applicationIconImage 切换到纸/墨两版，并在系统外观变化后同步。
// 图标素材与图标生成器（tools/makeicon.swift）保持同一「文隐」规格。

import (
	_ "embed"
	"time"
)

//go:embed resources/icon-light.png
var dockIconLight []byte

//go:embed resources/icon-dark.png
var dockIconDark []byte

// startDockIconSync 立即按当前外观设置一次 Dock 图标，然后每 3 秒检查外观变化。
func startDockIconSync() {
	apply := func(dark bool) {
		if dark {
			setDockIconPNG(dockIconDark)
		} else {
			setDockIconPNG(dockIconLight)
		}
	}
	dark := systemIsDarkMode()
	apply(dark)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			current := systemIsDarkMode()
			if current != dark {
				dark = current
				apply(dark)
			}
		}
	}()
}
