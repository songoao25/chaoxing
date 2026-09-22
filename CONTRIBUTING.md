# Contributing

Thanks for considering a contribution! Please read this guide before opening an issue or a pull request.

## How to contribute

### Reporting a bug

- Search the [issues](https://github.com/songoao25/chaoxing/issues) first — it may already be reported.
- Use the bug report form and include: steps to reproduce, expected behaviour, actual behaviour, and your environment.
- Sanitize log excerpts. **Never** paste cookies, account numbers, API keys or a full log file.

### Suggesting a feature

- Describe the use case in an issue first so we can agree on the scope.
- Implement it after the discussion.

### Sending code

1. Fork the repository and create a branch: `git checkout -b feature/xxx`
2. Follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `test:`, `chore:`.
3. Commit messages may be written in English or Chinese — just make the change clear.
4. Open a pull request and fill in the template (what changed, how you verified it).

### Test before you push

```bash
make lint      # compile check + full offline unit tests (335 tests)
make test-313  # the same suite on the CI version (3.13)
```

Behaviour changes must come with an offline test under `tests/` (see the `FakeSession` style in `tests/test_task_center.py`). Tests never touch the network or a real account.

## Hard rules

These exist because we have been burned before. Reviewers and CI check them:

- **Never fake success.** Without platform evidence (an accepted request or a completed status re-check) the tool must not report success, and a "skipped" task must never be recorded as "done".
- **Respect real time limits.** A video with a watch-duration requirement must play at 1x for real (replay if short); documents must be reported on a 30-second cadence. No speeding up, no forged heartbeats, no skipping durations.
- **Unlock order.** Task Center groups only unlock in order; re-fetch group state after finishing a group.
- **Rate limits.** Keep 1-2 seconds between requests; do not hammer the task engine.
- **No secrets in the repository.** Nothing from `~/.chaoxing/`, and no token or cookie from capture samples. Sanitize any new sample before committing.
- **Human-like student text.** Homework essays, discussions and thinking questions must go through `api/ai_writer.py` and pass the two-pass humanization audit (`python tools/audit/01_human_likeness_audit.py` plus an independent review).
- **Soft landing.** A failure in the Task Center phase must never break chapter brushing.

The full list lives in [`AGENTS.md`](AGENTS.md).

## Development environment

- Python 3.13+; no ruff and no pytest — tests use the standard library `unittest`.

| Command | Purpose |
| --- | --- |
| `make test` | Offline unit tests |
| `make lint` | Compile check + tests (run before every commit) |
| `make test-313` | The same suite on the CI version (3.13) |
| `make status` | Read-only Task Center progress snapshot |
| `make doctor` | Environment self-check |

User data lives in `~/.chaoxing/` (configuration, cookies, cache, logs) and is never part of the repository.

## Merging and releases

- External pull requests are reviewed and merged by the maintainer.
- After merging into `main`, the maintainer decides the version number and cuts the release.
- Release notes live in [CHANGELOG.md](CHANGELOG.md).

## Code of conduct

Please follow the [Code of Conduct](CODE_OF_CONDUCT.md). Participating in this project means you agree to it.

## License

Contributions are licensed under the same [GPL-3.0 license](LICENSE) as this project, keeping the upstream attribution to [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing).
