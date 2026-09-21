# 超星学习通自动刷课（命令行版）

[English](README.md) | **中文**

[![CI](https://github.com/songoao25/chaoxing/actions/workflows/tests.yml/badge.svg)](https://github.com/songoao25/chaoxing/actions/workflows/tests.yml)
[![Release](https://img.shields.io/github/v/release/songoao25/chaoxing?include_prereleases)](https://github.com/songoao25/chaoxing/releases)
[![Last commit](https://img.shields.io/github/last-commit/songoao25/chaoxing)](https://github.com/songoao25/chaoxing/commits/main)
[![许可证](https://img.shields.io/github/license/songoao25/chaoxing)](LICENSE)

一个命令行助手：不打开浏览器就能完成超星学习通（泛雅）的课程任务——章节，以及新版的任务中心。本项目是 [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing) 的深度扩展分支：沿用同一套命令行底座，在其上补全了**任务中心 · 教学任务**。

> **项目状态：章节稳定；任务中心较新，且部分链路尚未最终验证。**
> 章节（视频、文档、阅读、章节测验、直播）已完整支持。
> 任务中心的视频、章节同步、AI 实践、作业、主题讨论已用真实账号验收通过。
> **文档**链路已经接通：需要"读到底"（readEnd）的文档能完成；只要求阅读时长的文档实测打点 600 秒 + readEnd 后平台仍不计入（两次验证），工具不会假装完成，同一文档 24 小时内不重复消耗阅读时间。**思考题**不支持。
> 只有平台自己的状态复查返回「完成」，程序才会把它记为完成——绝不假装。

## 功能

### 覆盖范围

| 入口 | 状态 | 说明 |
| --- | :---: | --- |
| **章节（目录）** | ✅ | 全部任务点：视频、文档、阅读、章节测验、直播 |
| **任务中心 · 教学任务** | 🟡 | 见下方分类型表格 |

| 任务中心任务点 | 状态 | 处理方式 |
| --- | :---: | --- |
| 视频 | ✅ | 走任务引擎播放器的上报接口，按真实播放节奏打点 |
| 章节同步 | ✅ | 复用章节刷课，并调用平台的章节成绩同步 |
| AI 实践 | 🟡 | 已支持"思维阶梯"：走 `main-talk` SSE 对话，提交后再请求 end-report 让平台现算成绩；新版**情景对话**（`/mobile/situationalDialogue/*`）是另一套接口，暂未适配 |
| 作业 | ✅ | 选择 / 判断 / 填空走题库；简答题走 `api/ai_writer.py` |
| 主题讨论 | ✅ | 读已有回复做风格参考，生成一条不重复的回复并提交 |
| 文档 | ⚠️ | 已按 30 秒节奏打点；"读到底"（readEnd）类文档能完成，只要求时长的文档实测 600 秒打点 + readEnd 后平台仍不计入（两次验证）。不假装完成，同一文档 24 小时内不重复读 |
| 思考题 | ❌ | 不支持——程序会明确提示你手动完成 |
| 课堂活动 / 签到 | ❌ | 不支持——请手动完成 |

### 程序遵守的规则

- **真实时间，绝不伪造。** 有观看时长要求的视频按 1 倍速真实播放；时长不够就回看补足。文档按 30 秒节奏打点。
- **顺序解锁。** 任务中心的分组按顺序解锁；完成一组后程序会重新拉取分组状态，而不是跳着往下刷。
- **诚实完成。** 完成状态只认平台的状态复查；「提交了」不等于「完成了」。
- **账号隔离。** cookie 与配置按手机号分开存放在 `~/.chaoxing/`，绝不写进仓库。
- **可选通知。** Bark / ServerChan / Telegram / Qmsg 可在开始、完成、中断、出错时推送消息。

## 快速开始（零基础可用）

### 1. 安装 Python 3.13 或更高版本

从 [python.org](https://www.python.org/downloads/) 下载安装包并安装。
Windows 用户安装时请勾选 **“Add python.exe to PATH”**。

### 2. 下载项目并安装依赖

```bash
git clone https://github.com/songoao25/chaoxing.git
cd chaoxing
python -m pip install -r requirements.txt
```

macOS / Linux 上如果找不到 `python`，请改用 `python3`。

### 3. 启动向导

macOS / Linux：

```bash
./cx
```

Windows（`cx` 是 Bash 脚本，直接运行向导即可）：

```powershell
python setup_wizard.py
```

向导会一次只问一个问题：

1. 选用户（第一次使用时新增一个），
2. 登录，
3. 选刷什么——章节、任务中心，或两者都刷，
4. 选课程，
5. 每门课刷多少个任务点，
6. 确认并开始。

之后再次运行 `./cx` 即可沿用已保存的配置。`cx setup` 用来修改答题方式或通知；`cx --yes` 跳过最后的确认。

> **时间是真实的。** 几小时的视频就要跑几小时。建议挂在后台运行，让可选通知在完成时告诉你——保持运行的方法见[故障排查](#故障排查)。

> **AI 答题需要 DeepSeek 账号。** 推荐的答题方式使用 DeepSeek API，每门课可能产生少量费用。也可以选择题库、手动答题，或完全不答题——见[答题](#答题)。

<details>
<summary>其它运行方式（更习惯配置文件或命令行的用户）</summary>

```bash
python main.py -c config.ini                  # 使用 ~/.chaoxing/config.ini
python main.py -u <手机号> -p <密码> \
  -l <课程ID1>,<课程ID2>                       # 显式指定账号和课程
```

向导会自动写好 `~/.chaoxing/config.ini`；仓库里的 `config_template.ini` 记录了所有可配置项。

</details>

## 前置要求

| 要求 | 说明 |
| --- | --- |
| Python 3.13 或更高 | 项目面向 3.13+，已在 3.13 / 3.14 上验证 |
| macOS / Linux / Windows | `./cx` 需要 Bash；Windows 用 `python setup_wizard.py` |
| 能访问学习通 | 使用你自己的账号 |
| 依赖 | 由 `pip install -r requirements.txt` 安装（requests、beautifulsoup4、loguru、tqdm、openai、ddddocr 等） |
| 可选：DeepSeek API Key | 仅在选择 AI 答题时需要 |
| 可选：题库 token | 仅在选择言溪 / GO题时需要 |
| 可选：通知服务 | 仅在想收推送时需要 |

## 使用方法

### 入口

| 命令 | 作用 |
| --- | --- |
| `./cx` | **推荐。** 交互式向导：选用户 → 登录 → 选刷什么 → 选课 → 每门刷多少 → 确认 |
| `cx setup` | 重新配置答题方式或通知 |
| `cx --yes` | 与 `./cx` 相同，但跳过最后的人工确认（自动化用） |
| `python main.py -c config.ini` | 直接读取 `~/.chaoxing/config.ini` 运行 |
| `python main.py -u <手机号> -p <密码> -l <课程ID>` | 用显式账号、课程和参数运行 |

### 答题

| 向导中的选项 | 配置 `provider` | 说明 |
| --- | --- | --- |
| DeepSeek AI | `AI` | 推荐；什么题都能答，需要 DeepSeek API Key |
| 言溪题库 | `TikuYanxi` | 需要 `tk.enncy.cn` 的 token |
| GO 题 | `TikuGo` | 可选 authorization |
| 言溪 + AI 兜底 | `TikuYanxi,AI` | 更准，仍需言溪 token |
| GO题 + AI 兜底 | `TikuGo,AI` | 更准 |
| 手动答题 | `TikuManual` | 每题手动输入，最慢 |
| 不答题 | *（留空）* | 跳过测验，可能卡住章节解锁 |

- 章节测验与任务中心作业的选择 / 判断 / 填空题，都通过 `api/answer.py` 里配置的 provider 作答。
- 作业简答题与主题讨论回复由 `api/ai_writer.py` 生成：先去 AI 味，再过两遍真人化审计，才允许提交。
- 客观题只提交选项字母或「对 / 错」，绝不提交解释性长句。

### 提交模式

| 模式 | 行为 |
| --- | --- |
| `auto`（默认） | 作答完成后后台自动提交——适合挂着跑 |
| `confirm` | 每次提交前显示预览并询问 |

在配置里设置 `task_center_submit_mode = auto|confirm`，或在命令行用 `--task-center-submit-mode confirm|auto` 临时覆盖。
无论哪种模式，都只在平台状态复查为完成后才算完成——不会把「点了提交」当成完成。

### 通知（可选）

| 服务 | 配置值 | 推送渠道 |
| --- | --- | --- |
| Bark | `Bark` | iPhone 推送 |
| ServerChan | `ServerChan` | 微信推送 |
| Telegram | `Telegram` | Telegram Bot（还需填 `chat_id`） |
| Qmsg | `Qmsg` | QQ 推送 |

`[notification] provider` 留空即关闭。程序会在开始、完成、中断或出错时推送。

### 多账号

每个手机号都有独立的账号信息与 cookie，存放在 `~/.chaoxing/accounts/` 下，互不覆盖。向导会让你选择这次用哪个账号。

### 运行数据

```text
~/.chaoxing/
├── config.ini        # 你的配置（答题方式、通知、刷课选项）
├── accounts/         # 按手机号隔离的账号、cookie 与上次选课方案
├── cache.json        # 题库答案缓存
└── chaoxing.log      # 运行日志
```

数据目录以 `0700` 权限创建；账号、cookie 等凭据文件以 `0600` 权限写入。
仓库里不会保存任何用户数据。可用环境变量 `CX_DATA_HOME` 整体迁移数据目录。

### 常用配置项

| 配置项 | 默认值 | 含义 |
| --- | --- | --- |
| `chapter_study` | `true` | 是否刷「章节（目录）」 |
| `task_center` | `true` | 章节之后是否继续刷任务中心 · 教学任务 |
| `max_points_per_course` | `0` | 每门课最多刷几个章节任务点（`0` = 全部） |
| `max_tasks_per_course` | `0` | 每门课最多刷几个教学任务（`0` = 全部） |
| `task_center_submit_mode` | `auto` | `auto` 或 `confirm` |
| `ai_practice_min_score` | `85` | AI 实践的目标分数 |
| `ai_practice_max_rounds` | `5` | AI 实践最多补答轮数 |

章节与任务中心不能同时关闭——都关掉时程序会直接停止并提示，而不是静默什么都不做。

## 隐私与安全

**本项目不收集、不上传任何数据。** 没有遥测、没有统计、没有开发者服务器，也没有任何账号体系。

- **一切都在你本机。** 账号、cookie 与配置只存放在 `~/.chaoxing/`，目录权限 `0700`，凭据文件权限 `0600`。
- **绝不入库。** cookie、API Key、`config.ini`、日志都不允许提交到仓库，`.gitignore` 也已排除。
- **抓包样例均已脱敏。** 仓库 `docs/artifacts/` 中保留的抓包样例都已去掉 cookie 与 token；如需新增，请先脱敏。
- **网络请求去向明确。** 请求只发往学习通本身，以及你显式配置的第三方服务：DeepSeek API（AI 答题）、言溪 / GO题 题库、你选择的通知服务。除此之外没有别的。
- **用你自己的账号，并对使用负责。** 请遵守学校和平台的规定。
- **登录过程和浏览器一样。** 程序用你提供的凭据登录，并用本机 OCR 识别登录验证码；只会使用你自己配置的账号。

发现安全问题？请不要把凭据或私有课程数据贴到公开 Issue——见 [SECURITY.md](SECURITY.md)。

## 故障排查

| 现象 | 怎么办 |
| --- | --- |
| **登录失败，或卡在验证码** | 先检查手机号和密码；短时间尝试过多可能触发平台临时锁定。重新运行 `./cx` 让验证码识别再试一次，或先在网页 / App 登录一次解锁，再回来重试。 |
| **任务中心某个分组一直不解锁** | 分组是顺序解锁的；某一组里如果有程序做不了的任务点（通常是思考题），后面的组都会被卡住。请在浏览器或 App 里手动完成那个任务点，然后重新运行——日志会写明是哪个任务点卡住。 |
| **思考题被跳过了** | 这是预期行为：不支持思考题，程序会明确提示。请手动完成；它不会被记为完成。 |
| **日志在哪里？** | `~/.chaoxing/chaoxing.log`。提 Issue 时请只附脱敏后的片段——不要上传整个文件，里面含账号标识。 |
| **怎么停止？** | 按 `q`（macOS / Linux）或 `Ctrl+C`。已经上报给平台的进度会保留，重新运行会按平台进度继续。 |
| **可以关掉终端吗？** | 不可以——关掉终端进程就停了，而视频只有在运行期间才会推进。请用 `tmux` / `screen` / `nohup` 保持运行，或让窗口开着。可选通知会在完成时告诉你。 |
| **重新运行会不会把做过的再做一遍？** | 不会。程序会读取平台状态，跳过已经完成的任务点。 |
| **cookie 失效 / 提示登录过期** | 重新运行 `./cx` 登录即可，对应账号的 cookie 文件会自动刷新。 |
| **任务中心的文档任务一直不完成** | 已知缺口（⚠️）：30 秒打点与 readEnd 都已发出，但只要求时长的文档平台未计入（两次各 600 秒验证）。工具不会把它记成完成，并且同一文档 24 小时内不会重复消耗阅读时间。 |
| **AI 答题失败或提示 Key 无效** | 检查 DeepSeek Key 与余额。如果配置了兜底链（例如 `TikuYanxi,AI`），程序会尝试下一个 provider；否则该测验会被跳过。 |
| **`pip install` 在 `lxml` / `ddddocr` 上失败** | 用 Python 3.13 或 3.14 建一个干净的虚拟环境：`python -m venv .venv && .venv/bin/pip install -r requirements.txt`（Windows 为 `.venv\Scripts\pip`）。 |
| **`./cx: Permission denied`** | 执行一次 `chmod +x cx` 即可。Windows 用户请改用 `python setup_wizard.py`。 |

## 开发

```bash
make test      # 离线单测（当前 245 项，不联网、不碰真实账号）
make lint      # 编译检查 + 单测——提交前必跑
make status    # 只读的任务中心进度快照
make doctor    # 环境自检
make help      # 列出所有命令
```

- **没有 ruff，也没有 pytest。** 单测使用标准库 `unittest`，基于 `FakeSession` / `FakeTC` 完全离线运行，绝不碰真实账号。
- **CI** —— [.github/workflows/tests.yml](.github/workflows/tests.yml) 在每次 push 和 PR 时用 Python 3.13 执行：安装依赖、`compileall`、`unittest discover`。
- **改行为？** 在同一个 PR 里给 `tests/` 补上离线单测，并遵守 [AGENTS.md](AGENTS.md) 里的铁律。
- **当前状态与交接** —— [docs/handoff/HANDOFF.md](docs/handoff/HANDOFF.md)。

### 仓库结构

| 路径 | 作用 |
| --- | --- |
| `cx` | 推荐入口（`./cx`、`cx setup`、`cx --yes`） |
| `setup_wizard.py` | 交互式配置与刷课向导 |
| `main.py` | CLI 入口、章节任务队列与任务中心编排 |
| `api/base.py` | 超星核心：登录、章节任务点 |
| `api/task_center.py` | 任务中心客户端：分组、任务点、视频 / 文档打点 |
| `api/answer.py` | 题库（含大模型 provider）与答题 |
| `api/ai_writer.py` | 作业与讨论的去 AI 味文案生成 |
| `tools/probe/` | 只读探针：进度快照、分类总表 |
| `tests/` | 离线单测 |
| `docs/` | 交接文档、Runbook、架构说明、抓包样例 |

## 仓库规范

- 使用 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)：`feat:`、`fix:`、`docs:`、`test:`、`chore:`。描述可用中文，前缀必须是英文。
- 提交 PR 前先跑 `make lint`。
- 绝不提交 cookie、API Key、密码、`config.ini`、`accounts/` 或日志。
- 往 `docs/artifacts/` 添加抓包样例前必须先脱敏。
- 没有平台证据不许把任务记为完成，也不许伪造播放时长——完整铁律见 [AGENTS.md](AGENTS.md)。
- 对外文档面向用户，未经验证的行为必须如实标注。

## 参考项目

- [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing) —— 本分支扩展的上游项目。
- [Keep a Changelog](https://keepachangelog.com/zh-CN/) —— [CHANGELOG.md](CHANGELOG.md) 使用的格式。
- [Contributor Covenant](https://www.contributor-covenant.org/) —— [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) 的基础。
- [Conventional Commits](https://www.conventionalcommits.org/zh-hans/) —— 提交信息规范。

## 许可证

[GPL-3.0](LICENSE)。原始项目与版权归 [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing) 及其贡献者所有；本分支保留该出处，并以同一许可证发布自己的改动。

**免责声明**

- 本项目**仅供学习交流**。
- **禁止商业或盈利用途。**
- 任何衍生作品都必须同样以 GPL-3.0 发布，并保留上游出处。
- 使用者需自行承担使用本代码的一切责任；作者与贡献者不承担任何责任。
