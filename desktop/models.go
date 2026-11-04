package main

// models.go 定义 Go 后端与前端（以及 Python 桥）共享的数据结构。
//
// JSON 字段名严格对齐 desktop/docs/INTERFACE.md：
//   - §2 数据结构（Account/Course/ReviewSummary/ReviewItem/Settings）
//   - §3 命令结果（status/ping/courses.list/run.* /reviews.*）
//   - §5 Wails 绑定返回结构
//
// 说明：StartRun 的参数结构体 RunOptions 的 JSON tag 按契约 §3.9 的 params
// 原样命名（snake_case，例如 account_id / course_ids / scope.task_center /
// options.notopen_action）。前端必须用同样的 snake_case 字段名构造对象；
// 不要用 camelCase，Wails 绑定层按 json tag 解码，字段名不一致会得到零值
// （Go 侧会用当前 settings 补齐缺省值，但账号/课程/范围这类关键字段必须显式传）。

// Account 是脱敏后的账号信息（§2）。
type Account struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	UsernameMasked string `json:"username_masked"`
	LastUsed       string `json:"last_used"`
	HasPassword    bool   `json:"has_password"`
}

// Course 是课程列表项（§2，来自 api/decode.py）。
type Course struct {
	CourseID string `json:"courseId"`
	ClazzID  string `json:"clazzId"`
	Title    string `json:"title"`
	Teacher  string `json:"teacher"`
	Desc     string `json:"desc"`
	CPI      string `json:"cpi"`
}

// ReviewSummary 是复核条目摘要（§2）。
type ReviewSummary struct {
	SubmissionID string `json:"submission_id"`
	TS           string `json:"ts"`
	Day          string `json:"day"`
	Kind         string `json:"kind"`
	Course       string `json:"course"`
	Task         string `json:"task"`
	Status       string `json:"status"`
	Chars        int    `json:"chars"`
	TextPreview  string `json:"text_preview"`
}

// ReviewItem 是复核条目全文（§2）。字段平铺，不内嵌，方便 Wails 生成绑定。
type ReviewItem struct {
	SubmissionID string `json:"submission_id"`
	TS           string `json:"ts"`
	Day          string `json:"day"`
	Kind         string `json:"kind"`
	Course       string `json:"course"`
	Task         string `json:"task"`
	Status       string `json:"status"`
	Chars        int    `json:"chars"`
	TextPreview  string `json:"text_preview"`
	Text         string `json:"text"`
}

// Settings 是 global config.ini [common] 的白名单子集（§2）。
type Settings struct {
	Speed                float64 `json:"speed"`
	Jobs                 int     `json:"jobs"`
	NotOpenAction        string  `json:"notopen_action"`
	RetryInterval        float64 `json:"retry_interval"`
	SerialVideo          bool    `json:"serial_video"`
	AddLearningCount     bool    `json:"add_learning_count"`
	TargetCount          int     `json:"target_count"`
	WorkRedoEnabled      bool    `json:"work_redo_enabled"`
	WorkMaxRetries       int     `json:"work_max_retries"`
	TaskCenterSubmitMode string  `json:"task_center_submit_mode"`
}

// DefaultSettings 是桥不可用时（例如 mock 模式）使用的展示默认值。
func DefaultSettings() Settings {
	return Settings{
		Speed:                1.0,
		Jobs:                 2,
		NotOpenAction:        "continue",
		RetryInterval:        1.0,
		SerialVideo:          false,
		AddLearningCount:     false,
		TargetCount:          100,
		WorkRedoEnabled:      false,
		WorkMaxRetries:       3,
		TaskCenterSubmitMode: "auto",
	}
}

// EngineInfo 是 status 结果里的 engine 字段（§3.2）。
type EngineInfo struct {
	Repo         string `json:"repo"`
	Python       string `json:"python"`
	DataDir      string `json:"data_dir"`
	ConfigExists bool   `json:"config_exists"`
}

// ReviewCounts 是 status 结果里的 reviews 字段。
type ReviewCounts struct {
	Today  int `json:"today"`
	Recent int `json:"recent"`
}

// LastRunInfo 是 status 结果里的 last_run 字段。
type LastRunInfo struct {
	At       string `json:"at"`
	LogMtime string `json:"log_mtime"`
}

// Capabilities 是 status 结果里的 capabilities 字段。
type Capabilities struct {
	TikuProvider  string `json:"tiku_provider"`
	Notification  string `json:"notification"`
	LLMConfigured bool   `json:"llm_configured"`
}

// Status 是 GetStatus 的返回（= §3.2 status 结果 + engine_ready/bridge_state）。
type Status struct {
	Engine       EngineInfo   `json:"engine"`
	Accounts     []Account    `json:"accounts"`
	Settings     Settings     `json:"settings"`
	Reviews      ReviewCounts `json:"reviews"`
	LastRun      LastRunInfo  `json:"last_run"`
	Capabilities Capabilities `json:"capabilities"`
	EngineReady  bool         `json:"engine_ready"`
	BridgeState  string       `json:"bridge_state"`
}

// PingInfo 是 ping 命令的结果（§3.1）。
type PingInfo struct {
	Protocol      int    `json:"protocol"`
	BridgeVersion string `json:"bridge_version"`
	Python        string `json:"python"`
	Repo          string `json:"repo"`
}

// RunScope / RunLimits / RunSettings / RunOptions 对应 §3.9 run.start 的 params。
// 字段名使用 snake_case json tag，与契约逐字一致。

type RunScope struct {
	Chapters       bool `json:"chapters"`
	TaskCenter     bool `json:"task_center"`
	OnlyDiscussion bool `json:"only_discussion"`
}

type RunLimits struct {
	MaxPointsPerCourse int `json:"max_points_per_course"`
	MaxTasksPerCourse  int `json:"max_tasks_per_course"`
}

type RunSettings struct {
	Speed            float64 `json:"speed"`
	Jobs             int     `json:"jobs"`
	NotOpenAction    string  `json:"notopen_action"`
	RetryInterval    float64 `json:"retry_interval"`
	SerialVideo      bool    `json:"serial_video"`
	AddLearningCount bool    `json:"add_learning_count"`
	TargetCount      int     `json:"target_count"`
	WorkRedoEnabled  bool    `json:"work_redo_enabled"`
	WorkMaxRetries   int     `json:"work_max_retries"`
}

// RunOptions 是 StartRun 的参数（§3.9 params 的完整结构）。
type RunOptions struct {
	AccountID string      `json:"account_id"`
	CourseIDs []string    `json:"course_ids"`
	Scope     RunScope    `json:"scope"`
	Limits    RunLimits   `json:"limits"`
	Options   RunSettings `json:"options"`
}

// RunStartResult 是 run.start 的结果。
type RunStartResult struct {
	RunID     string `json:"run_id"`
	StartedAt string `json:"started_at"`
}

// RunState 是 GetRunState 的返回（§3.11 run.status）。
type RunState struct {
	Active    bool     `json:"active"`
	RunID     string   `json:"run_id"`
	StartedAt string   `json:"started_at"`
	CourseIDs []string `json:"course_ids"`
}

// CourseListResult 是 ListCourses 的返回（§3.8；§5 只要求 realname/courses）。
type CourseListResult struct {
	AccountID string   `json:"account_id"`
	RealName  string   `json:"realname"`
	Courses   []Course `json:"courses"`
}

// ReviewListResult 是 ListReviews 的返回（§3.12）。
type ReviewListResult struct {
	MarkdownPath string          `json:"markdown_path"`
	Items        []ReviewSummary `json:"items"`
}

// EngineStatus 是 GetEngineStatus/SetEnginePath 的返回（§5）。
type EngineStatus struct {
	Repo    string `json:"repo"`
	Python  string `json:"python"`
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}

// AppInfo 是 GetAppInfo 的返回（§5）。
// Platform 为 runtime.GOOS（additive），前端据此判断是否启用 darwin 透明材质。
type AppInfo struct {
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
	License  string `json:"license"`
	Platform string `json:"platform"`
}

// BridgeStateEvent 是 bridge:state 事件的 payload（§5）。
type BridgeStateEvent struct {
	State   string `json:"state"`
	Message string `json:"message"`
}
