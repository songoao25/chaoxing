package main

// batch.go：超星助手的多账号顺序批量运行。
//
// 与雨课助手共用同一套产品约定：顺序执行、账号之间停顿、可停止、状态事件、
// 执行器换成现有 Python 引擎（每个账号自动拉取全部课程后调用 StartRun）。

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// BatchAccountState 是批量队列里单个账号的状态。
type BatchAccountState struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Status    string `json:"status"` // queued | running | ok | failed | cancelled
	Message   string `json:"message,omitempty"`
	Courses   int    `json:"courses,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
}

// BatchState 是批量队列快照。
type BatchState struct {
	Running   bool                `json:"running"`
	Stopping  bool                `json:"stopping"`
	Current   string              `json:"current,omitempty"`
	Message   string              `json:"message,omitempty"`
	StartedAt string              `json:"started_at,omitempty"`
	UpdatedAt string              `json:"updated_at,omitempty"`
	Items     []BatchAccountState `json:"items"`
}

const (
	batchStatusQueued    = "queued"
	batchStatusRunning   = "running"
	batchStatusOK        = "ok"
	batchStatusFailed    = "failed"
	batchStatusCancelled = "cancelled"
)

// StartBatch 顺序运行多个账号；每个账号自动拉取全部课程，沿用当前范围与参数。
func (a *App) StartBatch(accountIDs []string, opts RunOptions) (BatchState, error) {
	if a.mock {
		return BatchState{}, fmt.Errorf("演示模式不支持批量运行")
	}
	if len(accountIDs) == 0 {
		return BatchState{}, fmt.Errorf("先选择要批量运行的账号")
	}
	a.mu.Lock()
	running := a.runActive
	a.mu.Unlock()
	if running {
		return BatchState{}, fmt.Errorf("当前有刷课任务在运行，结束或停止后再开批量")
	}

	accounts, err := a.ListAccounts()
	if err != nil {
		return BatchState{}, err
	}
	byID := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}
	selected := make([]Account, 0, len(accountIDs))
	items := make([]BatchAccountState, 0, len(accountIDs))
	for _, id := range accountIDs {
		account, ok := byID[id]
		if !ok {
			continue
		}
		selected = append(selected, account)
		items = append(items, BatchAccountState{AccountID: account.ID, Name: account.Name, Status: batchStatusQueued})
	}
	if len(selected) == 0 {
		return BatchState{}, fmt.Errorf("选中的账号已不存在，请刷新后重试")
	}

	a.batchMu.Lock()
	if a.batchRunning {
		a.batchMu.Unlock()
		return BatchState{}, fmt.Errorf("已有批量任务在运行")
	}
	a.batchRunning = true
	a.batchStop = false
	a.batchRunEvents = make(chan map[string]any, 4)
	startedAt := time.Now().Format(time.RFC3339)
	a.batchState = BatchState{
		Running: true, StartedAt: startedAt, UpdatedAt: startedAt, Items: items,
		Message: fmt.Sprintf("准备运行 %d 个账号", len(selected)),
	}
	state := a.batchState
	a.batchMu.Unlock()

	a.emit("batch:state", state)
	go a.runBatch(selected, opts)
	return state, nil
}

// StopBatch 停止当前批量：停止正在跑的账号，之后的账号标记取消。
func (a *App) StopBatch() BatchState {
	a.batchMu.Lock()
	if !a.batchRunning {
		state := a.batchState
		a.batchMu.Unlock()
		return state
	}
	a.batchStop = true
	a.batchState.Stopping = true
	a.batchState.Message = "正在停止…"
	state := a.batchState
	a.batchMu.Unlock()
	a.emit("batch:state", state)
	go func() {
		if err := a.StopRun(); err != nil {
			// 没有在跑的任务时忽略，批量循环会自行退出。
			_ = err
		}
	}()
	return state
}

// GetBatchState 返回批量队列快照。
func (a *App) GetBatchState() BatchState {
	a.batchMu.Lock()
	defer a.batchMu.Unlock()
	return a.batchState
}

// notifyBatchRunFinished 由 bridge 事件回调，把本次账号运行结果交给批量循环。
func (a *App) notifyBatchRunFinished(data map[string]any) {
	a.batchMu.Lock()
	ch := a.batchRunEvents
	running := a.batchRunning
	a.batchMu.Unlock()
	if !running || ch == nil {
		return
	}
	select {
	case ch <- data:
	default:
	}
}

func (a *App) emitBatchState() {
	a.batchMu.Lock()
	state := a.batchState
	a.batchMu.Unlock()
	a.emit("batch:state", state)
}

func (a *App) updateBatchItem(accountID string, mutate func(*BatchAccountState)) {
	a.batchMu.Lock()
	for i := range a.batchState.Items {
		if a.batchState.Items[i].AccountID == accountID {
			mutate(&a.batchState.Items[i])
			break
		}
	}
	a.batchState.UpdatedAt = time.Now().Format(time.RFC3339)
	a.batchMu.Unlock()
	a.emitBatchState()
}

func (a *App) batchStopped() bool {
	a.batchMu.Lock()
	defer a.batchMu.Unlock()
	return a.batchStop
}

func (a *App) runBatch(accounts []Account, opts RunOptions) {
	defer func() {
		a.batchMu.Lock()
		a.batchRunning = false
		a.batchStop = false
		a.batchRunEvents = nil
		a.batchState.Running = false
		a.batchState.Stopping = false
		a.batchState.Current = ""
		a.batchState.Message = a.batchSummaryLocked()
		a.batchState.UpdatedAt = time.Now().Format(time.RFC3339)
		state := a.batchState
		a.batchMu.Unlock()
		a.emit("batch:state", state)
	}()

	for index, account := range accounts {
		if a.batchStopped() {
			a.cancelBatchRemaining(accounts[index:], "已停止")
			return
		}
		startedAt := time.Now().Format(time.RFC3339)
		a.batchMu.Lock()
		a.batchState.Current = account.ID
		a.batchState.Message = "正在准备账号：" + account.Name
		a.batchMu.Unlock()
		a.updateBatchItem(account.ID, func(item *BatchAccountState) {
			item.Status = batchStatusRunning
			item.StartedAt = startedAt
			item.Message = "正在拉取课程…"
		})

		courses, err := a.ListCourses(account.ID)
		if err != nil {
			a.finishBatchItem(account.ID, batchStatusFailed, "拉取课程失败："+err.Error(), 0)
			continue
		}
		if a.batchStopped() {
			a.finishBatchItem(account.ID, batchStatusCancelled, "已停止", 0)
			a.cancelBatchRemaining(accounts[index+1:], "已停止")
			return
		}
		if len(courses.Courses) == 0 {
			a.finishBatchItem(account.ID, batchStatusFailed, "这个账号没有拉取到课程", 0)
			continue
		}
		courseIDs := make([]string, 0, len(courses.Courses))
		for _, course := range courses.Courses {
			if course.CourseID != "" {
				courseIDs = append(courseIDs, course.CourseID)
			}
		}
		a.updateBatchItem(account.ID, func(item *BatchAccountState) {
			item.Courses = len(courseIDs)
			item.Message = fmt.Sprintf("运行中（%d 门课程）", len(courseIDs))
		})

		runOpts := opts
		runOpts.AccountID = account.ID
		runOpts.CourseIDs = courseIDs
		if _, err := a.StartRun(runOpts); err != nil {
			a.finishBatchItem(account.ID, batchStatusFailed, "启动失败："+err.Error(), len(courseIDs))
			continue
		}

		result, stopped := a.waitBatchRunFinished()
		if stopped || a.batchStopped() {
			a.finishBatchItem(account.ID, batchStatusCancelled, "已停止", len(courseIDs))
			a.cancelBatchRemaining(accounts[index+1:], "已停止")
			return
		}
		ok, _ := result["ok"].(bool)
		reason, _ := result["reason"].(string)
		message, _ := result["message"].(string)
		switch {
		case ok && reason != "stopped":
			a.finishBatchItem(account.ID, batchStatusOK, "完成", len(courseIDs))
		case reason == "stopped":
			a.finishBatchItem(account.ID, batchStatusCancelled, "已停止", len(courseIDs))
		default:
			if message == "" {
				message = "运行中断"
			}
			a.finishBatchItem(account.ID, batchStatusFailed, message, len(courseIDs))
		}

		if index < len(accounts)-1 {
			delay := time.Duration(30+rand.Intn(61)) * time.Second
			a.batchMu.Lock()
			a.batchState.Message = fmt.Sprintf("等待 %s 后运行下一个账号", delay.Round(time.Second))
			a.batchMu.Unlock()
			a.emitBatchState()
			if !a.sleepBatch(delay) {
				a.cancelBatchRemaining(accounts[index+1:], "已停止")
				return
			}
		}
	}
}

// waitBatchRunFinished 等待本次账号运行结束；返回结果与是否被停止。
func (a *App) waitBatchRunFinished() (map[string]any, bool) {
	a.batchMu.Lock()
	ch := a.batchRunEvents
	a.batchMu.Unlock()
	if ch == nil {
		return map[string]any{"ok": false, "reason": "error", "message": "批量通道未初始化"}, false
	}
	timeout := time.NewTimer(8 * time.Hour)
	defer timeout.Stop()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case data := <-ch:
			return data, false
		case <-ticker.C:
			if a.batchStopped() {
				return nil, true
			}
		case <-timeout.C:
			_ = a.StopRun()
			return map[string]any{"ok": false, "reason": "timeout", "message": "单账号运行超时（8 小时）"}, false
		}
	}
}

func (a *App) sleepBatch(delay time.Duration) bool {
	deadline := time.Now().Add(delay)
	for time.Now().Before(deadline) {
		if a.batchStopped() {
			return false
		}
		step := 500 * time.Millisecond
		if remaining := time.Until(deadline); remaining < step {
			step = remaining
		}
		time.Sleep(step)
	}
	return !a.batchStopped()
}

func (a *App) finishBatchItem(accountID, status, message string, courses int) {
	a.updateBatchItem(accountID, func(item *BatchAccountState) {
		item.Status = status
		item.Message = message
		item.EndedAt = time.Now().Format(time.RFC3339)
		if courses > 0 {
			item.Courses = courses
		}
	})
}

func (a *App) cancelBatchRemaining(accounts []Account, message string) {
	for _, account := range accounts {
		a.updateBatchItem(account.ID, func(item *BatchAccountState) {
			if item.Status == batchStatusQueued || item.Status == batchStatusRunning {
				item.Status = batchStatusCancelled
				item.Message = message
				item.EndedAt = time.Now().Format(time.RFC3339)
			}
		})
	}
}

func (a *App) batchSummaryLocked() string {
	okCount, failCount, cancelCount := 0, 0, 0
	for _, item := range a.batchState.Items {
		switch item.Status {
		case batchStatusOK:
			okCount++
		case batchStatusFailed:
			failCount++
		case batchStatusCancelled:
			cancelCount++
		}
	}
	if a.batchState.Items == nil {
		return "空闲"
	}
	return fmt.Sprintf("批量结束：成功 %d，失败 %d，取消 %d", okCount, failCount, cancelCount)
}

// ensure imports used (context kept for future cancellation wiring)
var _ = context.Background
var _ = sync.Mutex{}
