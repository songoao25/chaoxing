package main

// mock.go 实现 CX_DESKTOP_MOCK=1 时的内置假数据：
// 不启动 Python、不联网，供前端离线联调、截图验收与 --self-test 使用。
//
// 数据全部为合成示例，不含真实账号信息。

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// defaultMockAccounts 返回演示账号。
func defaultMockAccounts() []Account {
	return []Account{
		{
			ID:             "13800000001",
			Name:           "演示同学",
			UsernameMasked: "138****0001",
			LastUsed:       "2026-11-04 09:30",
			HasPassword:    true,
		},
		{
			ID:             "13900000002",
			Name:           "示例同学",
			UsernameMasked: "139****0002",
			LastUsed:       "",
			HasPassword:    true,
		},
	}
}

// mockCourses 返回演示课程。
func mockCourses() []Course {
	return []Course{
		{
			CourseID: "2151141",
			ClazzID:  "8831201",
			Title:    "大学生心理健康教育",
			Teacher:  "王老师",
			Desc:     "演示课程：通识必修",
			CPI:      "mock-cpi-1",
		},
		{
			CourseID: "189191",
			ClazzID:  "7730012",
			Title:    "中国近现代史纲要",
			Teacher:  "李老师",
			Desc:     "演示课程：思政必修",
			CPI:      "mock-cpi-2",
		},
		{
			CourseID: "302457",
			ClazzID:  "6612340",
			Title:    "Python 程序设计",
			Teacher:  "张老师",
			Desc:     "演示课程：专业选修",
			CPI:      "mock-cpi-3",
		},
	}
}

// mockReviewSummaries 返回演示复核摘要（今天 2 条、更早 1 条）。
func mockReviewSummaries() []ReviewSummary {
	return []ReviewSummary{
		{
			SubmissionID: "mock-1",
			TS:           "2026-11-04 09:31:12",
			Day:          "2026-11-04",
			Kind:         "测验简答",
			Course:       "大学生心理健康教育",
			Task:         "1.2 压力与应对 单元测验",
			Status:       "待提交",
			Chars:        186,
			TextPreview:  "压力是机体在内外环境刺激下产生的一种非特异性反应，适度的压力有助于提升专注力……",
		},
		{
			SubmissionID: "mock-2",
			TS:           "2026-11-04 09:35:40",
			Day:          "2026-11-04",
			Kind:         "讨论回复",
			Course:       "中国近现代史纲要",
			Task:         "第 2 章 讨论：近代化道路的选择",
			Status:       "已提交",
			Chars:        92,
			TextPreview:  "结合课程内容，我认为近代化道路的选择受到国内外多重因素影响……",
		},
		{
			SubmissionID: "mock-3",
			TS:           "2026-11-02 14:03:00",
			Day:          "2026-11-02",
			Kind:         "作业简答",
			Course:       "Python 程序设计",
			Task:         "第 4 周作业：函数与模块",
			Status:       "失败",
			Chars:        140,
			TextPreview:  "函数是把一段可复用逻辑封装起来的基本单位，使用 def 关键字定义……",
		},
	}
}

// mockReviewTexts 是演示复核全文。
func mockReviewTexts() map[string]string {
	return map[string]string{
		"mock-1": "压力是机体在内外环境刺激下产生的一种非特异性反应。适度的压力有助于提升专注力与行动效率，" +
			"但长期过高的压力会削弱免疫系统、影响睡眠质量，并可能引发焦虑情绪。常见应对方式包括：" +
			"1）规律作息与适度运动；2）把大目标拆分成可执行的小步骤；3）主动寻求同伴和老师的支持；" +
			"4）必要时通过学校心理咨询中心获得专业帮助。",
		"mock-2": "结合课程内容，我认为近代化道路的选择受到国内外多重因素影响。" +
			"一方面，列强入侵打破了原有的社会结构；另一方面，不同阶层对变革的诉求并不一致。" +
			"最终道路的选择是历史条件、社会力量与国际环境共同作用的结果。",
		"mock-3": "函数是把一段可复用逻辑封装起来的基本单位，使用 def 关键字定义，通过参数接收输入、通过 return 返回结果。" +
			"模块则是把相关函数与变量组织到一个 .py 文件中，使用 import 语句导入。" +
			"合理拆分函数与模块可以显著提升代码的可读性与可维护性。",
	}
}

// mockEngineFinding 返回演示模式下的引擎状态（始终就绪）。
func mockEngineFinding() engineFinding {
	return engineFinding{
		Repo:      "（演示模式）",
		PythonExe: "",
		Python:    "3.14.7",
		Ready:     true,
		Message:   "演示模式：使用内置假数据，不会连接真实引擎。",
	}
}

// mockRunState 是 mock 模式下的运行状态。
type mockRunState struct {
	mu             sync.Mutex
	active         bool
	runID          string
	startedAt      time.Time
	accountID      string
	usernameMasked string
	courses        []Course
	stopCh         chan struct{}
}

// ---- mock 数据访问 ----

func (a *App) mockEngineStatus() EngineStatus {
	return EngineStatus{
		Repo:    "（演示模式）",
		Python:  "3.14.7",
		Ready:   true,
		Message: "演示模式：使用内置假数据，不会连接真实引擎。",
	}
}

func (a *App) mockStatus() Status {
	a.mu.Lock()
	settings := a.mockSettings
	accounts := append([]Account(nil), a.mockAccounts...)
	a.mu.Unlock()
	return Status{
		Engine: EngineInfo{
			Repo:         "（演示模式）",
			Python:       "3.14.7",
			DataDir:      "~/.chaoxing",
			ConfigExists: true,
		},
		Accounts: accounts,
		Settings: settings,
		Reviews:  ReviewCounts{Today: 2, Recent: 3},
		LastRun:  LastRunInfo{At: "2026-11-04 09:30", LogMtime: "2026-11-04 09:41"},
		Capabilities: Capabilities{
			TikuProvider:  "AI（演示）",
			Notification:  "ServerChan（演示）",
			LLMConfigured: true,
		},
		EngineReady: true,
		BridgeState: bridgeStateReady,
	}
}

func (a *App) mockListAccounts() []Account {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Account(nil), a.mockAccounts...)
}

func (a *App) mockAddAccount(username, password, name string) (Account, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return Account{}, errors.New("手机号和密码不能为空。")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, acc := range a.mockAccounts {
		if acc.ID == username {
			return Account{}, errors.New("该账号已存在，无需重复添加。")
		}
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = "演示账号"
	}
	account := Account{
		ID:             username,
		Name:           displayName,
		UsernameMasked: maskUsername(username),
		LastUsed:       "",
		HasPassword:    true,
	}
	a.mockAccounts = append(a.mockAccounts, account)
	return account, nil
}

func (a *App) mockRemoveAccount(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, acc := range a.mockAccounts {
		if acc.ID == id {
			a.mockAccounts = append(a.mockAccounts[:i], a.mockAccounts[i+1:]...)
			return nil
		}
	}
	return errors.New("账号不存在或已被删除。")
}

func (a *App) mockGetSettings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mockSettings
}

// mockSaveSettings 把前端传来的部分字段合并进当前设置（只处理 Settings 白名单）。
func (a *App) mockSaveSettings(settings map[string]any) (Settings, error) {
	if settings == nil {
		return Settings{}, errors.New("设置内容为空，未保存。")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.mockSettings
	if v, ok := settings["speed"].(float64); ok {
		next.Speed = v
	}
	if v, ok := settings["jobs"].(float64); ok {
		next.Jobs = int(v)
	}
	if v, ok := settings["notopen_action"].(string); ok {
		next.NotOpenAction = v
	}
	if v, ok := settings["retry_interval"].(float64); ok {
		next.RetryInterval = v
	}
	if v, ok := settings["serial_video"].(bool); ok {
		next.SerialVideo = v
	}
	if v, ok := settings["add_learning_count"].(bool); ok {
		next.AddLearningCount = v
	}
	if v, ok := settings["target_count"].(float64); ok {
		next.TargetCount = int(v)
	}
	if v, ok := settings["work_redo_enabled"].(bool); ok {
		next.WorkRedoEnabled = v
	}
	if v, ok := settings["work_max_retries"].(float64); ok {
		next.WorkMaxRetries = int(v)
	}
	if next.Speed < 1.0 || next.Speed > 2.0 {
		return Settings{}, errors.New("播放速度需在 1.0–2.0 之间。")
	}
	if next.Jobs < 1 || next.Jobs > 8 {
		return Settings{}, errors.New("并发数需在 1–8 之间。")
	}
	if next.NotOpenAction != "retry" && next.NotOpenAction != "continue" {
		return Settings{}, errors.New("未开放章节处理方式只能是 retry 或 continue。")
	}
	if next.WorkMaxRetries < 1 || next.WorkMaxRetries > 10 {
		return Settings{}, errors.New("作业重试次数需在 1–10 之间。")
	}
	a.mockSettings = next
	return next, nil
}

func (a *App) mockListCourses(accountID string) (CourseListResult, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return CourseListResult{}, errors.New("请先选择账号。")
	}
	a.mu.Lock()
	realName := "演示同学"
	for _, acc := range a.mockAccounts {
		if acc.ID == accountID {
			realName = acc.Name
			break
		}
	}
	a.mu.Unlock()
	return CourseListResult{
		AccountID: accountID,
		RealName:  realName,
		Courses:   mockCourses(),
	}, nil
}

func (a *App) mockStartRun(opts RunOptions) (RunStartResult, error) {
	if err := validateRunOptions(opts); err != nil {
		return RunStartResult{}, err
	}
	a.mu.Lock()
	if a.mockRun != nil && a.mockRun.active {
		a.mu.Unlock()
		return RunStartResult{}, errors.New("已有刷课任务在运行，请等待结束或先停止。")
	}
	now := time.Now()
	run := &mockRunState{
		active:    true,
		runID:     "r-" + now.Format("20060102-150405"),
		startedAt: now,
		accountID: opts.AccountID,
		stopCh:    make(chan struct{}),
	}
	for _, acc := range a.mockAccounts {
		if acc.ID == opts.AccountID {
			run.usernameMasked = acc.UsernameMasked
			break
		}
	}
	allCourses := mockCourses()
	byID := map[string]Course{}
	for _, c := range allCourses {
		byID[c.CourseID] = c
	}
	for _, id := range opts.CourseIDs {
		course, ok := byID[id]
		if !ok {
			course = Course{CourseID: id, Title: "演示课程 " + id}
		}
		run.courses = append(run.courses, course)
	}
	a.mockRun = run
	a.mu.Unlock()

	startedCourses := make([]map[string]any, 0, len(run.courses))
	for _, c := range run.courses {
		startedCourses = append(startedCourses, map[string]any{
			"courseId": c.CourseID,
			"title":    c.Title,
		})
	}
	a.emit("run:started", map[string]any{
		"run_id":          run.runID,
		"started_at":      formatTS(now),
		"account_id":      run.accountID,
		"username_masked": run.usernameMasked,
		"courses":         startedCourses,
		"scope": map[string]any{
			"chapters":        opts.Scope.Chapters,
			"task_center":     opts.Scope.TaskCenter,
			"only_discussion": opts.Scope.OnlyDiscussion,
		},
	})
	go a.mockRunLoop(run)

	return RunStartResult{RunID: run.runID, StartedAt: formatTS(now)}, nil
}

// mockRunLoop 用定时器模拟一次刷课运行并发出 §4 事件。
func (a *App) mockRunLoop(run *mockRunState) {
	stopCh := run.stopCh
	sleepOrStop := func(d time.Duration) bool {
		select {
		case <-stopCh:
			return true
		case <-time.After(d):
			return false
		}
	}
	logLine := func(level, text string) {
		a.emit("run:log", map[string]any{
			"run_id": run.runID,
			"level":  level,
			"text":   text,
			"ts":     formatTS(time.Now()),
		})
	}
	stage := func(name, text string) {
		a.emit("run:stage", map[string]any{
			"run_id": run.runID,
			"stage":  name,
			"text":   text,
		})
	}

	logLine("info", "（演示模式）开始刷课任务，不会产生真实学习行为。")
	stage("login", "正在登录学习通…")
	if sleepOrStop(300 * time.Millisecond) {
		a.finishMockRun(run, "stopped", "任务已停止。", 0)
		return
	}
	logLine("info", "登录成功（演示）。")
	stage("courses", fmt.Sprintf("共 %d 门课程待处理", len(run.courses)))

	total := len(run.courses)
	errors := 0
	for i, course := range run.courses {
		if sleepOrStop(350 * time.Millisecond) {
			a.finishMockRun(run, "stopped", "任务已停止。", errors)
			return
		}
		a.emit("run:progress", map[string]any{
			"run_id":    run.runID,
			"kind":      "course",
			"course_id": course.CourseID,
			"title":     course.Title,
			"done":      i,
			"total":     total,
		})
		stage("chapters", "正在学习："+course.Title)
		logLine("info", "正在处理《"+course.Title+"》的章节任务（演示）。")
		if sleepOrStop(400 * time.Millisecond) {
			a.finishMockRun(run, "stopped", "任务已停止。", errors)
			return
		}
		logLine("info", "《"+course.Title+"》处理完成（演示）。")
		a.emit("run:progress", map[string]any{
			"run_id":    run.runID,
			"kind":      "course",
			"course_id": course.CourseID,
			"title":     course.Title,
			"done":      i + 1,
			"total":     total,
		})
	}
	stage("review", "正在整理复核内容…")
	if sleepOrStop(300 * time.Millisecond) {
		a.finishMockRun(run, "stopped", "任务已停止。", errors)
		return
	}
	stage("done", "任务完成")
	logLine("info", "（演示模式）任务结束。")
	a.finishMockRun(run, "completed", "演示运行完成，去复核页看看结果吧。", errors)
}

// finishMockRun 收尾：更新状态并发出 run.finished。
func (a *App) finishMockRun(run *mockRunState, reason, message string, errCount int) {
	run.mu.Lock()
	if !run.active {
		run.mu.Unlock()
		return
	}
	run.active = false
	duration := time.Since(run.startedAt).Seconds()
	run.mu.Unlock()

	ok := reason == "completed"
	if reason == "stopped" {
		a.emit("run:stopping", map[string]any{"run_id": run.runID})
	}
	a.emit("run:finished", map[string]any{
		"run_id":     run.runID,
		"ok":         ok,
		"reason":     reason,
		"duration_s": duration,
		"summary": map[string]any{
			"courses": len(run.courses),
			"errors":  errCount,
		},
		"message": message,
	})
}

func (a *App) mockStopRun() error {
	a.mu.Lock()
	run := a.mockRun
	if run == nil || !run.active {
		a.mu.Unlock()
		return errors.New("当前没有正在运行的刷课任务。")
	}
	run.mu.Lock()
	if run.stopCh != nil {
		close(run.stopCh)
		run.stopCh = nil
	}
	run.mu.Unlock()
	a.mu.Unlock()
	return nil
}

func (a *App) mockGetRunState() RunState {
	a.mu.Lock()
	run := a.mockRun
	a.mu.Unlock()
	if run == nil || !run.active {
		return RunState{Active: false, RunID: "", StartedAt: "", CourseIDs: []string{}}
	}
	courseIDs := make([]string, 0, len(run.courses))
	for _, c := range run.courses {
		courseIDs = append(courseIDs, c.CourseID)
	}
	return RunState{
		Active:    true,
		RunID:     run.runID,
		StartedAt: formatTS(run.startedAt),
		CourseIDs: courseIDs,
	}
}

func (a *App) mockListReviews(days int) ReviewListResult {
	items := mockReviewSummaries()
	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days)
		filtered := make([]ReviewSummary, 0, len(items))
		for _, item := range items {
			day, err := time.Parse("2006-01-02", item.Day)
			if err == nil && day.Before(time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, cutoff.Location())) {
				continue
			}
			filtered = append(filtered, item)
		}
		items = filtered
	}
	if items == nil {
		items = []ReviewSummary{}
	}
	return ReviewListResult{
		MarkdownPath: "~/.chaoxing/reviews/2026-11-04.md",
		Items:        items,
	}
}

func (a *App) mockGetReview(submissionID string) (ReviewItem, error) {
	texts := mockReviewTexts()
	for _, item := range mockReviewSummaries() {
		if item.SubmissionID != submissionID {
			continue
		}
		return ReviewItem{
			SubmissionID: item.SubmissionID,
			TS:           item.TS,
			Day:          item.Day,
			Kind:         item.Kind,
			Course:       item.Course,
			Task:         item.Task,
			Status:       item.Status,
			Chars:        item.Chars,
			TextPreview:  item.TextPreview,
			Text:         texts[submissionID],
		}, nil
	}
	return ReviewItem{}, errors.New("复核条目不存在或已被清理。")
}

// maskUsername 把手机号/用户名脱敏成 138****9876 形式。
func maskUsername(username string) string {
	runes := []rune(username)
	if len(runes) <= 4 {
		return username
	}
	if len(runes) >= 8 {
		return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
	}
	return string(runes[:2]) + "****" + string(runes[len(runes)-2:])
}
