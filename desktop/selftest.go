package main

// selftest.go 实现 --self-test：不打开窗口、不启动 Wails GUI，
// 只跑引擎探测（+ 引擎就绪时起 Python 桥 ping/status），把结果 JSON 打到 stdout，
// 退出码 0（成功）/ 1（失败）。供无头验收使用。

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
)

// selfTestReport 是 --self-test 的输出结构。
type selfTestReport struct {
	OK       bool           `json:"ok"`
	Mode     string         `json:"mode"` // real | mock
	Engine   EngineStatus   `json:"engine"`
	Bridge   selfTestBridge `json:"bridge"`
	Window   selfTestWindow `json:"window"`
	Platform string         `json:"platform"`
	Update   selfTestUpdate `json:"update"`
	Error    string         `json:"error,omitempty"`
}

// selfTestWindow 输出窗口外壳参数，便于无头验收外观配置。
type selfTestWindow struct {
	Title              string `json:"title"`
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	MinWidth           int    `json:"min_width"`
	MinHeight          int    `json:"min_height"`
	TitleBar           string `json:"title_bar"`
	Appearance         string `json:"appearance"`
	Translucent        bool   `json:"window_translucent"`
	WebviewTransparent bool   `json:"webview_transparent"`
	OpaqueSwitch       bool   `json:"opaque_window"`
	Background         string `json:"background"`
	About              string `json:"about"`
}

// currentWindowReport 汇总当前窗口外观配置。
func currentWindowReport() selfTestWindow {
	opaque := isOpaqueWindow()
	report := selfTestWindow{
		Title:              windowTitle,
		Width:              windowWidth,
		Height:             windowHeight,
		MinWidth:           windowMinWidth,
		MinHeight:          windowMinHeight,
		TitleBar:           "hidden_inset",
		Appearance:         "system",
		Translucent:        !opaque,
		WebviewTransparent: !opaque,
		OpaqueSwitch:       opaque,
		Background:         "transparent",
		About:              aboutMessage(),
	}
	if opaque {
		report.Background = opaqueWindowHex
	}
	return report
}

// selfTestUpdate 输出自动更新的静态能力（不联网、不检查 Release）。
type selfTestUpdate struct {
	CurrentVersion    string `json:"current_version"`
	Supported         bool   `json:"supported"`
	AutoCheckEnabled  bool   `json:"auto_check_enabled"`
	UpdateSource      string `json:"update_source"`
	ExpectedAssetName string `json:"expected_asset_name"`
}

// currentUpdateReport 汇总更新能力。
func currentUpdateReport() selfTestUpdate {
	assetName, _ := updateAssetName(runtime.GOOS, runtime.GOARCH)
	return selfTestUpdate{
		CurrentVersion:    appVersion,
		Supported:         runtime.GOOS == "darwin",
		AutoCheckEnabled:  updateAutoCheckEnabled(),
		UpdateSource:      "github-releases:" + updateRepoSlug,
		ExpectedAssetName: assetName,
	}
}

type selfTestBridge struct {
	State      string          `json:"state"`
	Message    string          `json:"message,omitempty"`
	Ping       *PingInfo       `json:"ping,omitempty"`
	Status     json.RawMessage `json:"status,omitempty"`
	StderrTail []string        `json:"stderr_tail,omitempty"`
}

// runSelfTest 执行自检并返回退出码。
func runSelfTest(stdout, stderr io.Writer) int {
	report := selfTestReport{
		Bridge:   selfTestBridge{State: bridgeStateStopped},
		Window:   currentWindowReport(),
		Platform: runtime.GOOS,
		Update:   currentUpdateReport(),
	}

	if isMockMode() {
		report.OK = true
		report.Mode = "mock"
		report.Engine = mockEngineFinding().status()
		app := NewApp() // mock 模式下 NewApp 不碰真实引擎
		if raw, err := json.Marshal(app.mockStatus()); err == nil {
			report.Bridge.Status = raw
		}
		report.Bridge.State = bridgeStateReady
		report.Bridge.Message = "演示模式：使用内置假数据，不启动 Python"
		report.Bridge.Ping = &PingInfo{
			Protocol:      appProtocol,
			BridgeVersion: appVersion,
			Python:        "3.14.7",
			Repo:          "（演示模式）",
		}
		return writeSelfTestReport(stdout, stderr, report)
	}

	report.Mode = "real"
	finding := detectEngineDefault()
	report.Engine = finding.status()
	if !finding.Ready {
		report.Error = finding.Message
		return writeSelfTestReport(stdout, stderr, report)
	}

	bridge := NewBridge(finding.Repo, finding.PythonExe)
	defer bridge.Close()

	if err := bridge.Start(); err != nil {
		report.Bridge.State = bridgeStateFailed
		report.Bridge.Message = err.Error()
		report.Bridge.StderrTail = bridge.StderrTail(10)
		report.Error = err.Error()
		return writeSelfTestReport(stdout, stderr, report)
	}

	ping, err := bridge.Ping()
	if err != nil {
		state, message := bridge.State()
		report.Bridge.State = state
		report.Bridge.Message = message
		report.Bridge.StderrTail = bridge.StderrTail(20)
		report.Error = "Python 桥 ping 失败：" + err.Error()
		return writeSelfTestReport(stdout, stderr, report)
	}
	report.Bridge.Ping = &ping

	raw, err := bridge.Call("status", nil, bridgeDefaultTimeout)
	if err != nil {
		state, message := bridge.State()
		report.Bridge.State = state
		report.Bridge.Message = message
		report.Bridge.StderrTail = bridge.StderrTail(20)
		report.Error = "Python 桥 status 失败：" + err.Error()
		return writeSelfTestReport(stdout, stderr, report)
	}
	report.Bridge.Status = raw
	state, message := bridge.State()
	report.Bridge.State = state
	report.Bridge.Message = message
	report.OK = true
	return writeSelfTestReport(stdout, stderr, report)
}

// writeSelfTestReport 输出 JSON，返回退出码。
func writeSelfTestReport(stdout, stderr io.Writer, report selfTestReport) int {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "自检结果序列化失败：%v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	if !report.OK {
		fmt.Fprintf(stderr, "自检失败：%s\n", report.Error)
		return 1
	}
	return 0
}
