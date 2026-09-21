# 抓包 SOP（给有浏览器操控能力的 Agent）

> 目标：拿到**文本证据**（不是截图），存到 `docs/artifacts/`，供实现方直接复现请求。
> 原则：**少点、慢点、只抓必要请求**。平台有风控，短时间大量请求会触发验证码或学习数据回退。

## 0. 准备

- 浏览器已登录账号 **173****6569**（账号已脱敏；目标课程：企业战略管理）。
- 打开目标页面 → F12 → Network → 勾选 **Preserve log**（保留日志）。
- 每抓完一项：把请求 **Copy as cURL**（Firefox：右键 → Copy → Copy as cURL），
  粘贴到 `docs/artifacts/capture_<name>.txt`，并在文件头写清楚：
  - 页面 URL、操作步骤、时间；
  - 该请求的用途（一句话）。
- **脱敏**：curl 里的 `Cookie:` 整行替换成 `Cookie: <REDACTED>` 再提交（本地留一份完整的即可）。

## 1. 通用过滤词

| 目的 | Network 过滤 |
| --- | --- |
| 文档阅读打点 | `ac_mark` |
| 章节同步 | `autoPullChapterScore` |
| AI实践对话/提交 | `main-talk` / `answer/submit` / `load-data` |
| 作业 | `api/work` / `addStudentWorkNew` / `workId` |
| 讨论 | `bbs` / `discuss` / `reply` |

## 2. 文档阅读时长（→ `capture_doc_ac_mark.txt`）

1. 课程首页 → 任务中心 → 第 3 章「企业外部环境分析」→ 点开文档
   「"科大讯飞"上市十年的变与不变」（该任务点已解锁，要求阅读 5 分钟）。
2. 打开文档后**停留 1~2 分钟**，什么都不用做。
3. Network 过滤 `ac_mark`，应该能看到每 30 秒一条。
4. 复制 **2 条不同时间**的请求为 cURL（要能看到 `d` 和 `enc` 的差异）。
5. 同时在该页控制台执行并复制结果：
   `document.querySelector('[id=markDataStr]')?.textContent`
6. 存盘格式：

```
# page: <文档页 URL>
# step: 打开文档后停留 120 秒
# note: 两条 ac_mark 请求，间隔约 30 秒
<curl 1>

<curl 2>

# markDataStr:
{...}
```

## 3. 章节类任务点同步（→ `capture_autopull.txt`）

1. 任务中心 → 第 1 章 → 点开章节任务点「企业战略管理课程框架」
   （该章节 mooc 侧已刷完，但任务引擎显示未完成）。
2. Network 过滤 `autoPullChapterScore`（如果没有，就把整段请求都复制：`stustudy-page-transfer`、
   `studentstudy`、`knowledge/cards` 前后几条）。
3. 复制请求（**要完整 JSON 请求体**）+ 响应。
4. 若看到 `postMessage` 相关行为（例如通过 Console 监听 `window.addEventListener('message', e => console.log(e.data))`），
   把 `JOB_FINISH_INFO` 的 `data` 原样贴出来 —— 这是最关键的线索。

## 4. AI实践（→ `capture_ai_practice.txt`）

1. 任务中心 → 开学第一课 → 点开 AI实践「怎样才能通过学习战略管理成为一个CEO」。
2. Network 过滤 `main-talk`；在对话框输入一句真实回答（比如"我觉得CEO最重要的是判断方向和搭班子"）
   并发送。
3. 复制 `main-talk` 请求为 cURL（要 `userMessage=...` 的请求体），
   并在 Response 面板把**前 5 条 SSE chunk** 文本复制出来。
4. 再点到评分/提交（或等 AI 结束后触发提交），过滤 `answer/submit`，复制该请求。
5. 存盘时标注当前 `answerScore`（若页面上可见）。

## 5. 作业（→ `capture_homework.txt`，需前置分组解锁）

1. 任务中心 → 已解锁的作业任务点 → 点开进入作答页。
2. 复制：学习页 URL、**取题请求**（响应里应含题目 HTML/JSON）、**提交请求**（字段名要全）。
3. 不要真的乱交卷；如果必须提交才能抓到，用随意答案提交一次并注明（课程允许重做）。

## 6. 讨论/思考题（→ `capture_discussion.txt`，需解锁）

1. 点开主题讨论任务点，复制**帖子详情请求**（含其他同学的回复文本，用于模仿风格）。
2. 在讨论区发一条测试回复（内容如"测试，稍后删除"），复制发帖请求；抓完自行删除。

## 7. 抓完之后

- 在 `TASKS.yaml` 对应 `cap-*` 任务里补 `status: done`、`evidence: [文件路径]`；
- 在 `SESSION-LOG.md` 写一条记录（时间、操作、结果）；
- 然后按 `TASKS.md` 的顺序进入实现任务。
