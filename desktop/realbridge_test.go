package main

// realbridge_test.go 是可选的真实 Python 桥联调测试（默认跳过）。
//
// 运行方式：
//
//	CX_TEST_REAL_BRIDGE=1 go test -run TestRealBridgeIntegration -v ./...
//
// 只在引擎与 api/desktop_bridge.py 都存在时运行；
// 通过 CX_DATA_HOME 指向临时目录，绝不读写真实 ~/.chaoxing；
// 不调用需要联网的命令（courses.list / run.start）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealBridgeIntegration(t *testing.T) {
	if os.Getenv("CX_TEST_REAL_BRIDGE") != "1" {
		t.Skip("设置 CX_TEST_REAL_BRIDGE=1 时才跑真实 Python 桥联调")
	}
	finding := detectEngineDefault()
	if !finding.Ready {
		t.Skipf("引擎未就绪，跳过真实桥联调：%s", finding.Message)
	}
	if _, err := os.Stat(filepath.Join(finding.Repo, "api", "desktop_bridge.py")); err != nil {
		t.Skip("api/desktop_bridge.py 尚未交付，跳过真实桥联调")
	}

	dataHome := t.TempDir()
	b := NewBridge(finding.Repo, finding.PythonExe)
	b.extraEnv = append(b.extraEnv, "CX_DATA_HOME="+dataHome)
	t.Cleanup(b.Close)

	ping, err := b.Ping()
	if err != nil {
		t.Fatalf("ping 失败：%v\nstderr：%v", err, b.StderrTail(10))
	}
	if ping.Protocol != appProtocol {
		t.Errorf("protocol = %d，期望 %d", ping.Protocol, appProtocol)
	}
	if ping.Repo != finding.Repo {
		t.Errorf("repo = %q，期望 %q", ping.Repo, finding.Repo)
	}

	status, err := b.Status()
	if err != nil {
		t.Fatalf("status 失败：%v\nstderr：%v", err, b.StderrTail(10))
	}
	if status.Accounts == nil {
		t.Error("status.accounts 不应为 nil（应为空数组）")
	}
	if status.Settings.Jobs < 1 || status.Settings.Jobs > 8 {
		t.Errorf("status.settings.jobs 超出范围：%d", status.Settings.Jobs)
	}

	settings, err := b.GetSettings()
	if err != nil {
		t.Fatalf("settings.get 失败：%v", err)
	}
	if settings.TaskCenterSubmitMode != "auto" {
		t.Errorf("task_center_submit_mode = %q，期望 auto", settings.TaskCenterSubmitMode)
	}

	updated, err := b.SaveSettings(map[string]any{"speed": 1.5, "jobs": 3})
	if err != nil {
		t.Fatalf("settings.set 失败：%v", err)
	}
	if updated.Speed != 1.5 || updated.Jobs != 3 {
		t.Errorf("settings.set 结果未生效：%+v", updated)
	}

	added, err := b.AddAccount("13800000000", "demo-password", "联调测试账号")
	if err != nil {
		t.Fatalf("accounts.add 失败：%v", err)
	}
	if added.UsernameMasked != "138****0000" {
		t.Errorf("username_masked = %q，期望 138****0000", added.UsernameMasked)
	}
	if added.ID != "13800000000" {
		t.Errorf("account.id = %q，期望 13800000000", added.ID)
	}
	accounts, err := b.ListAccounts()
	if err != nil {
		t.Fatalf("accounts.list 失败：%v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != "13800000000" {
		t.Errorf("accounts.list = %+v，期望只包含联调测试账号", accounts)
	}
	if err := b.RemoveAccount("13800000000"); err != nil {
		t.Fatalf("accounts.remove 失败：%v", err)
	}
	if err := b.RemoveAccount("13800000000"); err == nil {
		t.Error("删除不存在的账号应返回错误")
	} else if !strings.Contains(err.Error(), "不存在") {
		t.Errorf("不存在的账号应返回中文错误，实际：%v", err)
	}

	reviews, err := b.ListReviews(0)
	if err != nil {
		t.Fatalf("reviews.list 失败：%v", err)
	}
	if reviews.Items == nil {
		t.Error("reviews.items 不应为 nil（应为空数组）")
	}

	state, err := b.GetRunState()
	if err != nil {
		t.Fatalf("run.status 失败：%v", err)
	}
	if state.Active {
		t.Errorf("刚启动时不应有运行中的任务：%+v", state)
	}
	if err := b.StopRun(); err == nil {
		t.Error("没有运行中任务时 run.stop 应返回错误")
	}

	if state, _ := b.State(); state != bridgeStateReady {
		t.Errorf("桥状态 = %q，期望 ready", state)
	}

	// 确认没有向真实 ~/.chaoxing 写入：联调用的临时目录应该被创建了。
	if _, err := os.Stat(dataHome); err != nil {
		t.Errorf("临时数据目录不存在：%v", err)
	}
}
