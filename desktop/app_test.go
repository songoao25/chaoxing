package main

// app_test.go 覆盖 App 层的 P2 修复：
//   - 运行中桥崩溃时补发 run:finished 终态；
//   - StartRun 用 settings 补齐缺省参数；
//   - GetEngineStatus 未就绪时每次重新探测。

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordedEvent 是 eventEmitter 捕获到的一条事件。
type recordedEvent struct {
	name    string
	payload map[string]any
}

// eventRecorder 捕获 App.emit 的事件（替代 Wails runtime）。
type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) emit(name string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		decoded = map[string]any{}
	}
	r.mu.Lock()
	r.events = append(r.events, recordedEvent{name: name, payload: decoded})
	r.mu.Unlock()
}

func (r *eventRecorder) find(name string) []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recordedEvent
	for _, ev := range r.events {
		if ev.name == name {
			out = append(out, ev)
		}
	}
	return out
}

func (r *eventRecorder) count(name string) int { return len(r.find(name)) }

// newFakeBackendApp 构造一个引擎就绪、桥用假实现的 App。
func newFakeBackendApp(t *testing.T, fake *Bridge) (*App, *eventRecorder) {
	t.Helper()
	rec := &eventRecorder{}
	app := &App{
		engine: engineFinding{
			Repo:      t.TempDir(),
			PythonExe: "unused",
			Python:    "3.14.7",
			Ready:     true,
			Message:   "引擎就绪（测试）",
		},
		engineDetected: true,
		eventEmitter:   rec.emit,
	}
	app.bridgeFactory = func(string, string) *Bridge { return fake }
	t.Cleanup(func() {
		app.mu.Lock()
		b := app.bridge
		app.bridge = nil
		app.mu.Unlock()
		if b != nil {
			b.Close()
		}
	})
	return app, rec
}

func TestRunCrashFinishesActiveRun(t *testing.T) {
	fake := newTestBridge(t, "crash-after-run-start")
	app, rec := newFakeBackendApp(t, fake)

	res, err := app.StartRun(RunOptions{
		AccountID: "13800000001",
		CourseIDs: []string{"2151141"},
		Scope:     RunScope{Chapters: true, TaskCenter: true},
	})
	if err != nil {
		t.Fatalf("StartRun 失败：%v", err)
	}
	if res.RunID != "r-crash" {
		t.Errorf("run_id = %q，期望 r-crash", res.RunID)
	}

	if !waitFor(t, 5*time.Second, func() bool { return rec.count("run:finished") == 1 }) {
		t.Fatalf("桥在运行中崩溃后应补发 run:finished，实际事件：%v", rec.find("run:finished"))
	}
	ev := rec.find("run:finished")[0]
	if ok, _ := ev.payload["ok"].(bool); ok {
		t.Errorf("崩溃终态 ok 应为 false，实际：%v", ev.payload)
	}
	if reason, _ := ev.payload["reason"].(string); reason != "error" {
		t.Errorf("崩溃终态 reason = %q，期望 error", reason)
	}
	if message, _ := ev.payload["message"].(string); message != "Python 桥异常退出，本次任务已中断" {
		t.Errorf("崩溃终态 message = %q", message)
	}
	if got, _ := ev.payload["run_id"].(string); got != "r-crash" {
		t.Errorf("崩溃终态 run_id = %q，期望 r-crash", got)
	}
	if len(rec.find("bridge:state")) == 0 {
		t.Error("崩溃后应收到 bridge:state 事件")
	}
	// 桥应自动重启（假桥新进程只等待请求，所以状态停在 restarting）。
	if !waitFor(t, 3*time.Second, func() bool {
		state, _ := fake.State()
		return state == bridgeStateRestarting || state == bridgeStateStarting || state == bridgeStateReady
	}) {
		t.Errorf("桥崩溃后应尝试自动重启，实际状态：%q", mustState(fake))
	}

	app.mu.Lock()
	active := app.runActive
	app.mu.Unlock()
	if active {
		t.Error("补发终态后 App 的 runActive 应被复位")
	}

	// 不应重复补发终态。
	time.Sleep(200 * time.Millisecond)
	if n := rec.count("run:finished"); n != 1 {
		t.Errorf("run:finished 发送次数 = %d，期望 1", n)
	}
}

func mustState(b *Bridge) string {
	state, _ := b.State()
	return state
}

func TestStartRunFillsSettingsDefaults(t *testing.T) {
	markerDir := t.TempDir()
	fake := newTestBridge(t, "record-run-params", "CX_TEST_MARKER_DIR="+markerDir)
	app, _ := newFakeBackendApp(t, fake)

	paramsPath := filepath.Join(markerDir, "run-params.json")
	readParams := func() (map[string]any, bool) {
		data, err := os.ReadFile(paramsPath)
		if err != nil {
			return nil, false
		}
		var params struct {
			Options map[string]any `json:"options"`
			Limits  map[string]any `json:"limits"`
			Scope   map[string]any `json:"scope"`
		}
		if err := json.Unmarshal(data, &params); err != nil {
			return nil, false
		}
		if params.Options == nil {
			return nil, false
		}
		return params.Options, true
	}

	// 1) options 完全缺省：整组使用 settings（假桥返回 speed=1.25/jobs=4/...）。
	if _, err := app.StartRun(RunOptions{
		AccountID: "13800000001",
		CourseIDs: []string{"2151141"},
		Scope:     RunScope{Chapters: true, TaskCenter: true},
	}); err != nil {
		t.Fatalf("StartRun 失败：%v", err)
	}
	var full map[string]any
	if !waitFor(t, 3*time.Second, func() bool {
		options, ok := readParams()
		if !ok {
			return false
		}
		full = options
		return true
	}) {
		t.Fatal("假桥未收到 run.start 参数")
	}
	expect := map[string]any{
		"speed":              1.25,
		"jobs":               4.0,
		"notopen_action":     "retry",
		"retry_interval":     2.0,
		"serial_video":       true,
		"add_learning_count": true,
		"target_count":       42.0,
		"work_redo_enabled":  true,
		"work_max_retries":   5.0,
	}
	for key, want := range expect {
		if got := full[key]; got != want {
			t.Errorf("缺省 options.%s = %v，期望 %v", key, got, want)
		}
	}

	// 2) 部分字段缺省：只补数值/字符串零值，bool 保持前端传值（false）。
	_ = os.Remove(paramsPath)
	if _, err := app.StartRun(RunOptions{
		AccountID: "13800000001",
		CourseIDs: []string{"189191"},
		Scope:     RunScope{Chapters: true},
		Options:   RunSettings{Speed: 1.5, Jobs: 3},
	}); err != nil {
		t.Fatalf("第二次 StartRun 失败：%v", err)
	}
	var partial map[string]any
	if !waitFor(t, 3*time.Second, func() bool {
		options, ok := readParams()
		if !ok {
			return false
		}
		partial = options
		return partial["speed"] == 1.5
	}) {
		t.Fatal("假桥未收到第二次 run.start 参数")
	}
	if partial["speed"] != 1.5 || partial["jobs"] != 3.0 {
		t.Errorf("前端显式传的字段不应被覆盖：%v", partial)
	}
	if partial["notopen_action"] != "retry" || partial["retry_interval"] != 2.0 ||
		partial["target_count"] != 42.0 || partial["work_max_retries"] != 5.0 {
		t.Errorf("缺省的数值/字符串字段应从 settings 补齐：%v", partial)
	}
	if partial["serial_video"] != false {
		t.Errorf("bool 字段无法区分未传与 false，应保持前端传值 false：%v", partial["serial_video"])
	}
}

func TestGetEngineStatusReprobesWhenNotReady(t *testing.T) {
	app := &App{}
	calls := 0
	app.engineProber = func() engineFinding {
		calls++
		if calls == 1 {
			return engineFinding{Message: "未找到引擎目录：请选择引擎目录。"}
		}
		return engineFinding{
			Repo:      "/tmp/fake-engine",
			PythonExe: "/tmp/fake-engine/.venv/bin/python3",
			Python:    "3.14.7",
			Ready:     true,
			Message:   "引擎就绪（Python 3.14.7）",
		}
	}

	first := app.GetEngineStatus()
	if first.Ready {
		t.Fatalf("首次探测应未就绪：%+v", first)
	}
	if !containsChinese(first.Message) {
		t.Errorf("未就绪信息应为中文：%q", first.Message)
	}

	second := app.GetEngineStatus()
	if !second.Ready || second.Repo != "/tmp/fake-engine" {
		t.Fatalf("环境变好后重新检测应就绪：%+v", second)
	}
	if calls != 2 {
		t.Errorf("探测次数 = %d，期望 2（未就绪时每次重新探测）", calls)
	}

	third := app.GetEngineStatus()
	if !third.Ready {
		t.Fatalf("第三次应返回缓存的就绪状态：%+v", third)
	}
	if calls != 2 {
		t.Errorf("已就绪后应使用缓存，探测次数 = %d，期望仍为 2", calls)
	}
}

// containsChinese 粗略判断字符串是否含中文字符（错误文案要求）。
func containsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func TestGetAppInfoIncludesPlatform(t *testing.T) {
	info := (&App{}).GetAppInfo()
	if info.Version != appVersion {
		t.Errorf("version = %q，期望 %q", info.Version, appVersion)
	}
	if info.Protocol != appProtocol {
		t.Errorf("protocol = %d，期望 %d", info.Protocol, appProtocol)
	}
	if info.License != "GPL-3.0" {
		t.Errorf("license = %q，期望 GPL-3.0", info.License)
	}
	if info.Platform != runtime.GOOS {
		t.Errorf("platform = %q，期望 %q", info.Platform, runtime.GOOS)
	}
}

func TestWindowShellConfig(t *testing.T) {
	report := currentWindowReport()
	if report.Title != "超星助手" {
		t.Errorf("title = %q", report.Title)
	}
	if report.Width != 1000 || report.Height != 680 {
		t.Errorf("默认尺寸 = %dx%d，期望 1000x680", report.Width, report.Height)
	}
	if report.MinWidth != 900 || report.MinHeight != 600 {
		t.Errorf("最小尺寸 = %dx%d，期望 900x600", report.MinWidth, report.MinHeight)
	}
	if report.TitleBar != "hidden_inset" || report.Appearance != "system" {
		t.Errorf("标题栏/外观配置异常：%+v", report)
	}
	if !report.Translucent || !report.WebviewTransparent {
		t.Errorf("默认应开启半透明材质：%+v", report)
	}
	if !strings.Contains(report.About, appVersion) || !strings.Contains(report.About, "GPL-3.0") {
		t.Errorf("关于面板文案应含版本与 GPL-3.0：%q", report.About)
	}

	macDefault := windowMacOptions(false)
	if macDefault.About == nil || macDefault.About.Title != "超星助手" {
		t.Errorf("mac.AboutInfo 未配置：%+v", macDefault.About)
	}
	if !macDefault.WebviewIsTransparent || !macDefault.WindowIsTranslucent {
		t.Error("默认 mac.Options 应开启透明材质")
	}

	// 逃生开关：CX_OPAQUE_WINDOW=1 时关闭透明并改用 #F4F4F5。
	t.Setenv("CX_OPAQUE_WINDOW", "1")
	opaqueReport := currentWindowReport()
	if !opaqueReport.OpaqueSwitch || opaqueReport.Translucent || opaqueReport.WebviewTransparent {
		t.Errorf("CX_OPAQUE_WINDOW=1 应关闭透明材质：%+v", opaqueReport)
	}
	if opaqueReport.Background != opaqueWindowHex {
		t.Errorf("不透明模式背景 = %q，期望 %q", opaqueReport.Background, opaqueWindowHex)
	}
	macOpaque := windowMacOptions(true)
	if macOpaque.WebviewIsTransparent || macOpaque.WindowIsTranslucent {
		t.Error("不透明模式不应开启透明材质")
	}
	if macOpaque.About == nil {
		t.Error("不透明模式也应保留 AboutInfo")
	}
}

func TestSelfTestMockReport(t *testing.T) {
	t.Setenv("CX_DESKTOP_MOCK", "1")
	t.Setenv("CX_DATA_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := runSelfTest(&stdout, &stderr)
	if code != 0 {
		t.Fatalf("mock 自检应 exit 0，实际 %d（stderr：%s）", code, stderr.String())
	}
	var report selfTestReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("自检输出不是合法 JSON：%v", err)
	}
	if !report.OK || report.Mode != "mock" {
		t.Errorf("自检结果异常：ok=%v mode=%q", report.OK, report.Mode)
	}
	if report.Platform != runtime.GOOS {
		t.Errorf("自检 platform = %q，期望 %q", report.Platform, runtime.GOOS)
	}
	if report.Window.Width != 1000 || report.Window.Height != 680 ||
		report.Window.MinWidth != 900 || report.Window.MinHeight != 600 {
		t.Errorf("自检窗口尺寸异常：%+v", report.Window)
	}
	if report.Bridge.Ping == nil || report.Bridge.Ping.Protocol != appProtocol {
		t.Errorf("自检 bridge.ping 异常：%+v", report.Bridge.Ping)
	}
}

func TestOnboardingStatePersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop.json")
	app := &App{settingsPath: path, dataDir: dir}
	if !app.GetOnboardingState().Needs {
		t.Fatal("新用户应需要新手教程")
	}
	if err := app.CompleteOnboarding(); err != nil {
		t.Fatalf("记录教程完成失败：%v", err)
	}
	if app.GetOnboardingState().Needs {
		t.Fatal("完成教程后不应再次显示")
	}
	loaded, err := loadDesktopSettings(path)
	if err != nil {
		t.Fatalf("读取设置失败：%v", err)
	}
	if !loaded.OnboardingDone {
		t.Fatal("onboarding_done 应已写入设置文件")
	}
}
