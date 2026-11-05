package main

// app.go 是暴露给前端的 Wails 绑定层（desktop/docs/INTERFACE.md §5）。
//
// 约定：
//   - 每个绑定方法返回 (结果, error)；error 会变成前端 Promise 的 reject，
//     文案一律中文、可操作、不包含账号/密码等敏感信息；
//   - 引擎相关方法通过 ensureBridge 懒启动 Python 桥（首次需要数据时才起）；
//   - Python 桥的 run.* 事件在这里转发为 run:* 前端事件；
//   - CX_DESKTOP_MOCK=1 时走 mock.go 的内置假数据，不启动 Python。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	appProtocol = 1
	appLicense  = "GPL-3.0"
)

// bridgeEventNames 把 Python 桥事件名映射为前端事件名（契约 §5）。
var bridgeEventNames = map[string]string{
	"run.started":  "run:started",
	"run.log":      "run:log",
	"run.stage":    "run:stage",
	"run.progress": "run:progress",
	"run.stopping": "run:stopping",
	"run.finished": "run:finished",
}

// App 是 Wails 绑定的主结构，线程安全。
type App struct {
	mu sync.Mutex

	ctx  context.Context
	mock bool

	engine         engineFinding
	engineDetected bool
	bridge         *Bridge

	// 测试注入点（生产为 nil，走默认实现）
	engineProber  func() engineFinding
	bridgeFactory func(repo, python string) *Bridge
	eventEmitter  func(eventName string, payload any)

	// 运行中的刷课任务（由 run.started/run.finished 维护，用于桥崩溃时补发终态）
	runActive    bool
	runID        string
	runStartedAt string
	runCourseIDs []string
	runStartTime time.Time

	desktop      desktopSettings
	settingsPath string
	dataDir      string

	// 自动更新（updater.go / update_app.go）
	updateState UpdateState
	updateInfo  UpdateInfo
	updateBusy  bool

	// mock 模式下的内存状态
	mockAccounts []Account
	mockSettings Settings
	mockRun      *mockRunState

	// 多账号批量（见 batch.go）
	batchMu        sync.Mutex
	batchRunning   bool
	batchStop      bool
	batchRunEvents chan map[string]any
	batchState     BatchState
}

// NewApp 创建应用实例；不做重活（探测/启动在 startup 或首次调用时进行）。
func NewApp() *App {
	home, _ := userHomeDir()
	dir := dataDirForHome(home)
	settingsPath := settingsPathForDataDir(dir)
	desktop, _ := loadDesktopSettings(settingsPath)

	a := &App{
		mock:         isMockMode(),
		desktop:      desktop,
		settingsPath: settingsPath,
		dataDir:      dir,
		mockSettings: DefaultSettings(),
		mockAccounts: defaultMockAccounts(),
		updateState: UpdateState{
			Supported:      runtime.GOOS == "darwin",
			CurrentVersion: appVersion,
			Message:        "尚未检查更新",
		},
	}
	if a.mock {
		a.engine = mockEngineFinding()
		a.engineDetected = true
	}
	return a
}

func isMockMode() bool {
	v := strings.TrimSpace(os.Getenv("CX_DESKTOP_MOCK"))
	return v == "1" || strings.EqualFold(v, "true")
}

// startup 在窗口创建后调用：保存 context 并探测引擎（不阻塞启动 Python 桥）。
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	// Dock / ⌘Tab 图标跟随系统深浅色（纸/墨两版）
	startDockIconSync()

	if a.mock {
		a.emitBridgeState(bridgeStateReady, "演示模式：使用内置假数据。")
		return
	}
	finding := a.detectEngineNow()
	a.mu.Lock()
	a.engine = finding
	a.engineDetected = true
	a.mu.Unlock()
	if finding.Ready {
		// 首次需要数据时才起桥；这里只把状态同步给前端。
		a.emitBridgeState(bridgeStateStarting, "引擎已就绪，等待首次数据请求…")
	} else {
		a.emitBridgeState(bridgeStateFailed, finding.Message)
	}

	// 自动更新：清理上次遗留的备份；Sparkle 负责检查/安装（GitHub 自更新仅保留为离线兜底）。
	if appPath, err := currentAppBundlePath(); err == nil {
		cleanupStaleUpdateArtifacts(appPath)
	}
	a.initSparkle()
}

// shutdown 在应用退出时调用，确保子进程被清理。
func (a *App) shutdown(_ context.Context) {
	a.batchMu.Lock()
	if a.batchRunning {
		a.batchStop = true
	}
	a.batchMu.Unlock()
	a.mu.Lock()
	b := a.bridge
	a.bridge = nil
	a.mu.Unlock()
	if b != nil {
		b.Close()
	}
}

// bridgeContext 返回 Wails 的 runtime context（可能为 nil，例如 --self-test）。
func (a *App) bridgeContext() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx
}

// emit 向前端发事件；无 GUI（ctx 为 nil）时静默忽略。
// 测试可通过 eventEmitter 注入收集器。
func (a *App) emit(eventName string, payload any) {
	a.mu.Lock()
	custom := a.eventEmitter
	ctx := a.ctx
	a.mu.Unlock()
	if custom != nil {
		custom(eventName, payload)
		return
	}
	if ctx == nil {
		return
	}
	wailsruntime.EventsEmit(ctx, eventName, payload)
}

// emitBridgeState 发 bridge:state 事件。
func (a *App) emitBridgeState(state, message string) {
	a.emit("bridge:state", BridgeStateEvent{State: state, Message: message})
}

// handleBridgeEvent 把 Python 桥事件转发为 run:* 前端事件，
// 并用 run.started/run.finished 维护“运行中任务”状态。
func (a *App) handleBridgeEvent(event string, data json.RawMessage) {
	name, ok := bridgeEventNames[event]
	if !ok {
		return
	}
	decoded := map[string]any{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &decoded)
	}
	switch event {
	case "run.started":
		runID, _ := decoded["run_id"].(string)
		startedAt, _ := decoded["started_at"].(string)
		var courseIDs []string
		if list, ok := decoded["courses"].([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					if id, ok := m["courseId"].(string); ok && id != "" {
						courseIDs = append(courseIDs, id)
					}
				}
			}
		}
		a.mu.Lock()
		a.runActive = true
		if runID != "" {
			a.runID = runID
		}
		if startedAt != "" {
			a.runStartedAt = startedAt
		}
		if len(courseIDs) > 0 {
			a.runCourseIDs = courseIDs
		}
		if a.runStartTime.IsZero() {
			a.runStartTime = time.Now()
		}
		a.mu.Unlock()
	case "run.finished":
		a.notifyBatchRunFinished(decoded)
		a.clearActiveRun()
	}
	a.emit(name, decoded)
}

// handleBridgeState 转发 bridge:state；桥异常重启/失败时补发运行终态。
func (a *App) handleBridgeState(state, message string) {
	a.emitBridgeState(state, message)
	if state == bridgeStateRestarting || state == bridgeStateFailed {
		a.failActiveRun("Python 桥异常退出，本次任务已中断")
	}
}

// clearActiveRun 复位运行状态（不发送事件）。
func (a *App) clearActiveRun() {
	a.mu.Lock()
	a.runActive = false
	a.runID = ""
	a.runStartedAt = ""
	a.runCourseIDs = nil
	a.runStartTime = time.Time{}
	a.mu.Unlock()
}

// failActiveRun 在桥异常中断时补发 run:finished（ok=false/reason=error）并复位状态。
func (a *App) failActiveRun(message string) {
	a.mu.Lock()
	if !a.runActive {
		a.mu.Unlock()
		return
	}
	runID := a.runID
	courses := append([]string(nil), a.runCourseIDs...)
	started := a.runStartTime
	a.runActive = false
	a.runID = ""
	a.runStartedAt = ""
	a.runCourseIDs = nil
	a.runStartTime = time.Time{}
	a.mu.Unlock()

	duration := 0.0
	if !started.IsZero() {
		duration = time.Since(started).Seconds()
	}
	a.emit("run:finished", map[string]any{
		"run_id":     runID,
		"ok":         false,
		"reason":     "error",
		"duration_s": duration,
		"summary": map[string]any{
			"courses": len(courses),
			"errors":  1,
		},
		"message": message,
	})
}

// detectEngineNow 用当前环境探测引擎（测试可注入 engineProber）。
func (a *App) detectEngineNow() engineFinding {
	a.mu.Lock()
	prober := a.engineProber
	a.mu.Unlock()
	if prober != nil {
		return prober()
	}
	return detectEngineDefault()
}

// ensureBridge 返回已启动的桥客户端；引擎未就绪/启动失败时返回中文错误。
func (a *App) ensureBridge() (*Bridge, error) {
	a.mu.Lock()
	if a.mock {
		a.mu.Unlock()
		return nil, errors.New("演示模式下没有 Python 桥，请关闭演示模式后重试。")
	}
	b := a.bridge
	engine := a.engine
	if b == nil {
		if !engine.Ready {
			a.mu.Unlock()
			return nil, errors.New(engine.Message)
		}
		factory := a.bridgeFactory
		if factory != nil {
			b = factory(engine.Repo, engine.PythonExe)
		} else {
			b = NewBridge(engine.Repo, engine.PythonExe)
		}
		if b == nil {
			a.mu.Unlock()
			return nil, errors.New("无法创建 Python 桥客户端。")
		}
		b.SetHandlers(a.handleBridgeEvent, a.handleBridgeState)
		a.bridge = b
	}
	a.mu.Unlock()

	if err := b.Start(); err != nil {
		return nil, err
	}
	return b, nil
}

// ---- 契约 §5 绑定方法 ----

// GetStatus 返回首页/设置页所需的聚合状态（§3.2 + engine_ready/bridge_state）。
func (a *App) GetStatus() (Status, error) {
	if a.mock {
		return a.mockStatus(), nil
	}
	b, err := a.ensureBridge()
	if err != nil {
		return Status{}, err
	}
	st, err := b.Status()
	if err != nil {
		return Status{}, err
	}
	state, _ := b.State()
	st.EngineReady = true
	st.BridgeState = state
	return st, nil
}

// ListAccounts 返回脱敏后的账号列表。
func (a *App) ListAccounts() ([]Account, error) {
	if a.mock {
		return a.mockListAccounts(), nil
	}
	b, err := a.ensureBridge()
	if err != nil {
		return nil, err
	}
	return b.ListAccounts()
}

// AddAccount 新增账号（用户名/密码为空时返回中文错误）。
func (a *App) AddAccount(username, password, name string) (Account, error) {
	if a.mock {
		return a.mockAddAccount(username, password, name)
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return Account{}, errors.New("手机号和密码不能为空。")
	}
	b, err := a.ensureBridge()
	if err != nil {
		return Account{}, err
	}
	return b.AddAccount(username, password, name)
}

// RemoveAccount 删除账号。
func (a *App) RemoveAccount(id string) error {
	if a.mock {
		return a.mockRemoveAccount(id)
	}
	b, err := a.ensureBridge()
	if err != nil {
		return err
	}
	return b.RemoveAccount(id)
}

// GetSettings 返回常用参数。
func (a *App) GetSettings() (Settings, error) {
	if a.mock {
		return a.mockGetSettings(), nil
	}
	b, err := a.ensureBridge()
	if err != nil {
		return Settings{}, err
	}
	return b.GetSettings()
}

// SaveSettings 保存常用参数；前端可只传需要修改的字段（Settings 白名单子集）。
func (a *App) SaveSettings(settings map[string]any) (Settings, error) {
	if a.mock {
		return a.mockSaveSettings(settings)
	}
	if settings == nil {
		return Settings{}, errors.New("设置内容为空，未保存。")
	}
	b, err := a.ensureBridge()
	if err != nil {
		return Settings{}, err
	}
	return b.SaveSettings(settings)
}

// ListCourses 拉取指定账号的课程列表（网络请求，前端需有 loading 态）。
func (a *App) ListCourses(accountID string) (CourseListResult, error) {
	if a.mock {
		return a.mockListCourses(accountID)
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return CourseListResult{}, errors.New("请先选择账号。")
	}
	b, err := a.ensureBridge()
	if err != nil {
		return CourseListResult{}, err
	}
	return b.ListCourses(accountID)
}

// StartRun 开始刷课；参数结构见 RunOptions（INTERFACE.md §3.9）。
func (a *App) StartRun(opts RunOptions) (RunStartResult, error) {
	if a.mock {
		return a.mockStartRun(opts)
	}
	if err := validateRunOptions(opts); err != nil {
		return RunStartResult{}, err
	}
	b, err := a.ensureBridge()
	if err != nil {
		return RunStartResult{}, err
	}
	opts, err = a.fillRunDefaults(b, opts)
	if err != nil {
		return RunStartResult{}, err
	}

	// 先登记 active run（run_start 期间桥若崩溃，也能在终态事件里带上对应信息）。
	a.mu.Lock()
	a.runActive = true
	a.runID = ""
	a.runStartedAt = ""
	a.runCourseIDs = append([]string(nil), opts.CourseIDs...)
	a.runStartTime = time.Now()
	a.mu.Unlock()

	result, err := b.StartRun(opts)
	if err != nil {
		a.mu.Lock()
		if a.runID == "" {
			a.runActive = false
			a.runCourseIDs = nil
			a.runStartTime = time.Time{}
		}
		a.mu.Unlock()
		return RunStartResult{}, err
	}

	a.mu.Lock()
	if a.runActive {
		a.runID = result.RunID
		a.runStartedAt = result.StartedAt
		if t, parseErr := time.Parse("2006-01-02 15:04:05", result.StartedAt); parseErr == nil {
			a.runStartTime = t
		}
	}
	a.mu.Unlock()
	return result, nil
}

// fillRunDefaults 在调用桥之前用当前 settings 补齐缺省的高级参数，并做基本校验，
// 避免前端漏传字段导致桥侧直接返回 bad_request。
func (a *App) fillRunDefaults(b *Bridge, opts RunOptions) (RunOptions, error) {
	settings, err := b.GetSettings()
	if err != nil {
		return opts, fmt.Errorf("读取默认参数失败，无法开始任务：%v", err)
	}
	if opts.Options == (RunSettings{}) {
		// 前端完全没传 options：整组使用全局设置。
		opts.Options = RunSettings{
			Speed:            settings.Speed,
			Jobs:             settings.Jobs,
			NotOpenAction:    settings.NotOpenAction,
			RetryInterval:    settings.RetryInterval,
			SerialVideo:      settings.SerialVideo,
			AddLearningCount: settings.AddLearningCount,
			TargetCount:      settings.TargetCount,
			WorkRedoEnabled:  settings.WorkRedoEnabled,
			WorkMaxRetries:   settings.WorkMaxRetries,
		}
	} else {
		// 部分字段缺省：只补数值/字符串类的零值（bool 无法区分 false 与未传，保持前端传值）。
		if opts.Options.Speed == 0 {
			opts.Options.Speed = settings.Speed
		}
		if opts.Options.Jobs == 0 {
			opts.Options.Jobs = settings.Jobs
		}
		if opts.Options.NotOpenAction == "" {
			opts.Options.NotOpenAction = settings.NotOpenAction
		}
		if opts.Options.RetryInterval == 0 {
			opts.Options.RetryInterval = settings.RetryInterval
		}
		if opts.Options.TargetCount == 0 {
			opts.Options.TargetCount = settings.TargetCount
		}
		if opts.Options.WorkMaxRetries == 0 {
			opts.Options.WorkMaxRetries = settings.WorkMaxRetries
		}
	}
	return opts, validateRunSettings(opts.Options)
}

// validateRunSettings 做与桥侧一致的参数范围校验（中文错误）。
func validateRunSettings(o RunSettings) error {
	if o.Speed < 1.0 || o.Speed > 2.0 {
		return fmt.Errorf("播放速度需在 1.0–2.0 之间（当前 %.1f）。", o.Speed)
	}
	if o.Jobs < 1 || o.Jobs > 8 {
		return fmt.Errorf("并发数需在 1–8 之间（当前 %d）。", o.Jobs)
	}
	if o.NotOpenAction != "retry" && o.NotOpenAction != "continue" {
		return fmt.Errorf("未开放章节的处理方式只能是 retry 或 continue（当前 %q）。", o.NotOpenAction)
	}
	if o.RetryInterval <= 0 || o.RetryInterval > 300 {
		return fmt.Errorf("重试间隔需大于 0 且不超过 300 秒（当前 %.1f）。", o.RetryInterval)
	}
	if o.TargetCount < 1 || o.TargetCount > 1000 {
		return fmt.Errorf("目标任务数需在 1–1000 之间（当前 %d）。", o.TargetCount)
	}
	if o.WorkMaxRetries < 1 || o.WorkMaxRetries > 10 {
		return fmt.Errorf("作业重试次数需在 1–10 之间（当前 %d）。", o.WorkMaxRetries)
	}
	return nil
}

// validateRunOptions 做前端侧的快速校验（桥侧还会再校验一次）。
func validateRunOptions(opts RunOptions) error {
	if strings.TrimSpace(opts.AccountID) == "" {
		return errors.New("请先选择账号。")
	}
	if len(opts.CourseIDs) == 0 {
		return errors.New("请至少选择一门课程。")
	}
	if !opts.Scope.Chapters && !opts.Scope.TaskCenter {
		return errors.New("请至少勾选“章节学习”或“任务中心”中的一项。")
	}
	return nil
}

// StopRun 请求停止当前刷课任务。
func (a *App) StopRun() error {
	if a.mock {
		return a.mockStopRun()
	}
	b, err := a.ensureBridge()
	if err != nil {
		return err
	}
	return b.StopRun()
}

// GetRunState 返回当前运行状态（切换页面回来可恢复）。
func (a *App) GetRunState() (RunState, error) {
	if a.mock {
		return a.mockGetRunState(), nil
	}
	b, err := a.ensureBridge()
	if err != nil {
		return RunState{}, err
	}
	return b.GetRunState()
}

// ListReviews 列出复核条目；days=0 表示全部。
func (a *App) ListReviews(days int) (ReviewListResult, error) {
	if a.mock {
		return a.mockListReviews(days), nil
	}
	if days < 0 {
		days = 0
	}
	b, err := a.ensureBridge()
	if err != nil {
		return ReviewListResult{}, err
	}
	return b.ListReviews(days)
}

// GetReview 返回单条复核全文。
func (a *App) GetReview(submissionID string) (ReviewItem, error) {
	if a.mock {
		return a.mockGetReview(submissionID)
	}
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return ReviewItem{}, errors.New("复核条目标识为空。")
	}
	b, err := a.ensureBridge()
	if err != nil {
		return ReviewItem{}, err
	}
	return b.GetReview(submissionID)
}

// ChooseEngineFolder 打开目录选择器；选中并校验通过后保存并重启桥。取消返回空串。
func (a *App) ChooseEngineFolder() (string, error) {
	ctx := a.bridgeContext()
	if ctx == nil {
		return "", errors.New("界面尚未就绪，请稍后重试。")
	}
	a.mu.Lock()
	defaultDir := a.engine.Repo
	a.mu.Unlock()
	dir, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:            "选择超星助手引擎目录",
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return "", fmt.Errorf("打开目录选择器失败：%v", err)
	}
	if strings.TrimSpace(dir) == "" {
		return "", nil // 用户取消
	}
	if _, err := a.SetEnginePath(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// SetEnginePath 校验目录（含 main.py 与 api/）后保存到 desktop.json 并重启桥。
func (a *App) SetEnginePath(repoPath string) (EngineStatus, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return EngineStatus{}, errors.New("引擎目录不能为空。")
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return EngineStatus{}, fmt.Errorf("无法解析引擎目录路径：%v", err)
	}
	if !isEngineRepo(abs, os.Stat) {
		return EngineStatus{}, errors.New("所选目录不是有效的引擎目录：需要包含 main.py 文件和 api/ 子目录。")
	}

	a.mu.Lock()
	a.desktop.RepoPath = abs
	desktop := a.desktop
	settingsPath := a.settingsPath
	a.mu.Unlock()

	if err := saveDesktopSettings(settingsPath, desktop); err != nil {
		return EngineStatus{}, err
	}

	// 重新探测（此时设置文件已指向新目录）。
	finding := detectEngine(probeOptions{RepoCandidates: []string{abs}})
	a.mu.Lock()
	old := a.bridge
	a.bridge = nil
	a.engine = finding
	a.engineDetected = true
	a.mu.Unlock()
	if old != nil {
		old.SetHandlers(nil, nil)
		old.Close()
	}

	status := finding.status()
	if !finding.Ready {
		a.emitBridgeState(bridgeStateFailed, finding.Message)
		return status, nil
	}
	// 后台重启桥并 ping 一次，让 UI 尽快看到 ready 状态。
	go func() {
		b, err := a.ensureBridge()
		if err != nil {
			return
		}
		_, _ = b.Ping()
	}()
	return status, nil
}

// GetEngineStatus 返回 Go 侧的 Python/仓库探测结果。
func (a *App) GetEngineStatus() EngineStatus {
	if a.mock {
		return a.mockEngineStatus()
	}
	// 已就绪则缓存；未就绪时每次调用都重新探测，
	// 这样设置页“重新检测”在用户装好 Python / 改好路径后能立即生效。
	a.mu.Lock()
	finding := a.engine
	a.mu.Unlock()
	if finding.Ready {
		return finding.status()
	}
	finding = a.detectEngineNow()
	a.mu.Lock()
	a.engine = finding
	a.engineDetected = true
	a.mu.Unlock()
	return finding.status()
}

// OpenDataDir 用系统文件管理器打开数据目录。
func (a *App) OpenDataDir() error {
	if a.mock {
		return nil
	}
	dir := a.dataDirPath()
	if dir == "" {
		return errors.New("无法确定数据目录。")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败：%v", err)
	}
	return openInFileManager(dir)
}

// OpenConfigFile 用默认编辑器打开引擎配置文件。
func (a *App) OpenConfigFile() error {
	if a.mock {
		return nil
	}
	path := filepath.Join(a.dataDirPath(), "config.ini")
	if _, err := os.Stat(path); err != nil {
		return errors.New("配置文件尚不存在：请先运行一次引擎或在设置中保存一次参数后再试。")
	}
	return openInFileManager(path)
}

// GetAppInfo 返回关于页信息（platform 为 additive 字段，供前端判断平台能力）。
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Version:  appVersion,
		Protocol: appProtocol,
		License:  appLicense,
		Platform: runtime.GOOS,
	}
}

// dataDirPath 返回数据目录（可通过 CX_DATA_HOME 覆盖）。
func (a *App) dataDirPath() string {
	if a.dataDir != "" {
		return a.dataDir
	}
	home, err := userHomeDir()
	if err != nil {
		return ""
	}
	return dataDirForHome(home)
}

// openInFileManager 用系统默认方式打开目录/文件。
func openInFileManager(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开失败：请手动前往 %s", path)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// formatTS 生成契约里使用的时间格式（YYYY-MM-DD HH:MM:SS）。
func formatTS(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
