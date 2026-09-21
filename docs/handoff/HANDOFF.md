# HANDOFF — 学习通「任务中心 / 教学任务」适配

| 字段 | 值 |
| --- | --- |
| 状态 | 🟡 进行中（章节 ✅ / 任务中心 视频✅ 文档⚠️ 章节✅ AI实践✅ 作业✅ 主题讨论✅） |
| 更新 | 2026-09-20 |
| 上一轮 Agent | dsh（DeepSeek Harness；作业与主题讨论真机打通、控制台降噪、默认自动提交） |
| 接手对象 | 具备浏览器操控能力的 Agent，或人类开发者 |
| 下一步（唯一入口） | 读 `docs/handoff/TASKS.yaml` → 思考题（第5章解锁后）与文档时长最终复验 |
| 验证命令 | `make lint`（编译 + 当前 245 项离线单测） |

---

## 1. 一分钟摘要

- **目标**：命令行（不打开浏览器）把课程「任务中心 → 教学任务」完整刷完，含视频、文档、章节、
  作业、思考题、主题讨论、AI实践；问答/讨论用大模型生成并**去 AI 味**；AI实践评分刷到 80~90。
- **已经能用**：章节全部任务点；任务中心的视频（含"真实观看时长"规则）；分组顺序解锁编排；
  任务中心读取/状态复查；**AI实践（思维阶梯）刷分**（默认提交前确认）；去 AI 味文案生成器。
- **本轮新增（2026-09-20）**：
  ① 向导登录后显式选择「刷什么内容」，并逐门选择"章节前 N 个 / 教学任务前 N 个"（D26/D27）；
  ② **作业任务点打通**：新版学习页 + `addStudentWorkNewWeb`，真机提交第1章作业
     （4 多选 + 2 填空）平台判 **83.3 分**、`isFinish=true`（D28，取证 `docs/artifacts/capture_homework.txt`）；
  ③ 日志归因：章节任务点/视频完成补 INFO，视频/文档失败补 WARNING；
     任务点第 0 页无 mArg 才 WARNING，第 1~6 页不再逐页打日志（每章汇总一条 DEBUG）；
     ④ 控制台降噪 + 默认自动提交：任务中心阶段也静音（内部日志只进文件）；
     提交模式默认 auto，作业/AI实践/讨论不再逐条确认；刷课参数不再追问，启动只显示一行推荐配置（D29/D30）；
     ⑥ **审计修复与 AI 答题质量**：DeepSeek thinking 改为 auto（V4.1 flash 默认带推理，正确率更高）、
     客观题 3 次采样投票、提示词改问选项字母；修复直播假成功/不可中断、无题库测验记完成、
     任务中心终止误报"全部完成"、确认页回车即开刷、tqdm 全局补丁泄漏、作业重复提交（本地台账）；
     隐私全仓脱敏（账号/uid/课程参数/取证样例）、README 与仓库规范文件重写（见 D34-D36）。
     ⑤ **主题讨论任务点打通**：groupweb 话题页 → 已有回复风格参考 → ai_writer 生成 →
     addReplys 提交；真机已回复并读回，文本过两遍真人化审计（D31，取证 `docs/artifacts/capture_discussion.txt`）。
- **上轮新增（2026-09-17）**：
  ① 章节同步链路打通并真机验收（`stuJobInfo` → `autoPullChapterScore`，第1章 0.3333 → 0.6667）；
  ② **AI实践刷分打通**：一次练习 = 逐个知识点答一遍，9 题全对就是 100 分；
  扣分的关键接口是 `POST /ai-ans/ai-evaluate/think/end-report` —— 提交**不会**出分，
  必须请求一次"学习质量评估报告"成绩才现算出来。开学第一课实测 4 次练习各 100 分、
  练习平均分 81.7、平台成绩字段 `answerScore=100`。
- **当前卡点**：文档时长类任务点平台仍不计入（2026-09-21 两次各 600 秒打点 + readEnd 验证）；
  思考题任务点在锁定分组、尚未取证；主题讨论/作业/AI实践已全部真机验收。
  主题讨论已完整验收：回复成功 → 约 3 分钟后引擎 `isFinish` 翻转（22:28 false → 22:31 true）。
- **当前路线**：文档路径真机复验（`B2-doc-chapter-sync-verify`）；第1章第 3 组解锁后抓作业/讨论
  （`cap-homework` / `cap-discussion`）。

---

## 2. 接手第一步

```bash
cd /path/to/chaoxing
make doctor          # 环境自检
    make lint            # 确认当前代码是绿的（当前 234 项离线单测）
make status          # 看账号当前进度（只读，会用到 173****6569 的 cookie；账号已脱敏）
```

当前基线（2026-09-20，账号 173****6569 / 企业战略管理；来自 `tools/probe/02` 只读快照 + 本轮真机提交）：

| 教学任务 | 进度 | 备注 |
| --- | --- | --- |
| 开学第一课 | 3/3 ✅ | 含 AI实践：4 次练习各 **100 分**，练习平均分 **81.7**，平台成绩字段 `answerScore=100`，`已达标` |
| 第1章 企业战略管理概述 | 5/6 | 两个章节任务点由 CLI 同步成功；**作业由 CLI 自动作答提交，平台判 83.3 分、isFinish=true**；剩 1 个文档任务点 |
| 第2章 企业愿景、使命、目标 | 4/6 | 任务中心 21 秒视频 + 章节视频已完成；作业已解锁可自动作答（本次未提交）；剩主题讨论 |
| 第3章 企业外部环境分析 | 2/15 | 第 1 组章节可学；作业/讨论在锁定分组 |
| 第4章 企业内部环境分析 | 0/14 | 第 1 组视频可学；文档 10 分钟 |
| 第5章 企业总体战略 | 1/13 | 思考题已完成；作业在锁定分组 |
| 第7章 企业国际化战略 | 0/9 | 文档 10 分钟 |

课程参数（示例课程，均已脱敏）：`courseId=<courseId> / clazzId=<clazzId> / cpi=<cpi> / fid=<fid> / uid=<uid>`。

---

## 3. 已验证事实（都有实测证据，不要重新猜）

| # | 结论 | 证据 |
| --- | --- | --- |
| 1 | 章节与任务中心是**两套独立入口**，记录不互通 | 任务中心 `taskSignupList` vs 章节 `studentcourse` 返回不同结构 |
| 2 | 新版 `knowledge/cards` 只有 `num=0` 页有完整 `mArg`，`num>=1` 是 `mArg = $mArg`（脚本注入） | 已修复，见 `api/base.py` 的 `get_job_list` |
| 3 | 视频有 `enableVideoWatchDuration` 时，**2 倍速只按一半计入**，原地心跳完全不计 | 2 倍速跑完 160s 视频 → 未完成；1 倍速再跑 → 完成；原地心跳 72s → 仍不完成 |
| 4 | 视频"播完"类（`enableVideoComplated`）可以用配置倍速 | 72s 视频 2 倍速完成 |
| 5 | 任务点完成状态**有延迟**，需等待 + 复查 | 文档/视频打点后数十秒~数分钟才翻状态 |
| 6 | 任务中心分组是**闯关式**，未解锁时 `getToStudyUrl` 直接返回 `任务点未解锁，不允许学习` | 直接调用结果 |
| 7 | 文档 `readPoint` 打点 + `readEnd` **单独不生效**（10 分钟实验仍未完成） | `tools/probe/04_文档阅读打点实验.py` 输出 |
| 8 | 任务中心章节页的 JSON 字段和原始 `eTaskUserId` 已由页面源码确认；补齐后服务端返回 `enc不正确`，说明结构已识别，但 `contentData.enc/time/finishCount` 的真实来源仍未找到 | 页面源码对照 + `tools/probe/06_章节类任务点同步实验.py` 受控复验 |
| 9 | AI实践可**重答刷分**（`answerCountLimit: 100`、`answerRecords` 多条记录） | `load-data` 响应 |
| 10 | AI实践的对话是 **SSE 流**（`main-talk`），提交是空 JSON 的 `/answer/submit`；**提交不会出分**，成绩由 `POST think/end-report`（页面"学习质量评估报告"按钮）现算并回填 | 前端 chunk 反解 + 2026-09-17 真机：提交后 10 分钟仍未评估，请求 end-report 立刻返回 `{"id":"score","content":"100"}` |
| 13 | AI实践**一次练习 = 逐个知识点答一遍**（本课程 9 题），9 题全对 = 100 分（评估报告："所有题目作答全部正确…无作答错误"）；只答 1 题就提交 = 30 分 | 真机 6 条记录对照：`[100, 100, 100, 100, 60, 30]`，平均 81.7，`answerScore=100` |
| 14 | AI实践的判分是**平台自己的大模型**，会自相矛盾（标准答案判错、判断题"错""对"都判错）；且知识点答完后平台仍会把最后一题推回来而**不再收录作答**（`messageList` 不增长） | 真机实测：同一题"错/对"来回换十几次；整局曾拖到 112 题、空转 23 分钟 |
| 11 | 章节类任务点的同步数据 `stuJobInfo` 由**平台下发**：章节页 `isTaskEngineNode=true` → `window.isEngineNode="1"`；视频打点响应 / `/mooc-ans/job/document` 响应里带 `stuJobInfo`，父页面 POST `autoPullChapterScore` | 真实浏览器抓包 + 同步后引擎复查，见 `docs/artifacts/capture_autopull.txt` |
| 12 | mooc 视频打点在部分网络下按**客户端指纹**拦截：同一 URL/参数/Cookie/UA，Chrome 200、curl 200、Python requests **403**；且 `playTime` 到结尾但未通过时必须在结尾反复重报是没用的，要**从头回看**（浏览器从 0 播到 ~232s 才通过） | 2026-09-17 实测；CLI 已加 curl 回退（决 D12）与回看逻辑（决 D13） |
| 15 | 任务中心作业走**新版学习页**：`getToStudyUrl` → `mooc2/work/task`（302 到 `dowork`）；章节测验的 `mooc-ans/api/work` 对课程级作业返回 **403**，作业提交接口是 `addStudentWorkNewWeb`。题型在 `input[name=answertype<id>]`（0 单选/1 多选/2 填空/3 判断/4 简答），题干要按 DOM 顺序拼——真实页面把 `<p>` 嵌在 `<h3>` 里属非法 HTML，lxml 会提前闭合 h3 | 2026-09-20 真机取证 `docs/artifacts/capture_homework.txt`；第1章作业自动作答提交后平台判 **83.3** 分、`isFinish=true` |

---

## 4. 未验证 / 待确认（接手后第一优先级）

1. 文档 `ac_mark` 的请求虽然已按前端样例复现（含 enc、Accept、Referer），但服务端是否计入任务时长仍未证实。
2. `autoPullChapterScore` 的 `enc/time/finishCount` 真实取值来源（父页只负责转发 `JOB_FINISH_INFO`，具体 `contentData` 仍需浏览器完成事件证据）。
3. 作业学习页的真实 URL 与取题/提交接口（当前被锁定分组挡住）。
4. 主题讨论的发帖接口与"已有回复"列表接口。
5. 课堂活动（签到 type 2）在任务中心的处理方式（老代码里有 `pre_sign/sign_in_normal`，未接线）。

---

## 5. 当前阻塞与下一步

| 阻塞 | 缺什么 | 下一步第一条命令/动作 |
| --- | --- | --- |
| 文档时长 | 已两次真机复验（泰康/吉利各 600 秒打点 + readEnd）：平台仍未计入 | 已停止重复消耗真实阅读时间；同一文档 24 小时内不重复读。下一步：用浏览器 DevTools 对照阅读器 JS 的完整行为（是否有页面/滚动事件参与计账） |
| 章节同步 | 第1章第二个章节任务点的真机验收（视频需回看 787s） | `./.venv/bin/python tools/probe/06_章节类任务点同步实验.py --kid 1199477654` |
| 思考题 | 取题/提交接口（第5章分组解锁后） | 抓思考题学习页 → `capture_question.txt` |
| 主题讨论完成同步 | ✅ 已复验：回复后约 3 分钟 `isFinish` 翻转（22:28 false → 22:31 true）；复查命令 `tools/probe/11_主题讨论只读取证.py --check-finish` | 无需额外同步接口；上游 `wait_plan_finished` 的等待+复查正好覆盖这个延迟 |
| AI实践刷分 | 已 ✅（2026-09-17 真机 4 次 100 分、平均 81.7、`answerScore=100`）；只有"未完成的 AI实践任务点"当前都锁在第4/5/7章，没能直接观察 `isFinish` 从 false → true 的翻转 | 第4/5/7章前置分组解锁后复验一次 |

细节（抓什么、存哪、验收标准）见 `docs/handoff/TASKS.yaml` 与 `CAPTURE-PROTOCOL.md`。

---

## 6. 文件与资产地图

**改代码主要落在**：`api/task_center.py`（任务点类型分派）、`api/ai_writer.py`（文案）、
`main.py`（`run_task_center_phase` 编排）、`api/base.py`（章节侧能力复用）。

**必看样例**：

| 文件 | 内容 |
| --- | --- |
| `docs/artifacts/classification.json` | 7 个教学任务的全部分组/任务点/完成规则 |
| `docs/artifacts/group_plans_sample.json` | 任务点原始 JSON（含 `planBreakthroughSet` 全字段） |
| `docs/artifacts/ac_mark_samples.json` | 3 条文档 `readPoint` 真实请求（d/t/enc） |
| `docs/artifacts/pan_markdata_sample.json` | 阅读器页 `markDataStr` 样例 |

**探针脚本**：`tools/probe/01~09`（只读或幂等），用途见 `docs/RUNBOOK.md`。

---

## 7. 整体验收标准（Definition of Done）

1. `make lint` 全绿，新增功能带离线单测。
2. 用账号 173****6569 跑 `./cx --yes`（或 `python main.py -c ~/.chaoxing/config.ini`），
   在**不打开浏览器**的情况下：
   - 7 个教学任务全部推进到 `taskStudyProgress = 1.0`；
   - 作业分数 ≥ 60；AI实践评分 ≥ 85；讨论/思考题提交成功且内容像真人；
   - 全程无"假装完成"（失败必须在日志和控制台说清楚）。
3. `docs/handoff/SESSION-LOG.md` 记录完整验证过程与观察到的进度变化。

---

## 8. 风险与注意事项

- **时间成本**：整门课需要数小时真实播放（仅第 1 章 3 个视频就 ~36 分钟）。
  不要为了"快"去加速或伪造心跳——平台会把异常学习数据回退为未完成。
- **风控**：抓包和探测都要慢；短时间大量请求会触发验证码或进度回退。
- **账号安全**：cookie 在 `~/.chaoxing/accounts/cookies/`；不要提交、不要外发。
- **改动边界**：任务中心的失败不能影响章节刷课（`main.py` 已隔离，别破坏）。

---

## 9. 相关文档

- `docs/handoff/TASKS.yaml` / `TASKS.md` — 任务板
- `docs/handoff/CAPTURE-PROTOCOL.md` — 浏览器抓包 SOP
- `docs/handoff/DECISIONS.md` — 已做决策
- `docs/handoff/SESSION-LOG.md` — 上一轮 Agent 时间线
- `docs/handoff/DEEP-DIVE-教学任务适配.md` — 详细版交接（原 `docs/交接文档.md`）
- `docs/任务中心与章节分类.md` — 平台结构分类与接口表
- `docs/ARCHITECTURE.md` / `docs/RUNBOOK.md`
