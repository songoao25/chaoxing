# 任务板（人读版）

> 机器可读版在 `TASKS.yaml`（认领/更新以 YAML 为准）。状态：🟢 ready / 🟡 in_progress / 🔴 blocked / ✅ done

| ID | 标题 | 优先级 | 状态 | 依赖 |
| --- | --- | --- | --- | --- |
| `cap-doc-ac-mark` | 【抓包】文档 readPoint（ac_mark） | P0 | ✅ | — |
| `cap-ai-practice` | 【抓包】AI实践 main-talk 与 submit | P0 | 🟡 | — |
| `cap-autopull` | 【抓包】章节同步 autoPullChapterScore | P0 | ✅ | — |
| `A-doc-reading-duration` | 打通文档阅读时长（2/5/10 分钟） | P0 | 🔴 | cap-doc-ac-mark |
| `B-chapter-plan-sync` | 章节类任务点完成后同步（已并入刷课流程） | P0 | ✅ | cap-autopull |
| `B2-doc-chapter-sync-verify` | 章节同步的文档路径真机验收 | P1 | 🟢 | — |
| `D0-human-like-history` | 跨篇"骨架指纹"治理（历史去重/换骨架） | P1 | 🟢 | — |
| `E-ai-practice-high-score` | AI实践重答刷到 80~90 分 | P0 | ✅ | cap-ai-practice |
| `C-homework` | 任务中心作业自动作答（真机 83.3 分、isFinish=true） | P1 | ✅ | B |
| `G-task-count-range` | 任务中心「刷前 N 个教学任务」选择 | P0 | ✅ | — |
| `D-discussion-and-question` | 思考题自动作答（主题讨论已完成） | P1 | 🟢 | B |
| `cap-homework` | 【抓包】作业取题与提交 | P1 | 🔴 | B |
| `cap-discussion` | 【抓包】讨论列表与发帖（2026-09-20 真机回复成功） | P1 | ✅ | B |
| `H-docs-registry` | 维护交接文档与任务板 | P2 | ✅ | — |
| `F-wizard-study-scope` | 向导显式选择刷课范围（章节 / 任务中心） | P0 | ✅ | — |

## 建议执行顺序

1. `cap-autopull` 已 ✅（`docs/artifacts/capture_autopull.txt`）：`stuJobInfo` 是平台下发的，CLI 侧已接入并被引擎接受。
2. `B-chapter-plan-sync` 已 ✅：5639750 与 5639751 两个章节任务点都由 CLI 真机跑通，
   第1章 `0.3333 → 0.5 → 0.6667`（4/6），第 3 组（作业 + 文档）解锁；
   剩下的文档路径真机验收拆成了 `B2-doc-chapter-sync-verify`。
3. mooc 视频打点在部分网络下会被客户端指纹拦截（requests 403 / curl 200），CLI 已加 curl 回退；
   视频进度已到结尾但未通过时会从头回看（D13）。
4. 文档 A 暂停继续消耗真实时长，等分组的文档任务点可学时再复验。
5. `E-ai-practice-high-score` 已 ✅（2026-09-17 真机）：AI实践一次练习 = 把所有知识点答对，
   提交后必须请求一次 `think/end-report` 才会现算成绩（不请求会一直"未评估"）。
   开学第一课实测 4 次练习各 100 分、练习平均分 81.7、平台成绩字段 `answerScore=100`。
6. 等第1章推进后分组解锁，再抓 `cap-homework`、`cap-discussion`，实现 `C`、`D`。

## 每个任务的完整信息

`context / files / steps / acceptance / evidence` 都在 `TASKS.yaml` 里，认领前务必读。
