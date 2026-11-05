# 超星助手 · 三端接口契约（v1）

本文件是 **Python 桥 ⇄ Go 后端 ⇄ 前端 UI** 的唯一接口事实来源。
三端各自实现时必须逐字段对齐；任何一方需要改字段，先在群里说、改本文件、再改代码。
协议版本：`protocol: 1`（`ping` 返回）。

## 0. 进程与数据流

```
┌────────────────────┐  Wails 绑定 / 事件  ┌──────────────┐  NDJSON(stdin/stdout)  ┌─────────────────────┐
│ 前端（frontend/dist）│ ◄────────────────► │ Go 后端(App)  │ ◄────────────────────► │ Python 桥           │
└────────────────────┘                    └──────────────┘                        │ api/desktop_bridge.py│
                                                                                  └──────────┬──────────┘
                                                                                             │ 复用现有引擎
                                                                                  ┌──────────▼──────────┐
                                                                                  │ api/* main.py 等     │
                                                                                  │ 数据目录 ~/.chaoxing │
                                                                                  └─────────────────────┘
```

- Go 在第一次需要引擎数据时启动 Python 桥：`[python] -u -m api.desktop_bridge`，
  工作目录 = 引擎仓库根目录（能 import `api` 包、存在 `main.py`）。
- 环境变量至少保证：`PYTHONUNBUFFERED=1`、`PYTHONIOENCODING=utf-8`、继承用户环境（含 `CX_DATA_HOME` 若设置了）。
- 桥的 **stdout 只允许出现 NDJSON 协议行**；任何诊断输出必须走 stderr（Go 收集进自己的日志，不解析）。
- 同时只允许一个刷课任务（`run`）；重复 `run.start` 返回 `busy`。
- 桥进程异常退出时 Go 负责重启（退避），并通过 `bridge:state` 事件通知前端。
- 前端不直接接触 Python；所有数据都经 Go 绑定/事件。

## 1. NDJSON 帧格式

每行一个 UTF-8 JSON 对象，行内不得出现换行。单行上限 1 MiB。

请求（Go → 桥）：

```json
{"id":"1","cmd":"status","params":{}}
```

成功响应（桥 → Go）：

```json
{"id":"1","ok":true,"result":{...}}
```

失败响应：

```json
{"id":"1","ok":false,"error":{"code":"not_found","message":"账号不存在","detail":"可选"}}
```

事件（桥 → Go，无 `id`，可随时到达）：

```json
{"event":"run.log","data":{"level":"info","text":"...","ts":"2026-11-04 21:00:00"}}
```

- `id` 由 Go 生成（字符串，自增），同一时刻唯一。
- 响应必须原样带回 `id`；Go 按 `id` 匹配待处理请求。
- 请求默认超时：普通命令 30s；`courses.list` 120s；`run.start` 30s（立即返回）。

### 错误码（三端统一）

| code | 含义 |
|---|---|
| `bad_request` | 参数缺失/类型不对/未知命令 |
| `not_found` | 账号、课程、复核条目等不存在 |
| `busy` | 已有刷课任务在跑 |
| `engine_error` | 引擎内部异常（附 `message`/`detail`） |
| `login_failed` | 登录失败（账号密码错误、风控、验证码失败等） |
| `network_error` | 网络请求失败 |
| `config_error` | 配置读写失败或被拒绝 |
| `not_ready` | 桥尚未就绪（Go 内部使用，不必下发前端） |
| `internal` | 未分类异常 |

## 2. 数据结构

```ts
// 账号（脱敏后发给前端）
Account = {
  id: string;              // 内部 id = username（仅本机使用，不入日志）
  name: string;            // 平台昵称，可能为空
  username_masked: string; // 例如 138****9876
  last_used: string;       // "2026-10-01 12:30" 或 ""
  has_password: boolean;
}

// 课程（字段来自 api/decode.py decode_course_list）
Course = {
  courseId: string;
  clazzId: string;
  title: string;
  teacher: string;         // 已清洗的纯文本，可能为空
  desc: string;            // 可能为空
  cpi: string;             // 页面参数，前端不要使用
}

// 复核条目摘要
ReviewSummary = {
  submission_id: string;
  ts: string;              // "2026-09-21 14:03:00"
  day: string;             // "2026-09-21"
  kind: string;            // 测验简答 / 作业简答 / 讨论回复 / AI实践作答
  course: string;
  task: string;
  status: string;          // 待提交 / 已提交 / 取消 / 失败 ...
  chars: number;
  text_preview: string;    // 前 120 字
}

// 复核条目全文
ReviewItem = ReviewSummary & { text: string; }  // text 为完整正文

// 设置（global config.ini [common] 的白名单子集）
Settings = {
  speed: number;                 // 1.0–2.0
  jobs: number;                  // 1–8
  notopen_action: "retry"|"continue";   // GUI 不允许 ask
  retry_interval: number;        // 秒
  serial_video: boolean;
  add_learning_count: boolean;
  target_count: number;          // 1–1000
  work_redo_enabled: boolean;
  work_max_retries: number;      // 1–10
  task_center_submit_mode: "auto";      // GUI 固定 auto（confirm 需要终端）
}
```

## 3. 命令（Python 桥）

以下 `params` 省略表示 `{}`。

### 3.1 `ping`
结果：`{"protocol":1,"bridge_version":"0.1.0","python":"3.14.7","repo":"/abs/path"}`

### 3.2 `status`
结果：

```json
{
  "engine": {"repo":"/abs/path","python":"3.14.7","data_dir":"~/.chaoxing",
             "config_exists": true},
  "accounts": [Account],
  "settings": Settings,
  "reviews": {"today": 0, "recent": 0},
  "last_run": {"at": "2026-10-01 12:30", "log_mtime": "2026-10-01 15:00"},
  "capabilities": {"tiku_provider": "AI", "notification": "ServerChan",
                    "llm_configured": true}
}
```

`last_run.at` 取最近 run 配置文件的 mtime；无则 `""`。`llm_configured` 看 `[tiku]` 里是否有非空 key/endpoint（不校验网络）。

### 3.3 `accounts.list` → `{"accounts":[Account]}`

### 3.4 `accounts.add` params `{"username":"手机号","password":"...","name":""}`
结果：`{"account": Account}`；用户名/密码为空 → `bad_request`。

### 3.5 `accounts.remove` params `{"id":"138..."}`
结果：`{"removed":true}`；不存在 → `not_found`。

### 3.6 `settings.get` → `{"settings": Settings}`

### 3.7 `settings.set` params `{"settings":{...部分字段...}}`
只接受 `Settings` 白名单字段；越界/非法值 → `bad_request`，不得部分写入。
结果：`{"settings": Settings}`（写入后的完整值）。
写入前必须 `paths.backup_config()`；保留文件其他节（`[tiku]`/`[notification]`/`[cx]` 等）原样。

### 3.8 `courses.list` params `{"account_id":"138..."}`
流程：读账号密码 → 登录 → 拉课程列表 → 返回。不写任何课程完成状态。
结果：`{"account_id":"...","realname":"张三","courses":[Course]}`
失败：`login_failed` / `network_error`。

### 3.9 `run.start`
params：

```json
{
  "account_id": "138...",
  "course_ids": ["2151141", "189191"],
  "scope": {"chapters": true, "task_center": true, "only_discussion": false},
  "limits": {"max_points_per_course": 0, "max_tasks_per_course": 0},
  "options": {"speed": 1.0, "jobs": 2, "notopen_action": "continue",
               "retry_interval": 1.0, "serial_video": false,
               "add_learning_count": false, "target_count": 100,
               "work_redo_enabled": false, "work_max_retries": 3}
}
```

- `course_ids` 必须非空；`chapters` 与 `task_center` 不能同时 false。
- 缺省字段：前端可以只传变化的字段，Go 侧会用当前 `Settings` 补齐
  `options`（speed/jobs/notopen_action/retry_interval/serial_video/add_learning_count/
  target_count/work_redo_enabled/work_max_retries）与 `limits` 默认 0（全部）；桥侧仍做最终校验。
- 桥把本轮配置写到 `~/.chaoxing/accounts/run_<safe_username>.ini`（先备份全局配置），
  保留 `[tiku]`/`[notification]` 原值；`[common]` 按 params 覆盖。
- **交互安全**：GUI 不允许任何终端交互。桥写入的配置必须强制
  `task_center_submit_mode=auto`、`notopen_action` 只能是 `retry`/`continue`、
  `discussion_mode=task`、`use_cookies=false`；并给引擎传 `--yes`（跳过 CLI 启动确认，
  确认由 GUI 自己的弹窗完成）。
- 立即返回：`{"run_id":"r-20261104-211500","started_at":"2026-11-04 21:15:00"}`，
  随后异步发事件（见 §4）。运行中再次调用 → `busy`。

### 3.10 `run.stop`
结果：`{"stopping":true}`；桥调用 `api.interrupt.request_stop()`。
无运行中任务 → `not_found`。

### 3.11 `run.status`
结果：`{"active":true,"run_id":"...","started_at":"...","course_ids":[...]}`；
无任务时 `{"active":false,"run_id":"","started_at":"","course_ids":[]}`。

### 3.12 `reviews.list` params `{"days":1,"limit":50}`（`days=0` 表示全部，Go 侧默认放大到 500）
结果：`{"markdown_path":"/abs/.../2026-11-04.md","items":[ReviewSummary]}`

### 3.13 `reviews.get` params `{"submission_id":"..."}`
结果：`{"item": ReviewItem}`；不存在 → `not_found`。

## 4. 事件（Python 桥 → Go → 前端）

| event | data | 说明 |
|---|---|---|
| `run.started` | `{"run_id","started_at","account_id","username_masked","courses":[{"courseId","title"}],"scope":{...}}` | 事件流第一帧 |
| `run.log` | `{"run_id","level","text","ts"}` | 引擎日志/标准输出，每行一条；`level` ∈ trace/debug/info/warning/error |
| `run.stage` | `{"run_id","stage","text"}` | `stage` ∈ login/courses/chapters/task_center/review/done；粗粒度阶段，允许重复 |
| `run.progress` | `{"run_id","kind":"course","course_id","title","done","total"}` | 可选、尽力而为；前端没有也能工作 |
| `run.stopping` | `{"run_id"}` | 收到停止请求后 |
| `run.finished` | `{"run_id","ok","reason","duration_s","summary":{"courses":2,"errors":0},"message":""}` | 事件流最后一帧；`reason` ∈ completed/stopped/error |

- `run.finished.ok=false` 时 `message` 必须是一句人话（例如“登录失败：密码错误”）。
- 事件丢失可接受；前端以 `run.finished` 为唯一终态。Go 重启桥时向所有订阅者补发 `bridge:state`。

## 5. Wails 绑定（Go → 前端）

前端通过 `window.go.main.App.<Method>(...)` 调用，返回 Promise；错误以 reject 形式抛出
（Go 方法返回 `(T, error)` 时，error 会变成 JS 异常，`error.Error()` 为文案）。

| 方法 | 参数 | 返回 | 说明 |
|---|---|---|---|
| `GetStatus()` | — | `Status`（= §3.2 结果 + `engine_ready`/`bridge_state`） | 首页/设置页 |
| `ListAccounts()` | — | `Account[]` | |
| `AddAccount(username, password, name)` | string×3 | `Account` | |
| `RemoveAccount(id)` | string | `null` / error | |
| `GetSettings()` | — | `Settings` | |
| `SaveSettings(settings)` | `Settings` 部分字段 | `Settings` | |
| `ListCourses(accountID)` | string | `{realname, courses:[Course]}` | 网络请求，前端要有 loading/错误态 |
| `StartRun(opts)` | `RunOptions`（= §3.9 params） | `{run_id, started_at}` | 参数结构体字段名用 camelCase 或 json tag 对齐 §3.9；由 go-dev 定并在代码注释里写明 |
| `StopRun()` | — | `null` / error | |
| `GetRunState()` | — | `{active, run_id, started_at}` | 切换页面回来能恢复运行中状态 |
| `ListReviews(days)` | number | `{markdown_path, items}` | |
| `GetReview(submissionID)` | string | `ReviewItem` | |
| `ChooseEngineFolder()` | — | `string`（取消返回空串） | 打开目录选择器，选中后写设置 |
| `SetEnginePath(repoPath)` | string | `EngineStatus` | 校验 main.py/api 存在后保存 |
| `GetEngineStatus()` | — | `{repo, python, ready, message}` | Go 侧 Python/仓库探测结果 |
| `OpenDataDir()` | — | `null` | 用系统文件管理器打开 `~/.chaoxing` |
| `OpenConfigFile()` | — | `null` | 用默认编辑器打开 `~/.chaoxing/config.ini` |
| `GetAppInfo()` | — | `{version, protocol, license, platform}` | 关于页；`platform` = runtime.GOOS（additive，用于判断 darwin 透明材质） |
| `GetUpdateState()` | — | `UpdateState` | 取缓存的更新状态（不联网；启动自动检查通过事件刷新） |
| `StartBatch(ids, opts)` | `{account_ids, run_options}` | `BatchState` | 顺序运行多个账号（每账号自动拉全部课程） |
| `StopBatch()` | — | `BatchState` | 停止当前批量 |
| `GetBatchState()` | — | `BatchState` | 取批量队列快照（含每账号状态） |
| `CheckForUpdate()` | — | `UpdateState` | 联网检查 GitHub Releases（设置页「检查更新」） |
| `StartUpdate()` | — | `{started:boolean}` | 开始下载并安装更新；立即返回，进度走事件 |

### 事件（Go → 前端，`window.runtime.EventsOn`）

| 事件名 | payload | 说明 |
|---|---|---|
| `bridge:state` | `{"state":"starting|ready|restarting|failed|stopped","message":""}` | 桥生命周期；`stopped` = 应用退出/主动关闭 |

> 崩溃兜底：刷课进行中若 Python 桥异常退出且没有收到 `run.finished`，Go 必须补发
> `run:finished{ok:false,reason:"error",message:"Python 桥异常退出，本次任务已中断"}`，
> 前端收到后复位运行态（不再卡在“运行中”）。
| `run:started` | 同 `run.started` | |
| `run:log` | 同 `run.log` | |
| `run:stage` | 同 `run.stage` | |
| `run:progress` | 同 `run.progress` | |
| `run:stopping` | 同 `run.stopping` | |
| `run:finished` | 同 `run.finished` | 前端收到后必须停止计时、恢复按钮 |
| `update:state` | `UpdateState` | 更新状态变化（检查中 / 有新版本 / 失败 / 安装中 / 完成） |
| `update:progress` | `{phase, downloaded, total, percent, message}` | 下载与安装进度（低频）；phase ∈ download/verify/install/restart |
| `update:error` | `{message}` | 更新失败（同时 update:state 里带 error） |
| `batch:state` | `BatchState` | 批量队列变化（开始/当前账号/每账号终态/结束） |

### UpdateState（additive）

```json
{
  "supported": true, "current_version": "0.2.0",
  "available": false, "latest_version": "", "tag": "",
  "release_url": "", "notes": "", "published_at": "",
  "asset_name": "", "asset_size": 0, "digest": "",
  "checked_at": "", "checking": false, "installing": false,
  "progress": 0, "phase": "", "message": "", "error": ""
}
```

更新源：GitHub Releases（仓库 `songoao25/chaoxing`，tag 前缀 `desktop-v`，资源名
`chaoxing-desktop-macos-arm64.zip`）。安装前校验 asset sha256、bundle id、版本号与代码签名。

## 6. 前端页面清单（v1，固定）

1. **概览**：引擎状态条（未就绪→引导选目录/重试）、账号概览、今日复核数、最近复核预览、主按钮“开始刷课”。
2. **刷课**：账号选择 → 拉课程列表（多选/全选/搜索/刷新/loading/空态/错误态）→ 范围开关
   （章节 / 任务中心；讨论区 v1 不提供 GUI 入口）→ 参数（速度、并发、每门课上限、高级折叠）
   → 确认摘要 → 开刷。运行中：阶段胶囊、已用时间、停止按钮（二次确认）、日志控制台
   （级别过滤、自动滚动开关、复制）。结束：结果摘要 + 去复核。
3. **复核**：左侧列表（今天/近 3 天/全部过滤），右侧全文（可选中复制），状态说明，打开文件夹按钮。
4. **设置**：账号管理（新增弹层/删除确认）、常用参数、引擎（仓库路径、Python 状态、打开数据目录/配置文件）、关于。
5. 窗口：隐藏式标题栏（内容延伸到标题栏，左侧交通灯），侧边栏 + 内容区；最小 880×620，默认 1000×720。

## 7. 视觉规范（苹果设计规范落地）

- 字体：`-apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", sans-serif`；
  正文 13px/1.45，标题 17/22px 半粗；数字用等宽数字（`font-variant-numeric: tabular-nums`）。
- 颜色：语义变量 + 亮/暗两套（跟随系统 `prefers-color-scheme`）；强调色系统蓝 `#0A84FF`（暗）/`#007AFF`（亮）。
- 材质：侧边栏用半透明 + `backdrop-filter: blur(24px) saturate(180%)`；窗口背景亮 `#F5F5F7`、暗 `#1E1E1E`；
  卡片/列表用 elevation 表面色 + 1px hairline 边框（不用重阴影）。
- 圆角：控件 7px，卡片 10px，弹层 14px；行高 36px；控件高 28px（紧凑）/32px（常规）。
- 间距：4 的倍数（4/8/12/16/24/32）；页面左右留白 24px。
- 动效：120–200ms，`cubic-bezier(0.2, 0.8, 0.2, 1)`；只做 opacity/transform；尊重 `prefers-reduced-motion`。
- 图标：内联 SVG，线性 1.6px，16/18px；不引外链资源，全部离线可用。
- 文案：中文、短句、动词开头；错误态给“发生了什么 + 怎么办”。

## 8. 验证与 Mock

- `CX_DESKTOP_MOCK=1`：Go 返回内置假数据（账号/课程/复核/运行事件），用于无网络、无账号的 UI 联调与截图验收。
- `chaoxing-desktop --self-test`：不打开窗口，跑一遍 `bridge ping/status` 与引擎探测，输出 JSON 后退出（0 成功 / 1 失败）。
- Python 桥离线测试：`python -m unittest tests.test_desktop_bridge`（不得联网、不得读写真实 `~/.chaoxing`——用 `CX_DATA_HOME` 指向临时目录）。
- 前端可在浏览器用 `desktop/frontend/dist` 直接打开，配 `mock.js` 注入假 `window.go` / `window.runtime`（ui-dev 自备，生产不加载）。
