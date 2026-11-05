package main

import (
	"strings"
	"testing"
)

func TestStartBatchRejectsEmptySelection(t *testing.T) {
	app := &App{}
	if _, err := app.StartBatch(nil, RunOptions{}); err == nil || !strings.Contains(err.Error(), "账号") {
		t.Fatalf("空选择应返回友好错误，实际：%v", err)
	}
}

func TestStartBatchRejectsWhenRunActive(t *testing.T) {
	app := &App{}
	app.runActive = true
	if _, err := app.StartBatch([]string{"acc_1"}, RunOptions{}); err == nil || !strings.Contains(err.Error(), "运行") {
		t.Fatalf("已有任务运行时应拒绝，实际：%v", err)
	}
}

func TestBatchSummary(t *testing.T) {
	app := &App{}
	app.batchState = BatchState{Items: []BatchAccountState{
		{AccountID: "a", Status: batchStatusOK},
		{AccountID: "b", Status: batchStatusFailed},
		{AccountID: "c", Status: batchStatusCancelled},
		{AccountID: "d", Status: batchStatusQueued},
	}}
	app.batchMu.Lock()
	summary := app.batchSummaryLocked()
	app.batchMu.Unlock()
	if !strings.Contains(summary, "成功 1") || !strings.Contains(summary, "失败 1") || !strings.Contains(summary, "取消 1") {
		t.Fatalf("批量汇总不对：%s", summary)
	}
}

func TestGetBatchStateDefaultIdle(t *testing.T) {
	app := &App{}
	state := app.GetBatchState()
	if state.Running || len(state.Items) != 0 {
		t.Fatalf("默认批量状态应为空闲：%+v", state)
	}
}

func TestCancelBatchRemainingOnlyTouchesUnfinished(t *testing.T) {
	app := &App{}
	app.batchState = BatchState{Items: []BatchAccountState{
		{AccountID: "a", Status: batchStatusOK},
		{AccountID: "b", Status: batchStatusQueued},
		{AccountID: "c", Status: batchStatusRunning},
	}}
	app.cancelBatchRemaining([]Account{{ID: "b", Name: "b"}, {ID: "c", Name: "c"}}, "已停止")
	app.batchMu.Lock()
	defer app.batchMu.Unlock()
	if app.batchState.Items[0].Status != batchStatusOK {
		t.Fatal("已完成账号不应被取消")
	}
	if app.batchState.Items[1].Status != batchStatusCancelled || app.batchState.Items[2].Status != batchStatusCancelled {
		t.Fatalf("未完成账号应标记取消：%+v", app.batchState.Items)
	}
}
