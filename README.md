# Chaoxing Course Automation (CLI)

**English** | [Chinese](README.zh-CN.md)

[![CI](https://github.com/songoao25/chaoxing/actions/workflows/tests.yml/badge.svg)](https://github.com/songoao25/chaoxing/actions/workflows/tests.yml)
[![Release](https://img.shields.io/github/v/release/songoao25/chaoxing?include_prereleases)](https://github.com/songoao25/chaoxing/releases)
[![Last commit](https://img.shields.io/github/last-commit/songoao25/chaoxing)](https://github.com/songoao25/chaoxing/commits/main)
[![License](https://img.shields.io/github/license/songoao25/chaoxing)](LICENSE)

A command-line assistant that finishes Chaoxing (Xuexitong / Fanya) course tasks without opening a browser — chapters and the newer Task Center. It is a deep extension fork of [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing): the same CLI foundation, with **Task Center · Teaching Tasks** support added on top.

> **Status: chapters are stable; the Task Center is newer and partly verified.**
> Chapters (videos, documents, readings, chapter quizzes, live sessions) are complete.
> In the Task Center, videos, chapter sync, AI practice, homework and topic discussions have been verified on a real account.
> The **document** path is wired but the platform has not been observed counting it yet, and **thinking questions** are not supported.
> A task is reported as complete only when the platform's own status re-check says so — the tool never pretends.
>
> Around the brushing itself: a **pre-run scan** always shows what is still missing, every AI-written answer is **printed live and kept for review** (`./cx review`), and discussions can be brushed automatically or **picked by hand from the board** (`./cx discuss`).

> Console samples in this document are translated from the Chinese UI that the CLI actually prints.

## Features

### Coverage

| Area | Status | What it covers |
| --- | :---: | --- |
| **Chapters (table of contents)** | ✅ | Every task point: video, document, reading, chapter quiz, live |
| **Task Center · Teaching Tasks** | 🟡 | See the per-type table below |

| Task Center task type | Status | How it is handled |
| --- | :---: | --- |
| Video | ✅ | Plays through the task engine and reports on the real playback rhythm |
| Chapter sync | ✅ | Reuses chapter automation and calls the platform's chapter-score sync |
| AI practice | 🟡 | The "thinking ladder" type is supported: dialogue over `main-talk` SSE, then the end-report call that makes the platform score it. The newer **situational dialogue** type (`/mobile/situationalDialogue/*`) is a different API and is not adapted yet |
| Homework | ✅ | Multiple-choice / true-false / fill-in via the question bank; short answers via `api/ai_writer.py` |
| Topic discussion | ✅ | Reads existing replies for style, generates a non-duplicate reply, submits it |
| Document | ⚠️ | 30-second heartbeats are sent, and "finish reading" documents complete. For duration-only documents the platform was **not** observed counting even 600 s of heartbeats plus the final readEnd call (verified twice); the tool keeps them unfinished and retries at most once per 24 h |
| Thinking questions | ❌ | Not supported — the tool tells you to finish them manually |
| Classroom activities / check-in | ❌ | Not supported — finish them manually |

### Comfort features around the brushing

| Feature | What it does |
| --- | --- |
| **Pre-run scan** | Before every run (no checkbox, always on) it prints what is left: chapters pending, teaching tasks done / pending / locked / unsupported, plus reminders for missing homework and discussions. Read-only; a scan failure never blocks the run |
| **Live answer trace** | While answering, each question is printed as it happens (`1. choice  A`, `4. short answer  ... (41 chars)`); discussion replies are shown in full. The same lines go to the normal run log |
| **Review (`./cx review`)** | Every AI-written text that was submitted (quiz short answers, homework essays, discussion replies, AI-practice answers) is kept under `~/.chaoxing/reviews/` and can be browsed later |
| **Two discussion modes** | Mode 1: task discussions, brushed automatically in the course's required order. Mode 2: `./cx discuss` lists the board, you pick threads (`1,3,5` / `1-3` / `all`), each one shows a draft and asks `y/n` before sending |
| **Safe defaults** | Parallel tasks default to 2 (fewer captchas); `--yes` never auto-sends discussion replies; Enter on a dangerous prompt means "no"; piped or non-interactive input can never hang the wizard |

### Rules the tool follows

- **Real time, never faked.** A video with a watch-duration requirement is played at 1× speed; if the recorded time falls short, the tool rewinds and replays. Documents are heart-beated on a 30-second cadence.
- **Unlock order.** Task Center groups unlock in order; the tool re-fetches group state after finishing a group instead of skipping ahead.
- **Honest completion.** Completion is only ever taken from a platform status re-check. Pressing submit is not a completed task.
- **Isolated accounts.** Cookies and configuration are stored per phone number under `~/.chaoxing/` — never inside the repository.
- **Optional notifications.** Bark, ServerChan, Telegram or Qmsg can push start / finish / interrupt / error messages.

### The whole flow at a glance

```text
start -> what to brush -> how to brush discussions -> pick courses -> how many per course -> confirm
      -> startup checks -> login -> pre-run scan -> chapters -> Task Center -> discussion board (if chosen)
      -> AI content trace hint -> result summary
```

Each step only asks what the previous choice needs, and every stage prints what it is about to do — nothing is asked twice and nothing is silently skipped.

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
3. choose what to study — chapters, Task Center, both, or discussions only,
4. choose how to brush discussions — task discussions automatically, or pick threads on the board,
5. choose the courses,
6. choose how many task points to do per course (skipped for "discussions only"),
7. confirm and start.

Afterwards, run `./cx` again to reuse the saved setup. `cx setup` changes the answering mode or notifications; `cx --yes` skips the final confirmation.

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
| Dependencies | Installed by `pip install -r requirements.txt` (requests, beautifulsoup4, loguru, tqdm, openai, ddddocr, ...) |
| Optional: DeepSeek API key | Only if you choose the AI answering mode |
| Optional: question-bank token | Only if you choose a question-bank service |
| Optional: notification service | Only if you want push messages |

## Usage

### Entry points

| Command | What it does |
| --- | --- |
| `./cx` | **Recommended.** Interactive wizard: user → login → what to study → courses → task points per course → confirm |
| `cx setup` | Reconfigure the answering mode or notifications |
| `cx discuss` | Browse the course discussion board and reply to the threads you pick (`--list-topics` lists only) |
| `cx review` | Review the AI-written text that was submitted (`--days N` / `--all` / `--list`) |
| `cx --yes` | Same as `./cx`, but skips the final confirmation (for automation) |
| `python main.py -c config.ini` | Run directly from `~/.chaoxing/config.ini` |
| `python main.py -u <phone> -p <password> -l <ids>` | Run with an explicit account and courses. Avoid `-p` in a shared terminal — it lands in your shell history; the wizard or a config file is safer. |

### What the numbers mean

A number is **how many *unfinished* task points to do in this run** — already-finished ones are always skipped and never re-done.

> Example: a course has 300 sections and the first 150 are already complete. Typing `150` means "do the next 150 unfinished points" (sections 151–300). It does **not** mean "only look at the first 150".

`all` (or just Enter) means "finish everything that is still unfinished". Chapters and Task Center tasks are counted separately.

### Pre-run scan (always on)

Before every run the tool scans what is still missing — no checkbox, no extra prompt:

```text
  Pre-run scan
  ----------------------------------------------
  Example Course
    Chapters   139 sections - 37 done - 102 left, resuming at "1.1 Course intro"
    Tasks      9 teaching tasks - 77 task points (62 done - 9 pending - 6 locked)
    Pending    homework 1 - topic discussion 1 - video 2 - document 3 - AI practice 2
    Note       - 1 homework not done (this run only brushes discussions; choose "Task Center" to brush it)
    Note       - 1 topic discussion not done (this run picks threads on the board instead)
  ----------------------------------------------
```

It only reads; nothing is submitted during the scan, and a scan failure never blocks the run.

### Discussions: two modes

| Mode | Entry | What it does |
| --- | --- | --- |
| 1. Task discussions (automatic) | included in scope 1/3, or scope 4 / `--only-discussion` | Walks the Task Center discussion points in the course's required order, reads the existing replies, writes one ordinary reply and submits it |
| 2. Discussion board (you pick) | wizard scope 4 → "discussion board", or `./cx discuss` | Reads the board first (how many threads), you pick which ones (`1,3,5` / `1-3` / `all`), then each one shows a draft, asks `y/n`, and is sent one by one; `./cx discuss --list-topics` only lists |

Both modes share the same reply pipeline: read existing replies → de-AI-ified text → humanization audit → submit → live trace and a record under `~/.chaoxing/reviews/`. A thread you already replied to is never replied to twice.

```text
  Discussion board - Example Course
  ----------------------------------------------
  Reading threads...
  20 threads - threads you already replied to are skipped

  Page 1 - threads 1-20

    1. [1 reply] Zhang San - 2 hours ago
       Does an opportunity in the external environment mean the company will succeed?
    2. [0 replies] Li Si - 7 hours ago
       What is the difference between substitute threats and existing rivalry?

  > Pick threads to reply to (e.g. 1,3,5 / 1-3 / all; n next page - q quit) 1

  [1/1] Does an opportunity in the external environment mean the company will succeed?
      --------------------------------------------
      An opportunity is only an external condition; whether it can be seized still
      depends on the company's own resources and capabilities.
      --------------------------------------------
      Send this reply? [y send - Enter skip - q quit] y
      sent

  Done: 1 sent - 0 skipped
```

The wizard asks only what your choice needs: chapters-only asks about chapter task points, Task Center asks about teaching tasks, and "discussions only" asks nothing about counts — it asks how to brush discussions instead.

### Study scope

| Choice | What runs |
| --- | --- |
| 1. Chapters + Task Center | Both entries (recommended) |
| 2. Chapters only | Chapters only |
| 3. Task Center only | Task Center teaching tasks only |
| 4. Discussions only | Only discussions; everything else is skipped this run. Also available as `--only-discussion` |

Topic discussions are part of the Task Center (they are one task-point type, not a separate entry on the platform), so options 1 and 3 include them as well.

### Answering

| Choice in the wizard | Config `provider` | Notes |
| --- | --- | --- |
| DeepSeek AI | `AI` | Recommended; answers most objective questions, needs a DeepSeek API key |
| Question bank | `TikuYanxi` | Needs a token from the provider |
| Question bank (GO) | `TikuGo` | Optional authorization |
| Question bank + AI fallback | `TikuYanxi,AI` | More accurate, still needs the question-bank token |
| Question bank (GO) + AI fallback | `TikuGo,AI` | More accurate |
| Manual | `TikuManual` | You type every answer; slowest |
| Do not answer | *(empty)* | Quizzes are skipped, which can block chapter unlocking |

- Chapter quizzes and Task Center homework (multiple-choice / true-false / fill-in) are answered through the configured provider in `api/answer.py`.
- Short-answer homework and topic-discussion replies are written by `api/ai_writer.py`, which de-AI-ifies the text and passes a two-pass humanization audit before submitting.
- Objective answers are submitted as option letters or true/false only — never as explanatory sentences.

### Submitting

| Mode | Behaviour |
| --- | --- |
| `auto` (default) | Submits in the background after answering — best for leaving it running |
| `confirm` | Shows a preview and asks before every submission |

Set it with `task_center_submit_mode = auto|confirm` in the config, or `--task-center-submit-mode confirm|auto` on the command line.
Either way, a task counts as complete only after the platform status re-check — never because the tool pressed submit.

### Reviewing AI-written content

The tool never hides what it wrote on your behalf. Every piece of substantive text that an AI produced and that was submitted to the platform — chapter-quiz short answers, homework essays, topic-discussion replies, AI-practice answers — is printed live during the run and kept in a readable log:

```text
~/.chaoxing/reviews/2026-09-21.md     # one Markdown file per day
~/.chaoxing/reviews/index.jsonl       # index used by the review command
```

Answers are also shown **live while the run is going** — one line per question:

```text
  Answering - Homework task A (4 questions)
     1. choice        Which of these belongs to ...            A
     2. multiple      Which items cover ...                    ABC
     3. true/false    Diversification always beats focus       true
     4. short answer  Which strategy would you pick           I would keep the main business solid ... (41 chars)
  Discussion - Discussion task A
    The example from class stuck with me; when the environment changes fast, an advantage can disappear quickly. ...
```

The same lines go into the normal run log (`~/.chaoxing/chaoxing.log`), so nothing extra needs to be generated.

Review it at any time:

| Command | What it shows |
| --- | --- |
| `./cx review` | Today's items, then type a number to read the full text |
| `./cx review --days 7` | The last 7 days |
| `./cx review --all` | Everything ever recorded |
| `./cx review --list` | Just the list (no interaction) |

Each run ends with a one-line hint when new content was recorded. Objective answers (A/B/C letters, true/false) are not recorded — they have nothing to review. The log never contains passwords, cookies or tokens.

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
  config.ini        # your settings (answer mode, notifications)
  accounts/         # per-phone credentials, cookies and run configs
  cache.json        # answer cache
  reviews/          # AI-written text kept for review
  submissions.json  # local ledger: avoid duplicate homework submissions
  chaoxing.log      # run log (DEBUG by default; CX_LOG_LEVEL=TRACE for full detail)
```

Nothing in this directory is ever committed to the repository.

### Common configuration keys

| Key | Default | Meaning |
| --- | --- | --- |
| `chapter_study` | `true` | Brush chapters |
| `task_center` | `true` | Brush Task Center teaching tasks |
| `only_discussion` | `false` | Only brush discussions |
| `discussion_mode` | `task` | `task` = automatic task discussions; `board` = pick threads on the discussion board |
| `jobs` | `2` | How many task points to process in parallel |
| `speed` | `2` | Video speed (watch-duration videos always run at 1×) |
| `max_points_per_course` | *(empty)* | How many unfinished chapter task points per course |
| `max_tasks_per_course` | *(empty)* | How many unfinished teaching tasks per course |
| `task_center_submit_mode` | `auto` | `auto` or `confirm` |
| `serial_video` | `false` | Process videos one at a time (useful if the platform rolls progress back) |
| `ai_practice_min_score` | `85` | Target score for AI practice |
| `ai_practice_max_rounds` | `5` | How many times AI practice may be retried |

## Privacy and security

- **No data leaves your machine.** The tool talks only to Chaoxing and to the answering provider you configure; there is no telemetry and no upload of course content.
- **Credentials stay local.** Accounts, cookies, logs and review records live in `~/.chaoxing/` (directory `0700`, files `0600`) and are never part of the repository.
- **Sanitized samples.** Packet-capture samples under `docs/artifacts/` have account numbers, user ids, course parameters and tokens replaced with placeholders.
- **Do not commit secrets.** Never commit cookies, API keys, tokens or logs. `.gitignore` protects `tools/probe/local.env` and `*.har`.
- **Captcha.** Login captchas are recognised locally with OCR on your own machine; nothing is sent to a third-party service.
- **Your responsibility.** Use it only with an account you own, and follow your school's rules.

## Troubleshooting

| Problem | What to do |
| --- | --- |
| **Login captcha keeps failing** | Run `./cx` again; if it still fails, log in once in a browser and retry. The tool backs off for 60 seconds after repeated failures instead of hammering the platform |
| **A Task Center group never unlocks** | Groups unlock in order and the platform syncs with a delay; re-run later. The pre-run scan shows how many points are still locked |
| **Thinking questions are skipped** | Not supported — the tool prints a clear message telling you to finish them manually |
| **A Task Center document task stays incomplete** | Known gap: 30-second heartbeats and the final readEnd are sent, but for duration-only documents the platform was not observed counting them (verified twice, 600 s each). The tool never marks it complete; it also skips re-reading the same document for 24 h |
| **AI practice scores below the pass line** | The platform grades the practice itself. The tool answers with a reasoning model and majority voting, but a low score can still happen; it retries within the configured rounds |
| **AI answering fails or the key is rejected** | Check the DeepSeek key and balance. If a fallback chain is configured (for example `TikuYanxi,AI`), the tool tries the next provider; otherwise the quiz is skipped |
| **Where is the log?** | `~/.chaoxing/chaoxing.log`. When filing an issue, attach only a sanitized excerpt — never the whole file, it contains account identifiers |
| **How do I stop it?** | Press `q` (macOS / Linux) or `Ctrl+C`. Work already reported to the platform is kept; re-running picks up from the platform's own progress |
| **Can I close the terminal?** | No — closing it stops the process, and videos only progress while it runs. Use `tmux` / `screen` / `nohup` to keep it alive, or leave the window open. Notifications can tell you when it is done |
| **Will re-running redo finished tasks?** | No. The tool reads the platform state and skips task points that are already complete |
| **Cookies expired / "login expired"** | Run `./cx` again and log in; the per-account cookie file is refreshed automatically |
| **`pip install` fails on `lxml` / `ddddocr`** | Use Python 3.13 or 3.14 in a fresh virtual environment: `python -m venv .venv && .venv/bin/pip install -r requirements.txt` (Windows: `.venv\Scripts\pip`) |
| **`./cx: Permission denied`** | Make it executable once: `chmod +x cx`. On Windows, run `python setup_wizard.py` instead |

## Development

```bash
make test      # offline unit tests (335 tests, no network, no real account)
make lint      # compile check + tests — run this before every commit
make test-313  # same suite on the CI version (3.13); local Python may be 3.14
               # (annotations are evaluated lazily there, so CI can differ)
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
| `cx` | Recommended launcher (`./cx`, `cx setup`, `cx --yes`, `cx discuss`, `cx review`) |
| `setup_wizard.py` | Interactive setup and study wizard |
| `main.py` | CLI entry, chapter task queue and Task Center orchestration |
| `api/base.py` | Chaoxing core: login and chapter task points |
| `api/task_center.py` | Task Center client: groups, task points, video / document reporting |
| `api/discussion.py` | Discussion board: thread list, board resolution, pick-and-reply |
| `api/scan.py` | Pre-run scan: what is still missing, in one block |
| `api/review.py` | Review log for AI-written text (`cx review`) |
| `api/answer.py` | Question banks (including AI providers) and answering |
| `api/ai_writer.py` | Human-like writing for homework and discussions |
| `tools/probe/` | Read-only probes: progress snapshots, classification tables |
| `tests/` | Offline unit tests |
| `docs/` | Handoff notes, runbook, architecture, capture samples |

## Repository policy

- Issues and pull requests are welcome; please keep the scope of a change small and focused.
- Behaviour changes must come with an offline test and must respect the guardrails in [AGENTS.md](AGENTS.md): never fake success, respect real time limits, keep unlock order, do not hammer the platform.
- Never commit credentials, cookies, tokens or personal data. Samples must be sanitized.
- This is a fork: upstream attribution is kept, and improvements that belong upstream are welcome there too.

## References

- Upstream project: [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing)
- Platform structure notes: [docs/architecture-notes.md](docs/PLATFORM-NOTES.md)
- Handoff notes and decisions: [docs/handoff/](docs/handoff/HANDOFF.md)

## License

GPL-3.0 — see [LICENSE](LICENSE).

> This project is for learning and personal use only. Do not use it for commercial purposes, and do not use it on accounts you do not own. You are responsible for complying with your school's rules and the platform's terms of service.

