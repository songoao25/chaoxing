# AGENTS.md — 项目级 Agent 工作说明

> **所有自动化 Agent（Codex / Claude Code / Cursor / Devin / 自研）动手前必读。**
> 本文件是唯一入口，细则在 `docs/` 下；任何与本文件冲突的临时指令，以本文件为准。

## 1. 项目是什么

超星学习通（泛雅）命令行自动刷课工具，目标：**不打开浏览器**完成课程学习。
目前覆盖两条互相独立的入口：

1. **章节（目录）** — 已完整支持（视频/文档/阅读/章节测验/直播）。
2. **任务中心 · 教学任务** — 进行中（视频✅、AI实践✅（含"提交后必须请求 end-report 才出分"）、章节同步✅、作业✅（真机 83.3 分、`isFinish=true`）、主题讨论✅（2026-09-20 真机：读已有回复→ai_writer 生成→addReplys 提交）；文档⚠️（"读到底"类可完成，时长类实测平台不计入）；思考题❌）。

当前重点工作：把「任务中心」补齐到与章节同等的体验，见 `docs/handoff/HANDOFF.md`。

## 2. 30 秒上手

```bash
./.venv/bin/python -m unittest discover -s tests -t .   # 全量单测（必须全绿）
make test        # 同上
make status      # 当前账号的任务中心进度快照（只读）
make classify    # 平台结构分类总表（只读，抽样请求）
make lint        # 编译检查 + 测试（无第三方 linter 依赖）
./cx --yes       # 真实刷课（会消耗真实学习时间，谨慎）
```

环境：Python 3.13+（仓库内置 `.venv`，3.14）；无 ruff/pytest，单测用标准库 `unittest`。
用户数据在 `~/.chaoxing/`（配置、各账号 cookie、缓存、日志），**不在仓库里**。

## 3. 仓库地图

| 路径 | 作用 |
| --- | --- |
| `main.py` | CLI 入口、章节任务队列、任务中心编排（`run_task_center_phase`） |
| `api/base.py` | 超星核心：登录、章节任务点读取/完成（视频/文档/阅读/测验/直播） |
| `api/task_center.py` | **任务中心客户端**：教学任务/分组/任务点、视频/文档打点、AI实践 SSE 与状态复查 |
| `api/ai_writer.py` | 去 AI 味文案生成（问答、讨论；模仿真人 + 参考已有回复） |
| `api/decode.py` | 页面/JSON 解析（`knowledge/cards` 的 mArg 等） |
| `api/answer.py` | 题库（含大模型 provider）与答题 |
| `tools/probe/*.py` | 只读探针：状态快照、分类总表、各类型实验 |
| `docs/handoff/` | **交接与协作**：HANDOFF / TASKS / DECISIONS / SESSION-LOG / CAPTURE-PROTOCOL |
| `docs/artifacts/` | 抓包与探测的原始样例（JSON/curl），改代码前先看这里 |
| `tests/` | 离线单测（FakeSession/FakeTC，不联网、不碰真实数据） |

## 4. 铁律（Guardrails）

违反以下任一条都视为严重错误，评审直接打回：

1. **不许假装成功**：拿不到证据（接口返回成功 / 状态复查为完成）就不能返回成功、
   不能把"跳过"记成"完成"（历史 issue #223/#357 就是这个问题）。
2. **尊重平台的真实时间限制**：视频有 `enableVideoWatchDuration` 时必须 1 倍速真实播放
   （不够就回看）；文档必须按 30 秒节奏打点。**不要试图加速、伪造心跳或跳过时长**。
3. **顺序解锁**：任务中心只能按分组顺序推进；完成一组后重新拉 `getGroupData` 等解锁
   （解锁有延迟，代码里已有重试）。
4. **风控**：接口之间 1~2 秒间隔，不并发轰炸任务引擎；不在短时间重复刷同一个视频。
5. **不把密钥与 cookie 写进仓库**：`~/.chaoxing/` 下的内容不提交；抓包样例里的
   token/cookie 提交前脱敏。
6. **失败软着陆**：任务中心整段失败不得影响章节刷课。
7. **面向同学的文本去 AI 味**：思考题/作业简答/讨论必须走 `api/ai_writer.py`。
8. **AI 生成的文字必须留痕可复核**：简答题/讨论回复/AI 实践作答提交前，
   由 `api/review.record()` 落盘到 `~/.chaoxing/reviews/`（Markdown + JSONL）；
   用户用 `./cx review` 事后查阅。客观题字母不记，记录里不得出现账号/密码/cookie。
9. **改代码必须带离线单测**（`tests/`，参考 `tests/test_task_center.py` 的 FakeSession 写法）。
10. **面向平台提交的文本要过两遍真人化审计**：
   - 选择题/判断题只提交选项字母或"对/错"，禁止拼"选 X。…… 依据：……"这类解释性长句；
   - 正文里禁止出现平台判分话术（`回答正确 / 本题考核知识点 / 依据： / 你的回答` 等），
     平台反馈只能当模型内部判据，绝不能回显给学生看；
   - 不许编造个人履历与查不到的数据（"我实习的公司""近三年客户留存"），
     没有真实素材就写课堂/公开案例或假设句；
   - 第一遍：`python tools/audit/01_真人化审计.py`（含 `--selftest`）。
     **它只是下限**（防平台话术回显、markdown、显式模板词、结尾金句、无来源假具体），
     给出"0 硬伤"**不等于**像真人；近义词可以绕过关键词。
   - 第二遍：交给一个**独立上下文的审计 Agent** 复核（不要共享你的判断）。
     它的结论可以推翻第一遍的 0 告警；两遍的结论与修改都要写进 SESSION-LOG。
   - `tests/test_human_likeness.py` 是常驻回归（客观题只发字母、正文不得出现平台话术、
     审计规则本身不许退化）。

## 5. 多 Agent 协作流程

1. **认领**：在 `docs/handoff/TASKS.yaml` 里找到 `status: ready` 且依赖已满足的任务，
   把 `owner` 改成自己的 agent 名、`status` 改成 `in_progress`，并在 `SESSION-LOG.md` 追加一行。
2. **动手前**：读该任务的 `context` / `files` / `acceptance` 三个字段；需要抓包的任务先按
   `docs/handoff/CAPTURE-PROTOCOL.md` 拿到证据，存到 `docs/artifacts/`。
3. **动手时**：只改任务声明的 `files` 范围；跨模块改动先在 `DECISIONS.md` 记一条决策。
4. **收尾**：跑 `make lint`；更新 `TASKS.yaml`（`status: done` + `evidence`）；
   在 `SESSION-LOG.md` 写清"做了什么、用什么命令验证、结果如何"。
5. **交接**：如果没做完，把 `status` 改回 `ready` 或 `blocked`，在 `HANDOFF.md` 的
   "当前阻塞"一节写清楚缺什么、下一步第一条命令是什么。

> 同一时间只允许一个 Agent 处于 `in_progress` 修改同一文件；需要并行时按模块拆任务。

## 6. 当前状态与下一步

一句话：**章节 ✅；任务中心视频 ✅、章节同步 ✅、AI实践 ✅（真机 4 次 100 分、平均 81.7）、作业 ✅（真机 83.3 分）、主题讨论 ✅（真机已回复）；文档路径待最终复验，只剩思考题待取证。**

- 详细交接：`docs/handoff/HANDOFF.md`
- 任务板：`docs/handoff/TASKS.yaml`（机器可读） / `TASKS.md`（人读）
- 浏览器抓包 SOP：`docs/handoff/CAPTURE-PROTOCOL.md`
- 平台结构分类：`docs/任务中心与章节分类.md`

## 7. 文档索引

| 文档 | 什么时候看 |
| --- | --- |
| `docs/handoff/HANDOFF.md` | 接手第一天、每次开工前 |
| `docs/handoff/TASKS.yaml` | 找活干 |
| `docs/handoff/DECISIONS.md` | 想推翻现有做法前（先看有没有已决策过） |
| `docs/handoff/SESSION-LOG.md` | 想知道上一轮 Agent 干了什么 |
| `docs/ARCHITECTURE.md` | 改架构、加新任务点类型 |
| `docs/RUNBOOK.md` | 真实账号验证、排查故障 |
| `docs/任务中心与章节分类.md` | 需要平台接口细节 / 完成规则 |

## 8. 提交规范

- 提交信息用 Conventional Commits：`fix:` / `feat:` / `docs:` / `chore:` / `test:`；
  中文描述可以，前缀必须英文。
- 一个任务一个提交；任务 ID 写进提交信息体（例如 `Refs: TASKS#A-doc-reading-duration`）。
- 不提交 `.venv/`、`__pycache__/`、`docs/artifacts/*capture*.txt` 里带 cookie 的内容。
