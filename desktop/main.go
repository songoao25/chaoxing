package main

// main.go 是 Wails 桌面应用入口：窗口配置 + 启动自检分支。
//
// 窗口：标题「超星助手」，默认 1000×680，最小 900×600（与雨课堂刷课助手一致），
// macOS 使用隐藏式标题栏（左侧交通灯，内容延伸到标题栏），外观跟随系统；
// 默认开启原生半透明材质（侧边栏可透出桌面模糊），
// 设置 CX_OPAQUE_WINDOW=1 可切换为不透明窗口（排查渲染问题用）。
// 单实例锁避免双开，第二个实例启动时聚焦已有窗口。

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// 窗口尺寸与标题（selftest.go 也会引用）。
const (
	windowTitle     = "超星助手"
	windowWidth     = 1000
	windowHeight    = 680
	windowMinWidth  = 900
	windowMinHeight = 600

	// singleInstanceID 是单实例锁 id（避免双开；与雨课堂桌面端做法一致）。
	singleInstanceID = "chaoxing-desktop-manager"
)

// 不透明模式的窗口底色（DESIGN.md 的亮色 --bg）。
const opaqueWindowHex = "#F4F4F5"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if hasSelfTestFlag(os.Args[1:]) {
		os.Exit(runSelfTest(os.Stdout, os.Stderr))
	}

	app := NewApp()
	opaque := isOpaqueWindow()
	err := wails.Run(&options.App{
		Title:     windowTitle,
		Width:     windowWidth,
		Height:    windowHeight,
		MinWidth:  windowMinWidth,
		MinHeight: windowMinHeight,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				focusMainWindow(app)
			},
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: windowBackgroundColour(opaque),
		Mac:              windowMacOptions(opaque),
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败：", err)
		os.Exit(1)
	}
}

// isOpaqueWindow 判断是否启用逃生开关：CX_OPAQUE_WINDOW=1 时不透明。
func isOpaqueWindow() bool {
	v := strings.TrimSpace(os.Getenv("CX_OPAQUE_WINDOW"))
	return v == "1" || strings.EqualFold(v, "true")
}

// windowMacOptions 返回 macOS 窗口外观参数。
//
// 默认（半透明）：WebviewIsTransparent + WindowIsTranslucent，Wails 会在内容视图
// 下方加 NSVisualEffectView（behind-window 模糊），前端侧边栏用半透明色即可透出桌面。
// 不透明模式：两者关闭，背景用纯色 #F4F4F5，方便排查渲染问题。
// 两种模式都保留隐藏式标题栏、跟随系统外观，以及「关于」面板信息。
func windowMacOptions(opaque bool) *mac.Options {
	opts := &mac.Options{
		TitleBar:   mac.TitleBarHiddenInset(),
		Appearance: mac.DefaultAppearance, // 跟随系统
		About: &mac.AboutInfo{
			Title:   windowTitle,
			Message: aboutMessage(),
		},
	}
	if !opaque {
		opts.WebviewIsTransparent = true
		opts.WindowIsTranslucent = true
	}
	return opts
}

// aboutMessage 是 macOS「关于超星助手」面板的说明文案。
func aboutMessage() string {
	return fmt.Sprintf("%s %s\n协议版本 %d · 许可证 %s\n基于 Samueli924/chaoxing 的桌面图形界面",
		windowTitle, appVersion, appProtocol, appLicense)
}

// focusMainWindow 在第二个实例启动时聚焦已有窗口。
func focusMainWindow(app *App) {
	ctx := app.bridgeContext()
	if ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
}

// windowBackgroundColour 返回窗口底色。
// 半透明模式用完全透明避免启动白闪；不透明模式用 DESIGN.md 的亮色 --bg。
func windowBackgroundColour(opaque bool) *options.RGBA {
	if opaque {
		// #F4F4F5
		return &options.RGBA{R: 0xF4, G: 0xF4, B: 0xF5, A: 1}
	}
	return &options.RGBA{R: 0, G: 0, B: 0, A: 0}
}

// hasSelfTestFlag 判断是否带 --self-test 参数。
func hasSelfTestFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--self-test" || arg == "-self-test" {
			return true
		}
	}
	return false
}
