package main

// update_app.go 把 updater.go 的检查/安装能力接成 Wails 绑定与事件：
//
//	GetUpdateState()      启动时取缓存状态（不联网）
//	CheckForUpdate()      主动检查（设置页按钮）
//	StartUpdate()         用户点「下载并安装」后开始；进度通过事件推送
//
// 事件：
//
//	update:state      UpdateState（检查中/有新版本/失败/安装中状态）
//	update:progress   UpdateProgress（下载进度等，低频）
//	update:error      {message}
//
// 安装流程必须是用户显式触发；启动时只自动检查并提示。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// updateStateFromInfo 把内部检查结果转成前端可用的状态。
func updateStateFromInfo(info UpdateInfo) UpdateState {
	state := UpdateState{
		Supported:      runtime.GOOS == "darwin",
		CurrentVersion: info.CurrentVersion,
		Available:      info.Available,
		LatestVersion:  info.LatestVersion,
		Tag:            info.Tag,
		ReleaseURL:     info.ReleaseURL,
		Notes:          info.Notes,
		PublishedAt:    info.PublishedAt,
		AssetName:      info.AssetName,
		AssetSize:      info.AssetSize,
		Digest:         info.Digest,
	}
	if !info.CheckedAt.IsZero() {
		state.CheckedAt = info.CheckedAt.Local().Format("2006-01-02 15:04")
	}
	return state
}

// GetUpdateState 返回缓存的更新状态（不联网；启动自动检查通过事件刷新）。
func (a *App) GetUpdateState() UpdateState {
	a.mu.Lock()
	defer a.mu.Unlock()
	state := a.updateState
	if state.CurrentVersion == "" {
		state.CurrentVersion = appVersion
	}
	state.Supported = runtime.GOOS == "darwin"
	return state
}

// setUpdateState 修改并广播更新状态。
func (a *App) setUpdateState(mut func(*UpdateState)) {
	a.mu.Lock()
	if a.updateState.CurrentVersion == "" {
		a.updateState.CurrentVersion = appVersion
	}
	a.updateState.Supported = runtime.GOOS == "darwin"
	mut(&a.updateState)
	state := a.updateState
	a.mu.Unlock()
	a.emit("update:state", state)
}

// CheckForUpdate 主动检查更新（设置页「检查更新」）。
func (a *App) CheckForUpdate() (UpdateState, error) {
	if a.mock {
		state := a.GetUpdateState()
		state.CheckedAt = time.Now().Format("2006-01-02 15:04")
		state.Message = "演示模式：不联网检查更新"
		return state, nil
	}
	if runtime.GOOS != "darwin" {
		return a.GetUpdateState(), errUpdateUnsupported
	}

	a.setUpdateState(func(s *UpdateState) {
		s.Checking = true
		s.Error = ""
		s.Message = "正在检查更新…"
	})
	info, err := checkForUpdate(context.Background(), defaultUpdateDeps(), appVersion)

	state := updateStateFromInfo(info)
	state.Checking = false
	switch {
	case err != nil && errors.Is(err, errUpdateNoAsset):
		state.Error = err.Error()
		state.Message = "新版本没有当前架构的安装包"
	case err != nil:
		state.Error = err.Error()
		state.Message = "检查更新失败"
	case info.NoReleases:
		state.Message = "还没有发布版本"
	case state.Available:
		state.Message = "发现新版本 v" + state.LatestVersion
	default:
		state.Message = "已是最新版本"
	}

	a.mu.Lock()
	a.updateInfo = info
	a.updateState = state
	a.mu.Unlock()
	a.emit("update:state", state)

	if err != nil && !errors.Is(err, errUpdateNoAsset) {
		return state, err
	}
	return state, nil
}

// autoCheckForUpdate 启动后的静默检查（失败不打扰用户）。
func (a *App) autoCheckForUpdate() {
	time.Sleep(2500 * time.Millisecond)
	if a.mock || runtime.GOOS != "darwin" || !updateAutoCheckEnabled() {
		return
	}
	_, _ = a.CheckForUpdate()
}

// StartUpdate 开始下载并安装更新；立即返回，进度用事件推送。
func (a *App) StartUpdate() (UpdateStartResult, error) {
	if a.mock {
		return UpdateStartResult{}, errors.New("演示模式不支持安装更新")
	}
	if runtime.GOOS != "darwin" {
		return UpdateStartResult{}, errUpdateUnsupported
	}

	a.mu.Lock()
	if a.updateBusy {
		a.mu.Unlock()
		return UpdateStartResult{}, errors.New("更新已在进行中")
	}
	info := a.updateInfo
	if !info.Available || info.AssetURL == "" {
		a.mu.Unlock()
		return UpdateStartResult{}, errors.New("没有可安装的更新，请先检查更新")
	}
	if len(info.Digest) <= len("sha256:") {
		a.mu.Unlock()
		return UpdateStartResult{}, errors.New("发布资源缺少 sha256 校验值，已停止更新")
	}
	a.updateBusy = true
	a.updateState.Installing = true
	a.updateState.Checking = false
	a.updateState.Progress = 0
	a.updateState.Phase = "prepare"
	a.updateState.Error = ""
	a.updateState.Message = "准备下载更新…"
	state := a.updateState
	a.mu.Unlock()
	a.emit("update:state", state)

	go a.performUpdate(info)
	return UpdateStartResult{Started: true}, nil
}

// emitUpdateProgress 发送下载/安装进度（不触发整页重绘，前端局部更新）。
func (a *App) emitUpdateProgress(progress UpdateProgress) {
	a.mu.Lock()
	a.updateState.Progress = progress.Percent
	if progress.Phase != "" {
		a.updateState.Phase = progress.Phase
	}
	if progress.Message != "" {
		a.updateState.Message = progress.Message
	}
	a.mu.Unlock()
	a.emit("update:progress", progress)
}

// failUpdate 统一处理更新失败：复位状态并通知前端。
func (a *App) failUpdate(err error) {
	a.mu.Lock()
	a.updateBusy = false
	a.updateState.Installing = false
	a.updateState.Checking = false
	a.updateState.Error = err.Error()
	a.updateState.Message = "更新失败"
	a.updateState.Phase = "error"
	state := a.updateState
	a.mu.Unlock()
	a.emit("update:state", state)
	a.emit("update:error", map[string]string{"message": err.Error()})
}

// performUpdate 执行下载、校验、替换、重启。
func (a *App) performUpdate(info UpdateInfo) {
	appPath, err := currentAppBundlePath()
	if err != nil {
		a.failUpdate(err)
		return
	}
	backup, err := runUpdatePipeline(context.Background(), defaultUpdateDeps(), info, appPath,
		a.emitUpdateProgress, func(path string) error {
			return exec.Command("open", "-n", path).Start()
		})
	if err != nil {
		a.failUpdate(err)
		return
	}
	_ = backup // 旧版本由下次启动 cleanupStaleUpdateArtifacts 清理
	if ctx := a.bridgeContext(); ctx != nil {
		time.Sleep(700 * time.Millisecond)
		wailsruntime.Quit(ctx)
		return
	}
	os.Exit(0)
}

// runCheckUpdate 实现 --check-update：不打开窗口，联网检查后输出 JSON。
func runCheckUpdate(stdout, stderr io.Writer) int {
	type report struct {
		OK             bool   `json:"ok"`
		CurrentVersion string `json:"current_version"`
		Available      bool   `json:"available"`
		LatestVersion  string `json:"latest_version"`
		Tag            string `json:"tag"`
		AssetName      string `json:"asset_name"`
		DigestPresent  bool   `json:"digest_present"`
		Message        string `json:"message"`
		Error          string `json:"error,omitempty"`
	}
	out := report{CurrentVersion: appVersion}
	info, err := checkForUpdate(context.Background(), defaultUpdateDeps(), appVersion)
	if err != nil {
		out.Error = err.Error()
		out.Message = "检查更新失败"
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(data))
		fmt.Fprintf(stderr, "检查更新失败：%v\n", err)
		return 1
	}
	out.Available = info.Available
	out.LatestVersion = info.LatestVersion
	out.Tag = info.Tag
	out.AssetName = info.AssetName
	out.DigestPresent = strings.HasPrefix(info.Digest, "sha256:")
	switch {
	case info.NoReleases:
		out.Message = "还没有发布版本"
	case info.Available:
		out.Message = "发现新版本 v" + info.LatestVersion
	default:
		out.Message = "已是最新版本"
	}
	out.OK = true
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(stdout, string(data))
	return 0
}
