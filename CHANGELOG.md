# Changelog

本项目的所有重要变更都记录在此文件。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。
本分支是 [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing) 的扩展分支；更早的历史属于上游项目。

## [Unreleased]

### Added
- Live answer trace: every question is printed as it is answered (`1. 选择  A`, `4. 简答  …（41 字）`) and written to the normal run log; discussion replies are shown in full.
- Review step for AI-written content: every submitted short answer, homework essay, discussion reply and AI-practice answer is logged to `~/.chaoxing/reviews/` (daily Markdown + JSONL index) and can be browsed with `./cx review` (`--days N`, `--all`, `--list`).
- Local submission ledger (`~/.chaoxing/submissions.json`) to avoid duplicate homework submissions and repeated document reading.
- `api/llm.py`: adaptive thinking policy (auto / on / off) with automatic fallback and reasoning fallback.
- README (English + 中文), CONTRIBUTING, SECURITY, SUPPORT, CODE_OF_CONDUCT, issue forms, dependabot.
- Probe configuration via `tools/probe/local.env` (gitignored) instead of hard-coded accounts.

### Changed
- Console cleanup: one startup header instead of duplicated lines, condensed 403/captcha messages, quieter notification hints, and a one-line summary for failures.
- Default parallel tasks lowered from 4 to 2 (fewer captcha/403 triggers); existing configs are migrated once.
- Console/log noise reduction: the q-key hint prints once per run, chapter progress lines fit ~80 columns, AI-writer retries and per-question details are debug-only, long videos/documents print one progress line, and failures collapse into a single summary line.
- Log file defaults to DEBUG; set `CX_LOG_LEVEL=TRACE` for full request-level tracing.
- AI answering: thinking defaults to `auto` (V4.1 flash reasons by default), objective questions use 3-sample majority voting, prompts ask for option letters, and answer parsing accepts JSON / code fences / plain text.
- Live sessions now run in real time (1x), honour `q`/Ctrl+C, and return failure when a heartbeat fails.
- Document tasks: "finish reading" documents complete; duration-only documents are reported unfinished (platform was not observed counting 600 s + readEnd) and are not re-read for 24 h.
- Wizard: Enter on the final confirmation cancels instead of starting the run.
- Tests always use a temporary `CX_DATA_HOME`.

### Fixed
- New AI-practice subtype "situational dialogue" (`/mobile/situationalDialogue/*`) is detected and reported as unsupported instead of a confusing "missing parameters" warning.
- Task Center phase interrupted by `q` no longer reports "all done".
- Chapter quizzes without a question bank are no longer recorded as completed.
- `tqdm.format_sizeof` global patch is always restored.
- Removed dead `--auto-sign` flag, dead `app.py`, and unused `celery` / `flask` / `argparse` / `chardet` dependencies.
- Repository-wide privacy scrub: accounts, uids, course parameters, tokens and `__pycache__` removed from the working tree.

### Security
- All packet-capture samples are sanitized; `.gitignore` protects `tools/probe/local.env` and `*.har`.

### Added

- 任务中心 · 教学任务：视频（含真实观看时长规则）、章节同步、AI 实践、作业、主题讨论。
- `./cx` 交互式向导：登录后显式选择「刷什么内容」，并逐门选择章节任务点 / 教学任务数量。
- 任务中心作业：选择题 / 判断题 / 填空题走题库，简答题走 `api/ai_writer.py`。
- 主题讨论：读取已有回复做风格参考，生成一条不重复的回复并提交。
- AI 实践：提交后请求 end-report 让平台现算成绩；支持目标分与补答轮数。

### Changed

- 任务中心提交模式默认 `auto`（后台自动提交），可配置为 `confirm` 逐次确认。
- 刷课参数不再逐项追问，启动只显示一行推荐配置。

### Fixed

- 多选题字母串漏选、选项图片丢失、成绩判定不可信等问题。
- 目录课程解析与 mArg 解析加固；视频上报卡死增加重报上限。
- 命令行模式读取用户配置里的题库设置。

### Known issues

- 任务中心「文档」：打点已接入，但平台计账尚未被观察到（⚠️）。
- 任务中心「思考题」：不支持，需手动完成（❌）；课堂活动 / 签到同样不支持。
- 任务中心按分组顺序解锁：被不支持的任务点卡住时，后续分组不会解锁。

## [3.1.3] - 2026-09-20

### Added

- 多账号：cookie 与配置按手机号隔离在 `~/.chaoxing/`（目录 `0700`、文件 `0600`）。
- 答题方式：DeepSeek AI / 言溪题库 / GO题 / 手动答题 / 不答题，支持多题库兜底链。
- 可选通知：Bark / ServerChan / Telegram / Qmsg，在开始、完成、中断、出错时推送。
- 章节（目录）全部任务点：视频、文档、阅读、章节测验、直播。

### Notes

- 本版本以 `./cx` 为推荐入口；`cx setup` 重新配置，`cx --yes` 跳过确认。
- 离线单测使用标准库 `unittest`（当前 245 项），CI 见 [`.github/workflows/tests.yml`](.github/workflows/tests.yml)。
