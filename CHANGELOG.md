# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the version numbers follow [Semantic Versioning](https://semver.org/).
This branch is an extension fork of [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing); earlier history belongs to the upstream project.

## [Unreleased]

### Added

- Discussion mode 2: `./cx discuss` lists every thread of the course board and lets you pick which post to reply to (`--list-topics` lists only). Both discussion modes share the same reply pipeline.
- Pre-run scan (always on, read-only): before every run the tool reports what is still unfinished — chapters, teaching tasks, locked groups, unsupported types, and missing homework and discussions.
- "Discussions only" study scope (wizard option 4 / `--only-discussion`): only discussions are processed in this run.
- Live answer trace: every question is printed as it is answered (`1. choice  A`, `4. short answer  ... (41 chars)`) and written to the normal run log; discussion replies are shown in full.
- Review step for AI-written content: every submitted short answer, homework essay, discussion reply and AI-practice answer is logged to `~/.chaoxing/reviews/` (daily Markdown + JSONL index) and can be browsed with `./cx review` (`--days N`, `--all`, `--list`).
- Local submission ledger (`~/.chaoxing/submissions.json`) to avoid duplicate homework submissions and repeated document reading.
- `api/llm.py`: adaptive thinking policy (auto / on / off) with automatic fallback and reasoning fallback.
- `api/scan.py` (pre-run scan) and `api/discussion.py` (discussion board).
- README (English + Chinese), CONTRIBUTING, SECURITY, SUPPORT, CODE_OF_CONDUCT, issue forms and Dependabot.
- Probe configuration via `tools/probe/local.env` (gitignored) instead of hard-coded accounts.

### Changed

- The wizard asks only what the chosen scope needs ("discussions only" no longer asks about task counts); a new step picks between automatic task discussions and picking threads on the board.
- Discussion-board flow rebuilt: read-only count → pick threads (`1,3,5` / `1-3` / `all`) → per-thread draft → `y/n` confirmation → send one by one.
- All y/n prompts use lowercase `[y/n]` consistently.
- The number you type clearly means "how many unfinished task points to do" (finished ones are always skipped); the wizard and README say so explicitly.
- Console cleanup: one startup header instead of duplicated lines, condensed 403/captcha messages, quieter notification hints, and a one-line summary for failures.
- Default parallel tasks lowered from 4 to 2 (fewer captcha/403 triggers); existing configs are migrated once.
- Log noise reduction: the q-key hint prints once per run, chapter progress lines fit ~80 columns, AI-writer retries and per-question details are debug-only, long videos and documents print one progress line.
- Log file defaults to DEBUG; set `CX_LOG_LEVEL=TRACE` for full request-level tracing.
- AI answering: thinking defaults to `auto` (V4.1 flash reasons by default), objective questions use 3-sample majority voting, prompts ask for option letters, and answer parsing accepts JSON, code fences or plain text.
- Live sessions run in real time (1x), honour `q`/Ctrl+C, and return failure when a heartbeat fails.
- Document tasks: "finish reading" documents complete; duration-only documents are reported unfinished (the platform was not observed counting 600 s + readEnd) and are not re-read for 24 h.
- Wizard: Enter on the final confirmation cancels instead of starting the run.
- Tests always use a temporary `CX_DATA_HOME`.

### Fixed

- Discussion board no longer reports success when nothing was sent (it returns real `sent`/`skipped` counts to the console and to notifications).
- "Discussions only" no longer prints "Task Center only" wording; the scope, its explanation and the confirmation page each appear once.
- `--yes` no longer auto-sends discussion replies (posting AI text publicly always asks per post).
- "Discussions only" was blocked by the startup check because the run config wrote `task_center = false`; the Task Center path now always stays enabled for it.
- `skip_discussion` was referenced but never assigned, so the whole Task Center phase failed with a NameError (reported as "read failed").
- The answer-mode table was unpacked in the wrong order: question-bank modes asked for a DeepSeek key, and choosing "no quizzes" at the key step discarded the just-entered token.
- Task Center phase interrupted by `q` no longer reports "all done".
- Chapter quizzes without a question bank are no longer recorded as completed.
- Unknown card types are no longer silently dropped (they used to make a chapter look like an empty, completed one).
- Chapter documents are no longer treated as successful on HTTP 200 alone.
- `tqdm.format_sizeof` global patch is always restored.
- Removed the dead `--auto-sign` flag, dead `app.py`, and unused `celery` / `flask` / `argparse` / `chardet` dependencies.
- Repository-wide privacy scrub: accounts, user ids, course parameters, tokens, real names and `__pycache__` removed from the working tree.

### Security

- All packet-capture samples are sanitized; `.gitignore` protects `tools/probe/local.env` and `*.har`.

### Known issues

- Task Center documents: heartbeats are wired, but the platform has not been observed counting duration-only documents (⚠️).
- Task Center thinking questions are not supported and must be finished manually (❌); classroom activities and check-in are not supported either.
- Task Center groups unlock in order: an unsupported task point blocks every later group.

## [3.1.3] - 2026-09-20

### Added

- Multiple accounts: cookies and configuration are isolated per phone number under `~/.chaoxing/` (directory `0700`, files `0600`).
- Answering modes: DeepSeek AI, a question bank, GO, manual, or no answering at all, with fallback chains.
- Optional notifications: Bark, ServerChan, Telegram or Qmsg, pushed on start, finish, interrupt and error.
- Chapters (table of contents): every task point — video, document, reading, chapter quiz, live.

### Changed

- Task Center submit mode defaults to `auto` (submits in the background); set it to `confirm` to approve every submission.
- Brushing parameters are no longer asked one by one; startup prints a single recommended-config line.

### Fixed

- Multi-choice letter strings were missing options, option images were lost, and score detection was unreliable.
- Chapter parsing and mArg parsing hardened; video reporting got a retry cap instead of hanging.
- Command-line mode now reads the question-bank settings from the user configuration.

### Notes

- This version recommends `./cx` as the entry point; `cx setup` reconfigures, `cx --yes` skips the confirmation.
- Offline unit tests use the standard library `unittest` (335 tests); CI is defined in [`.github/workflows/tests.yml`](.github/workflows/tests.yml).
