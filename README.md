# Chaoxing Course Automation (CLI)

**English** | [中文](README.zh-CN.md)

[![CI](https://github.com/songoao25/chaoxing/actions/workflows/tests.yml/badge.svg)](https://github.com/songoao25/chaoxing/actions/workflows/tests.yml)
[![Release](https://img.shields.io/github/v/release/songoao25/chaoxing?include_prereleases)](https://github.com/songoao25/chaoxing/releases)
[![Last commit](https://img.shields.io/github/last-commit/songoao25/chaoxing)](https://github.com/songoao25/chaoxing/commits/main)
[![License](https://img.shields.io/github/license/songoao25/chaoxing)](LICENSE)

A command-line assistant that completes Chaoxing (学习通 / 泛雅) course tasks without opening a browser — chapters and the newer Task Center. It is a deep extension fork of [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing): the same CLI foundation, with **Task Center · Teaching Tasks** support added on top.

> **Status: chapters are stable; the Task Center is newer and partly verified.**
> Chapters (videos, documents, readings, chapter quizzes, live sessions) are complete.
> In the Task Center, videos, chapter sync, AI practice, homework and topic discussions have been verified on a real account.
> The **document** path is wired but the platform has not been observed counting it yet, and **thinking questions (思考题)** are not supported.
> A task is reported as complete only when the platform's own status re-check says so — the tool never pretends.

## Features

### Coverage

| Area | Status | What it covers |
| --- | :---: | --- |
| **Chapters (目录)** | ✅ | Every task point: video, document, reading, chapter quiz, live |
| **Task Center · Teaching Tasks (任务中心 · 教学任务)** | 🟡 | See the per-type table below |

| Task Center task type | Status | How it is handled |
| --- | :---: | --- |
| Video | ✅ | Plays through the task engine and reports on the real playback rhythm |
| Chapter sync | ✅ | Reuses chapter automation and calls the platform's chapter-score sync |
| AI practice | 🟡 | "Thinking ladder" (思维阶梯) is supported: `main-talk` SSE dialogue, then the end-report call that makes the platform score it. The newer **situational dialogue** (情景对话, `/mobile/situationalDialogue/*`) is a different API and is not adapted yet |
| Homework | ✅ | Multiple-choice / true-false / fill-in via the question bank; short answers via `api/ai_writer.py` |
| Topic discussion | ✅ | Reads existing replies for style, generates a non-duplicate reply, submits it |
| Document | ⚠️ | 30-second heartbeats are sent, and "finish reading" documents (readEnd) complete. For duration-only documents the platform was **not** observed counting even 600 s of heartbeats + readEnd (verified twice); the tool keeps them unfinished and retries at most once per 24 h |
| Thinking questions (思考题) | ❌ | Not supported — the tool tells you to finish it manually |
| Classroom activities / check-in | ❌ | Not supported — finish them manually |

### Rules the tool follows

- **Real time, never faked.** A video with a watch-duration requirement is played at 1× speed; if the recorded time falls short, the tool rewinds and replays. Documents are heart-beated on a 30-second cadence.
- **Unlock order.** Task Center groups unlock in order; the tool re-fetches group state after finishing a group instead of skipping ahead.
- **Honest completion.** Completion is only ever taken from a platform status re-check. Pressing submit is not a completed task.
- **Isolated accounts.** Cookies and configuration are stored per phone number under `~/.chaoxing/` — never inside the repository.
- **Optional notifications.** Bark, ServerChan, Telegram or Qmsg can push start / finish / interrupt / error messages.

## Quick start (no command-line experience needed)

### 1. Install Python 3.13 or newer

Download it from [python.org](https://www.python.org/downloads/) and run the installer.
On Windows, tick **"Add python.exe to PATH"** during installation.

### 2. Download the project and install its dependencies

```bash
git clone https://github.com/songoao25/chaoxing.git
cd chaoxing
python -m pip install -r requirements.txt
```

On macOS / Linux, use `python3` if `python` is not found.

### 3. Start the wizard

macOS / Linux:

```bash
./cx
```

Windows (`cx` is a Bash script, so run the wizard directly):

```powershell
python setup_wizard.py
```

The wizard walks you through one question at a time:

1. choose an account (add one the first time),
2. log in,
3. choose what to study — chapters, Task Center, or both,
4. choose the courses,
5. choose how many task points to do per course,
6. confirm and start.

Afterwards, run `./cx` again to reuse the saved setup. `cx setup` changes the answering mode, notifications or study options; `cx --yes` skips the final confirmation.

> **Time is real.** A course with hours of video takes hours. Leave it running in the background and let the optional notifications tell you when it finishes — see [Troubleshooting](#troubleshooting) for keeping it alive.

> **AI answering needs a DeepSeek account.** The recommended answering mode uses the DeepSeek API and may cost a small amount of money per course. You can also pick a question-bank service, answer manually, or skip quizzes entirely — see [Answering](#answering).

<details>
<summary>Other ways to run (for people who prefer configuration files or flags)</summary>

```bash
python main.py -c config.ini                  # use ~/.chaoxing/config.ini
python main.py -u <phone> -p <password> \
  -l <course-id-1>,<course-id-2>              # explicit account and courses
```

The wizard writes `~/.chaoxing/config.ini` for you; `config_template.ini` in the repository documents every option.

</details>

## Requirements

| Requirement | Notes |
| --- | --- |
| Python 3.13 or newer | The project targets 3.13+ and is tested on 3.13 / 3.14 |
| macOS, Linux or Windows | `./cx` needs a Bash shell; on Windows use `python setup_wizard.py` |
| Network access to Chaoxing | With an account you own |
| Dependencies | Installed by `pip install -r requirements.txt` (requests, beautifulsoup4, loguru, tqdm, openai, ddddocr, …) |
| Optional: DeepSeek API key | Only if you choose the AI answering mode |
| Optional: question-bank token | Only if you choose 言溪 / GO题 |
| Optional: notification service | Only if you want push messages |

## Usage

### Entry points

| Command | What it does |
| --- | --- |
| `./cx` | **Recommended.** Interactive wizard: user → login → what to study → courses → task points per course → confirm |
| `cx setup` | Reconfigure the answering mode or notifications |
| `cx --yes` | Same as `./cx`, but skips the final confirmation (for automation) |
| `python main.py -c config.ini` | Run directly from `~/.chaoxing/config.ini` |
| `python main.py -u <phone> -p <password> -l <ids>` | Run with an explicit account and courses. Avoid `-p` in a shared terminal — it lands in your shell history; the wizard or a config file is safer. |

### Answering

| Choice in the wizard | Config `provider` | Notes |
| --- | --- | --- |
| DeepSeek AI | `AI` | Recommended; answers most objective questions, needs a DeepSeek API key |
| 言溪 question bank | `TikuYanxi` | Needs a token from `tk.enncy.cn` |
| GO题 | `TikuGo` | Optional authorization |
| 言溪 + AI fallback | `TikuYanxi,AI` | More accurate, still needs the 言溪 token |
| GO题 + AI fallback | `TikuGo,AI` | More accurate |
| Manual | `TikuManual` | You type every answer; slowest |
| Do not answer | *(empty)* | Quizzes are skipped, which can block chapter unlocking |

- Chapter quizzes and Task Center homework (multiple-choice / true-false / fill-in) are answered through the configured provider in `api/answer.py`.
- Short-answer homework and topic-discussion replies are written by `api/ai_writer.py`, which de-AI-ifies the text and passes a two-pass humanization audit before submitting.
- Objective answers are submitted as option letters or 对 / 错 only — never as explanatory sentences.

### Submitting

| Mode | Behaviour |
| --- | --- |
| `auto` (default) | Submits in the background after answering — best for leaving it running |
| `confirm` | Shows a preview and asks before every submission |

Set it with `task_center_submit_mode = auto|confirm` in the config, or `--task-center-submit-mode confirm|auto` on the command line.
Either way, a task counts as complete only after the platform status re-check — never because the tool pressed submit.

### Notifications (optional)

| Provider | Config value | Delivery |
| --- | --- | --- |
| Bark | `Bark` | iPhone push |
| ServerChan | `ServerChan` | WeChat push |
| Telegram | `Telegram` | Telegram bot (also needs a `chat_id`) |
| Qmsg | `Qmsg` | QQ push |

Leave `[notification] provider` empty to disable. Messages are sent when a run starts, finishes, is interrupted, or errors.

### Multiple accounts

Each phone number gets its own credentials and cookies under `~/.chaoxing/accounts/`, so accounts never overwrite each other. The wizard lets you pick which account to run.

### Runtime data

```text
~/.chaoxing/
├── config.ini        # your settings (answer mode, notifications, study options)
├── accounts/         # per-phone credentials, cookies and last-used plan
├── cache.json        # question-bank answer cache
└── chaoxing.log      # run log
```

The directory is created with mode `0700`; credential files are written with mode `0600`.
Nothing is stored inside the repository. Set `CX_DATA_HOME` to move the whole directory.

### Common configuration keys

| Key | Default | Meaning |
| --- | --- | --- |
| `chapter_study` | `true` | Study the Chapters (目录) path |
| `task_center` | `true` | Study Task Center · Teaching Tasks after chapters |
| `max_points_per_course` | `0` | Max chapter task points per course (`0` = all) |
| `max_tasks_per_course` | `0` | Max teaching tasks per course (`0` = all) |
| `task_center_submit_mode` | `auto` | `auto` or `confirm` |
| `ai_practice_min_score` | `85` | Target score for AI practice |
| `ai_practice_max_rounds` | `5` | Maximum AI practice rounds |

Chapters and the Task Center cannot both be disabled — the program stops with a message instead of silently doing nothing.

## Privacy and security

**This project does not collect or upload any data.** There is no telemetry, no analytics, no developer server, and no account of any kind.

- **Everything stays on your machine.** Accounts, cookies and configuration are stored only under `~/.chaoxing/`, created with directory mode `0700` and credential files `0600`.
- **Nothing is committed.** Cookies, API keys, `config.ini` and logs must never be committed to the repository — they are also excluded by `.gitignore`.
- **Capture samples are sanitized.** Every packet-capture sample kept in `docs/artifacts/` has been stripped of cookies and tokens. If you add one, sanitize it first.
- **Network traffic goes where you expect.** Requests go to Chaoxing itself, plus only the third-party services you explicitly configure: the DeepSeek API (AI answering), the 言溪 / GO题 question banks, and the notification provider you choose. Nothing else.
- **Your account, your responsibility.** Use an account you own, and follow your school's and the platform's rules.
- **Login works like a browser.** The tool signs in with the credentials you provide and reads the login captcha with local OCR. It only ever uses the account you configured.

Found a security problem? Do not paste credentials or private course data into a public issue — see [SECURITY.md](SECURITY.md).

## Troubleshooting

| Symptom | What to do |
| --- | --- |
| **Login fails, or gets stuck on the captcha** | Check the phone number and password first; too many attempts can trigger a temporary lock on the platform. Re-run `./cx` to let the captcha OCR try again, or log in once in the web / App to clear the lock, then retry. |
| **A Task Center group never unlocks** | Groups unlock in order, and a group containing a task the tool cannot do (usually 思考题) blocks the ones behind it. Finish the blocking task manually in the browser or App, then re-run — the log says which task point is blocking. |
| **Thinking questions (思考题) were skipped** | Expected: they are not supported. The tool says so explicitly. Finish them manually; they are not counted as completed. |
| **Where is the log?** | `~/.chaoxing/chaoxing.log`. Attach a sanitized excerpt to an issue — never the whole file, which contains account identifiers. |
| **How do I stop it?** | Press `q` (macOS / Linux) or `Ctrl+C`. Work already reported to the platform is kept; re-running picks up from the platform's own progress. |
| **Can I close the terminal?** | No — closing it stops the process, and videos only progress while it runs. Use `tmux` / `screen` / `nohup` to keep it alive, or leave the window open. Notifications can tell you when it is done. |
| **Will re-running redo finished tasks?** | No. The tool reads the platform state and skips task points that are already complete. |
| **Cookies expired / "login expired"** | Run `./cx` again and log in; the per-account cookie file is refreshed automatically. |
| **A Task Center document task stays incomplete** | Known gap (⚠️): 30-second heartbeats and the final readEnd are sent, but for duration-only documents the platform was not observed counting them (verified twice, 600 s each). The tool never marks it complete; it also skips re-reading the same document for 24 h. |
| **AI answering fails or the key is rejected** | Check the DeepSeek key and balance. If a fallback chain is configured (for example `TikuYanxi,AI`), the tool tries the next provider; otherwise the quiz is skipped. |
| **`pip install` fails on `lxml` / `ddddocr`** | Use Python 3.13 or 3.14 in a fresh virtual environment: `python -m venv .venv && .venv/bin/pip install -r requirements.txt` (Windows: `.venv\Scripts\pip`). |
| **`./cx: Permission denied`** | Make it executable once: `chmod +x cx`. On Windows, run `python setup_wizard.py` instead. |

## Development

```bash
make test      # offline unit tests (245 tests, no network, no real account)
make lint      # compile check + tests — run this before every commit
make status    # read-only Task Center progress snapshot
make doctor    # environment self-check
make help      # list all targets
```

- **No ruff, no pytest.** Tests use the standard library `unittest` and run fully offline against fakes (`FakeSession`, `FakeTC`) — they never touch a real account.
- **CI** — [.github/workflows/tests.yml](.github/workflows/tests.yml) runs on every push and pull request with Python 3.13: install requirements, `compileall`, then `unittest discover`.
- **Changing behaviour?** Add an offline test under `tests/` in the same pull request, and follow the guardrails in [AGENTS.md](AGENTS.md).
- **Current status and handoff notes** — [docs/handoff/HANDOFF.md](docs/handoff/HANDOFF.md).

### Repository layout

| Path | Purpose |
| --- | --- |
| `cx` | Recommended launcher (`./cx`, `cx setup`, `cx --yes`) |
| `setup_wizard.py` | Interactive setup and study wizard |
| `main.py` | CLI entry, chapter task queue and Task Center orchestration |
| `api/base.py` | Chaoxing core: login and chapter task points |
| `api/task_center.py` | Task Center client: groups, task points, video / document reporting |
| `api/answer.py` | Question banks (including AI providers) and answering |
| `api/ai_writer.py` | Human-like writing for homework and discussions |
| `tools/probe/` | Read-only probes: progress snapshots, classification tables |
| `tests/` | Offline unit tests |
| `docs/` | Handoff notes, runbook, architecture, capture samples |

## Repository policy

- Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `test:`, `chore:`. A Chinese description is fine; the prefix must be English.
- Run `make lint` before opening a pull request.
- Never commit cookies, API keys, passwords, `config.ini`, `accounts/`, or logs.
- Sanitize any capture sample before adding it to `docs/artifacts/`.
- Do not report a task as complete without platform evidence, and do not fake playback time — see [AGENTS.md](AGENTS.md) for the full guardrails.
- Keep public documentation user-facing, and label unverified behaviour as unverified.

## References

- [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing) — the upstream project this fork extends.
- [Keep a Changelog](https://keepachangelog.com/) — the format used by [CHANGELOG.md](CHANGELOG.md).
- [Contributor Covenant](https://www.contributor-covenant.org/) — the basis of [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- [Conventional Commits](https://www.conventionalcommits.org/) — the commit message convention.

## License

[GPL-3.0](LICENSE). The original project and its copyright belong to [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing) and its contributors; this fork keeps that attribution and publishes its own changes under the same license.

**Disclaimer**

- This project is for **learning and exchange only**.
- **Commercial or for-profit use is prohibited.**
- Any derivative work must also be released under GPL-3.0, with the upstream attribution preserved.
- Users are responsible for their own use of this code; the authors and contributors accept no liability.
