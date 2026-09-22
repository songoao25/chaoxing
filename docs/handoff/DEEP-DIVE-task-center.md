# 交接文档：学习通「任务中心 / 教学任务」刷课适配

> 本文档交给有**浏览器操控能力**的 AI 接手。前半部分是已探明的全部事实与代码现状，
> 后半部分是**必须靠真实浏览器抓包才能继续的任务**，每项都写清楚了"抓什么、交付什么、验收标准"。
>
> 生成时间：2026-09-16；代码仓库：`<仓库根目录>`

---

## 0. 一句话目标

让这个 CLI 刷课工具**不打开浏览器**就能把「任务中心 → 教学任务」完整刷完：
视频、文档、章节、作业、思考题、主题讨论、AI实践，全部分组按顺序解锁，
问答/讨论内容用大模型生成并**去 AI 味**（讨论要模仿真实同学的回复风格），
AI实践的评分要刷到 80~90 分（当前是 30 分）。

---

## 1. 环境与运行方式

| 项 | 值 |
| --- | --- |
| 仓库 | `<仓库根目录>`（origin: `<fork>`，upstream: Samueli924/chaoxing） |
| Python | `./.venv/bin/python`（3.14） |
| 依赖 | `requirements.txt`（requests / bs4 / loguru / tqdm …） |
| 启动 | `./cx`（交互）、`./cx --yes`、`./.venv/bin/python main.py -c ~/.chaoxing/config.ini` |
| 用户数据 | `~/.chaoxing/`：`config.ini`、`accounts/cookies/<手机号>.txt`、`cache.json`、`chaoxing.log` |
| 测试 | `./.venv/bin/python -m unittest discover -s tests -t .`（当前 123 项全绿） |
| 日志 | `~/.chaoxing/chaoxing.log`（全量），控制台只显示警告以上 |

**cookie 用法**（探针脚本都这么取）：`SessionManager.get_session().cookies.update(cookie_mod.use_cookies("<手机号>"))`。

---

## 2. 账号与目标课程

| 账号（已脱敏） | 课程 | 说明 |
| --- | --- | --- |
| 139****0000 | 15 门 | 主账号（用户常用） |
| **138****0000** | **示例课程** | **本次适配的目标课程，任务中心有 7 个教学任务** |

目标课程参数（大量接口都要这三个值；示例课程，均已脱敏）：

```
courseId = <courseId>
clazzId  = <clazzId>
cpi      = <cpi>
fid      = <fid>
uid      = <uid>（passportUID，任务引擎用）
```

浏览器直接打开的地址：
`https://mooc2-ans.chaoxing.com/mooc2-ans-vue/fanyav3/stu?courseid=<courseId>&clazzid=<clazzId>`
（首页 tab = 任务中心，章节 tab = 老目录）

---

## 3. 平台结构严格分类（避免把概念混在一起）

| 层级 | 数据来源 | 结构 |
| --- | --- | --- |
| **章节（目录）** | `mooc2-ans/mycourse/studentcourse` → `mooc-ans/knowledge/cards` | 章 → 任务点卡片（video/document/read/workid/live） |
| **任务中心 · 教学任务** | `mooc2-ans/fanya3/s/taskSignupList` | 教学任务 → **分组（闯关式解锁）** → 任务点(plan) |
| 任务中心 · 课堂活动 | `mooc2-ans/fanya3/s/activityList` | 签到(type 2)、提醒(type 45) 等 |
| 任务中心 · 作业考试 | `mooc1-api.chaoxing.com/mooc-ans/work/api/task`、`/exam-ans/mooc2/exam/task` | 课程级作业/考试（当前返回"无效的用户"，待修） |
| 任务中心 · 学情概览 | `stat2/study-data/v3/{overview,job,work,test,exam,ai-evaluate}` | 完成数统计（当前直连拿不到 JSON，可能要走 stat2 专用域名） |

**关键结论：章节和教学任务是两套互相独立的学习入口，记录不互通。**

### 教学任务的 7 种任务点类型（`planType`）与完成规则（来自 `planBreakthroughSet`）

| planType | 名称 | 完成规则字段 | 人话 |
| --- | --- | --- | --- |
| 10 | 视频 | `enableVideoComplated=1` | 播完即可 |
| 10 | 视频 | `enableVideoWatchDuration=1, videoWatchDuration=2.0` | **累计观看 2 分钟** |
| 11 | 文档 | `enableDocumentWatchDuration=1, documentWatchDuration=5.0` | **累计阅读 5 分钟** |
| 11 | 文档 | `enableCompleteRead=1` | 还要 readEnd |
| 8 | 章节 | `enableChapterFinishPercent=1` | 对应章节点全部完成（需任务引擎同步） |
| 4 | 作业 | `enableWorkScore=1` | 作业要有得分 |
| 9 | 思考题 | 无特殊规则 | 提交回答 |
| 14 | 主题讨论 | `enableReplyDiscuss=1` | 回复讨论 |
| 15 | AI实践 | `enableAicoach=1` / `enableFinishAicoach=1` | 完成 AI 对话（有评分） |

分组解锁条件示例：`所有任务点闯关完成；每个视频完成观看`、`每篇文档完成阅读`、`完成AI对话`、`需完成上个分组`。
**硬约束**：上一组没完成，`getToStudyUrl` 直接返回 `任务点未解锁，不允许学习`，无法跳级。

---

## 4. 已完成的代码与验证结论

### 4.1 新增/修改的文件

| 文件 | 内容 |
| --- | --- |
| `api/task_center.py` | 任务中心客户端：任务/分组/任务点读取、顺序解锁推进、视频真实节奏打点（含时长回看）、文档 readPoint 打点 + readEnd |
| `api/ai_writer.py` | 去 AI 味文案生成器：`answer()` / `discussion()`，用配置里的大模型，讨论会参考已有回复 |
| `main.py` | 章节刷完后自动跑任务中心（`run_task_center_phase`），配置 `task_center=true`，`--task-center/--no-task-center` |
| `api/base.py` | **修复新版 cards 解析**：`num>=1` 页只有 `mArg = $mArg`，旧逻辑一解析失败就整章判失败 → 新版课程一个任务点都读不到 |
| `api/decode.py` | mArg 缺失日志降级（避免刷屏） |
| `config_template.ini` / `README.md` / `api/README.md` | 配置项与说明 |
| `docs/PLATFORM-NOTES.md` | 平台结构分类 + 接口表 + 阻塞点 |
| `docs/artifacts/*.json` | 原始抓取样例（见第 9 节） |
| `tools/probe/*.py` | 9 个可复用的只读探针脚本（见第 9 节） |
| `tests/test_task_center.py` / `tests/test_ai_writer.py` | 25 项离线单测 |

### 4.2 已实测验证过的结论（重要，别重复踩坑）

1. **视频时长要求按真实时间算**：同一个视频用 2 倍速打点只按一半计入；停在原地发心跳（时间不前进）**完全不计**。→ 有 `enableVideoWatchDuration` 的视频必须 1 倍速真实播放，不够就回看（本仓库已实现）。
2. **视频"播完"类**（只有 `enableVideoComplated`）可以用配置倍速。
3. **任务点完成状态有延迟**：打点够了之后要等几十秒到几分钟才变，验证要留等待+复查（已实现 `wait_plan_finished`，6 次 × 5 秒）。
4. **文档 readPoint 打点单独不生效**（见任务 A）。
5. **章节类任务点 mooc 侧刷完了，任务引擎不认**（见任务 B）。
6. **AI实践可以重答刷分**：`load-data` 返回 `remainAnswerCount: 98`、`answerRecords` 多次记录，说明最多可重答 100 次、按记录取分。

---

## 5. 已探明的接口清单（可直接抄）

### 5.1 任务中心（`mooc2-ans.chaoxing.com`）

| 接口 | 方法 | 参数 | 返回 |
| --- | --- | --- | --- |
| `/mooc2-ans/fanya3/s/taskSignupList` | GET | courseId, clazzId, cpi | `{status, data:[{id,name,planCount,planFinishCount,taskStudyProgress,...}]}` |
| `/mooc2-ans/fanya3/s/activityList` | GET | courseId, clazzId, cpi | 课堂活动列表（type 2=手势签到，45=提醒） |

### 5.2 任务引擎（`task.chaoxing.com`）

| 接口 | 方法 | 参数 | 说明 |
| --- | --- | --- | --- |
| `/api/v1/middlePageApi/jumpStudyPlanList` | GET | taskId | 302 到 `/userStudyPlan/myPlanList?encryTaskUserId=...`，HTML 里有 `const eTaskUserId = "..."`、`const encryTaskId = "..."` |
| `/userStudyPlan/getGroupData` | POST | encryTaskUserId | 分组：`groupAllowStudy`、`groupAllowStudyCondition`、`encryptGroupId` |
| `/userStudyPlan/getPlanDataByGroupId` | POST | encryTaskUserId, encryGroupId | 任务点：`planId/planType/name/isFinish/planUser.finish/planBreakthroughSet/encryptPlanId/externalDataId` |
| `/userStudyPlan/getToStudyUrl` | POST | encryptPlanId, encryTaskUserId, studyJumpType=0, isInterface=false | 学习地址；未解锁返回 `{result:false,message:"任务点未解锁，不允许学习"}` |
| `/userStudyPlan/autoPullChapterScore` | POST(JSON) | encryTaskUserId, uid, finishCount, clazzId, enc, time, jobCount, knowledgeId | **章节类任务点同步**（2026-09-17 已打通）。`data` 就是平台下发的 `stuJobInfo`，CLI 从视频打点 / `/mooc-ans/job/document` 响应里取，详见 `docs/artifacts/capture_autopull.txt` |
| `/videoDataLog/dataLog/{duration}/{currentTime}/{status}?encryId=` | GET | status: 0=开始 1=心跳(6s) 2=结束 | 视频打点 |
| `/planUserSchedule/lastCurrentTime/{t}?encryId=` | GET | | 播放位置 |
| `/videoStudy/learnPage?encryPlanUserId=` | GET | | 视频页，HTML 内嵌 `const videoLearnVo = {...}`（encryId、videoInfo.duration、farthestTimeValue） |
| `/documentStudy/learnPage?encryPlanUserId=` | GET | | 文档页，HTML 内嵌 `const docStudyUrlInfo = {"yunPanUrl":...}` 和 `encryPlanUserId` |
| `/documentStudy/readEnd?encryPlanUserId=` | GET | | 标记读完（返回 `{"result":true}` 但不一定算完成） |

### 5.3 文档阅读打点（`data-xxt.aichaoxing.com`，**未打通**）

阅读器页面（`pan-yz.chaoxing.com/screen/v2/...`）每 30 秒发一次：

```
GET https://data-xxt.aichaoxing.com/analysis/ac_mark?&f=readPoint&u=<passportUID>&d=<JSON>&t=<yyyyMMddHHmmssSSS>&enc=<32位小写哈希>
d = {"r":"<objectId>","t":"doc","l":1,"f":4,"p":1,"tp":2,"wc":0,"ic":2,"v":2,"s":<序号>,"h":0,"ext":"{\"rwyq_doc\":<planId>,\"pId\":<planId>,\"oId\":\"<objectId>\",\"mrId\":0}"}
```

现有三组真实脱敏样例可由 `addPoint.js` 的规则复现：对参数名排序后拼接 `f/u/t/encodeURIComponent(d)`，追加盐 `NrRzLDpWB2JkeodIVAn4` 后取 MD5。接口仍可能对无效来源返回 HTTP 200 空 body；两轮带签名的真实 5 分钟验证也未让任务点完成，因此最终判定必须依赖任务中心状态复查，不能只看 HTTP 状态。样例见 `docs/artifacts/ac_mark_samples.json`。

### 5.4 AI实践 / 思维阶梯（`mooc2-ans.chaoxing.com`）

学习页：`/ai-evaluate/v2/answer?courseid=&clazzid=&cpi=&publishRelationUuid=<uuid>&__from=taskEngine`
→ 302 到 `/mooc2-ans-vue/think-ladder/answer/pc-index?...&aiEnc=<hash>`

| 接口 | 方法 | 参数 | 说明 |
| --- | --- | --- | --- |
| `/mooc2-ans/ai-evaluate/v2/answer/init` | POST(form-urlencoded) | `{type:2, courseid, clazzid, cpi, publishRelationUuid}` | 返回 `answerUuid`、`recordUuid` |
| `/mooc2-ans/ai-evaluate/v2/answer/load-data` | GET | courseid, clazzid, cpi, publishRelationUuid | 题目/要求/`dimensionList`/`answerRecords`/`answerScore` |
| `/ai-ans/ai-evaluate/think/main-talk` | POST(**SSE**) | query: courseId, clazzId, cpi, aiEnc, recordUuid, isPreview, newAnswerView；body(`application/x-www-form-urlencoded`): `userMessage=<回答>` | 学生发一条 → AI 流式回复（SSE chunk 里 `type` 有 preAppendContent / questionType / questionTypeInt / dimension / knowledgePoint / questionStem / option-X） |
| `/ai-ans/ai-evaluate/think/change-question` | POST(SSE) | 同上 | 换一题 |
| `/mooc2-ans/ai-evaluate/v2/answer/submit` | POST(form-urlencoded 空体) | courseid, clazzid, cpi, recordUuid | 提交并出评分报告 |

现状样例：`answerScore: 30.0`、`answerCount: 2`、`scoreCount: 1`、`remainAnswerCount: 98`。

### 5.5 章节（老接口，已修复）

`GET https://mooc1.chaoxing.com/mooc-ans/knowledge/cards?clazzid=&courseid=&knowledgeid=&ut=s&cpi=&v=2025-0424-1038-3&mooc2=1&num=<0..6>`
- **只有 `num=0` 页里有完整 `mArg = {...}`（含 attachments）；`num>=1` 页是 `mArg = $mArg;`（脚本注入，无 JSON）**。
- 逐页解析、坏页跳过、按 jobid 去重（已实现）。
- 章节号 → 任务点卡片：`video` / `document` / `read` / `workid` / `live`。

新增发现：任务引擎给章节类任务点的学习地址是
`https://mooc1.chaoxing.com/courseapi/stustudy-page-transfer?chapterId=<kid>&courseId=&classId=&uid=&enc=&isTaskEngineNode=true&hideCatalog=1&hideType=1`
→ 302 到老 `/mooc-ans/mycourse/studentstudy?...&enc=...`（带 `isTaskEngineNode=true`）。
  该页面会注入 `window.isEngineNode="1"`，并定义 `sendStuJobInfoMsg(data)`：把
  `{module:"KNOWLEDGE", type:"JOB_FINISH_INFO", data}` 用 `window.opener.postMessage` 交给任务中心父页面。

  **`data` 的真实来源（2026-09-17 定位并抓包）**：
  - 视频：`/ananas/videojs-ext/videojs-ext.min.js` 的 `sendLog` 回调 —— 打点响应里
    `allowSendStuJobInfoMsg === true` 且 `window.parent.parent.isEngineNode == '1'` 时，
    调用 `window.parent.parent.sendStuJobInfoMsg(resp.stuJobInfo)`；
  - 文档：`/ananas/ueditor/documentJob.js` 的 `finishJob()` —— `GET /mooc-ans/job/document?...&courseEngineInfo=true`，
    响应里同样带 `stuJobInfo`。

  所以 CLI 的实现是：刷章节时带 `courseEngineInfo=true`，从响应里捕获 `stuJobInfo`，
  再 POST `autoPullChapterScore`（**不能自己拼 `enc`**，决 D11）。
  注意：mooc 视频打点在部分网络下会被风控按客户端指纹拒绝（requests 403 / curl 200，决 D12），
  且 `playTime` 到结尾但未通过时要从头回看（决 D13）。

---

## 6. 接手任务（按优先级）

### 任务 A：文档阅读时长（阻塞 3 个任务点，其中两个要求 10 分钟）

- **现状**：`api/task_center.py` 的 `study_document()` 已实现"打开阅读器 → 取 markDataStr → 每 30 秒发 readPoint → 调 readEnd"，实测任务点不完成。
- **判断**：`enc` 规则已由样例和前端脚本确认；但服务端计账还没有被任务状态证明。
- **需要抓包**：浏览器打开一个文档任务点，读 1~2 分钟，F12 Network 过滤 `ac_mark`：
  1. 至少 **2 条不同时间**的请求，右键 **Copy as cURL**（要带完整 URL、请求头、Cookie）；
  2. 同时把该页 HTML 里的 `<div id="markDataStr">...` 内容一起发我（里面是 r/oId/ext/token）。
- **交付**：把 cURL 文本存到 `docs/artifacts/capture_doc_ac_mark.txt`。
- **验收**：用捕获的 enc 规律在 Python 里复现后，文档任务点能在要求时长后变成 `isFinish=true`。

### 任务 B：章节类任务点同步（2026-09-17 已打通，真机验收进行中）

- **已确认**：同步数据由平台在章节页（引擎节点）下发，CLI 捕获 `stuJobInfo` 后 POST
  `autoPullChapterScore`；真实账号上 `5639750` 同步成功、第1章 `0.3333 → 0.5`。
- **代码**：`api/base.py`（`engine_info`、`stuJobInfo` 捕获、curl 指纹回退、回看）、
  `main.py`（`process_chapter(engine_info=True)` → `sync_chapter_plan` → 复查）、
  `api/task_center.py`（`SUPPORTED_PLAN_TYPES` 含章节）。
- **交付**：`docs/artifacts/capture_autopull.txt`（已写）。
- **剩余验收**：第1章第二个章节任务点（kid=1199477654，视频 787s 需 1 倍速回看）与
  "文档是本章最后一个任务点"的路径；探针：`tools/probe/06_章节类任务点同步实验.py --kid 1199477654`。

### 任务 C：作业（课程级，4 个）

- **现状**：全部在锁定分组，未解锁；`mooc1-api.chaoxing.com/mooc-ans/work/api/task` 直接调返回 `无效的用户`。
- **需要抓包**：等前置分组刷完后（或你手动用浏览器做完前置组），点开一个作业任务点：
  - 学习地址（`getToStudyUrl` 返回的 URL 全量）；
  - 作业答题页的取题接口（大概率是 `mooc-ans/api/work?api=1&workId=...`）请求与响应；
  - 提交接口（大概率是 `mooc-ans/work/addStudentWorkNew`）请求体格式。
- **交付**：`docs/artifacts/capture_homework.txt`。
- **实现落点**：`api/task_center.py` 新增 `study_homework()`，复用 `api/base.py` 里现成的 `study_work()`（题库/大模型答题 + 判分重做）与 `api/ai_writer.py` 的简答题生成。
- **验收**：作业得分 ≥ 60，任务点 isFinish。

### 任务 D：主题讨论 / 思考题（2 个讨论 + 1 个思考题）

- **现状**：讨论在锁定分组；思考题（第5章）平台显示已完成，没抓到提交接口。
- **需要抓包**：解锁后，在浏览器里发一条讨论回复（内容随便，例如"测试"，之后可删）：
  - 讨论列表/详情接口（拿帖子和**其他同学已有的回复内容**，用于模仿风格）；
  - 发回复接口（URL + 请求体字段）。
- **实现落点**：`api/ai_writer.py` 的 `discussion(topic, requirement, existing_posts)` 已经写好（会参考已有回复、去 AI 味、控制长度）；只差调用 + 提交。
- **验收**：讨论提交成功，`enableReplyDiscuss` 任务点 isFinish。

### 任务 E：AI实践刷到 80~90 分（用户最在意，当前 30 分）

- **现状**：接口已由前端 bundle 定位并完成浏览器无关客户端与离线测试（见 5.4）；仍缺脱敏浏览器请求样本和当前账号的真实高分验收。
- **需要抓包**：浏览器打开 AI实践任务点（开学第一课「怎样才能通过学习战略管理成为一个CEO」）：
  1. F12 过滤 `main-talk`，在对话框里输入一句真实回答并发送，把请求的 URL + **请求体（userMessage=... 的完整表单）**+ **SSE 响应前几条 chunk** 存下来；
  2. 再抓一次 `submit` 请求（URL + body）。
- **交付**：`docs/artifacts/capture_ai_practice.txt`。
- **现行实现**（提交模式默认 `confirm`，只有显式 `auto` 才提交）：
  1. `load-data` 拿 `title/requirement/dimensionList`（AI 要考察的维度）和当前 `answerScore`；
  2. 每次作答前先 `init` 新建记录（可重答 100 次）；
  3. 循环：读 SSE 得到 AI 的问题（`questionStem/questionType/radioArr`）→ 用 `api/ai_writer.py` 生成答案（简答写正文；选择题就发选项字母）→ `userMessage` 发回；
  4. AI 说结束时 POST `/answer/submit?recordUuid=`；
  5. 再 `load-data` 读回 `answerScore`，**低于目标分（建议 85）就再 init 重答**，最多 N 轮（建议 5）直到达标。
- **验收**：`answerScore ≥ 85`，且任务中心的综合成绩里有体现。

---

## 7. 浏览器抓包规范（交给会操控电脑的 AI）

1. 用已登录的浏览器（用户本机 Firefox 已登录，账号 138****0000）打开目标页面；
2. F12 → Network → 勾选 Preserve log；过滤关键字按上面每项写明的；
3. 触发一次真实操作（点开任务点 / 读文档 / 发讨论 / AI 作答 / 提交作业）；
4. 对目标请求右键 → **Copy as cURL**（Firefox 是 Copy → Copy as cURL）；
5. **不要只给截图**，要文本；存到 `docs/artifacts/capture_<name>.txt`，文件名见各任务；
6. 如果是 SSE（AI实践），额外把响应面板里的前几行 chunk 文本一起存下来；
7. 抓包时**尽量少点、慢点**（平台有风控，短时间大量请求会触发验证码/回退）。

**抓完包之后**：按第 6 节的实现落点改代码，补 `tools/probe/` 下对应的探针脚本，跑通后把结论追加到 `docs/PLATFORM-NOTES.md`。

---

## 8. 代码约定（接手必读）

1. **不许假装成功**：任何一步拿不到证据就返回 `False` 并写日志，绝不能把"没做"记成"已完成"（历史 issue #223/#357 就是这个问题）。
2. **真实时间限制**：视频有 `enableVideoWatchDuration` 时必须 1 倍速（可回看补齐）；文档必须按 30 秒节奏打点；不要试图加速或伪造心跳。
3. **顺序解锁**：只能按分组顺序推进；完成一组后要重新读 `getGroupData` 等解锁，解锁有延迟（代码里已有重试）。
4. **去 AI 味**：所有要提交给老师/同学的文本（思考题、作业简答、讨论）必须走 `api/ai_writer.py`，不要直接塞大模型原文；讨论要模仿已有回复的长度和口吻。
5. **风控**：接口之间加 1~2 秒间隔，不要并发轰炸任务引擎；视频打点本来就慢，别叠加并发。
6. **失败软着陆**：任务中心整段失败不能影响章节刷课（`main.py` 里已隔离）。
7. 新功能要补**离线单测**（`tests/`，用 FakeSession/FakeTC 那套，参考 `tests/test_task_center.py`）。

---

## 9. 现成资产（省时间用）

### 探针脚本 `tools/probe/`（全部只读或幂等）

| 脚本 | 用途 |
| --- | --- |
| `01_分类总表.py` | 拉全量结构：教学任务/分组/任务点/规则 + 课堂活动 + 作业考试 + 章节抽样 |
| `02_当前任务状态快照.py` | 一眼看每个教学任务进度、哪些还没解锁 |
| `03_教学任务dry_run与实刷.py` | `dry`=只读看学习地址；`apply`=实刷指定教学任务 |
| `04_文档阅读打点实验.py` | 复现"打点 10 分钟仍不完成"，改 enc 后可直接验证 |
| `05_文档打开阅读器再readEnd.py` | 先开阅读器再 readEnd 的对照实验 |
| `06_章节类任务点同步实验.py` | 调 `autoPullChapterScore`（当前参数不对，等任务 B 抓包） |
| `07_作业学习页探针.py` | 解锁后看作业 URL/页面结构，并尝试复用 `study_work` |
| `08_AI实践接口探针.py` | 调 `init/load-data`，看题目/维度/分数 |
| `09_章节学习页同步实验.py` | 验证打开章节学习页是否触发同步（结论：不触发） |

### 原始样例 `docs/artifacts/`

| 文件 | 内容 |
| --- | --- |
| `classification.json` | 全量结构 dump（7 个教学任务的所有分组/任务点/规则/完成状态） |
| `group_plans_sample.json` | 单个分组的任务点原始 JSON（含 planBreakthroughSet 全字段） |
| `ac_mark_samples.json` | 3 组文档 readPoint 真实请求（d/t/enc） |
| `pan_markdata_sample.json` | 阅读器页面 markDataStr 样例（含 r/oId/ext/token） |

---

## 10. 账号当前进度快照（2026-09-16）

| 教学任务 | 进度 | 说明 |
| --- | --- | --- |
| 开学第一课 | **3/3 ✅** | 2 视频 + AI实践（**AI实践只有 30 分，任务 E 要刷高**） |
| 第1章 示例课程概述 | 2/6 | 视频✅；2 个章节 mooc 侧已刷完但任务引擎未同步（任务 B）；作业+文档锁定 |
| 第2章 企业愿景、使命、目标 | 0/6 | 第1组视频可学；后面 3 章节 + 作业 + 讨论全锁 |
| 第3章 企业外部环境分析 | 0/15 | 第1组视频+文档可学（文档要 5 分钟，任务 A）；其余锁 |
| 第4章 企业内部环境分析 | 0/14 | 第1组视频可学；文档要 10 分钟 |
| 第5章 企业总体战略 | 1/13 | 思考题已完成；视频可学 |
| 第7章 企业国际化战略 | 0/9 | 视频+文档可学（文档要 10 分钟） |

> 提醒：**整门课刷完需要真实花费数小时**（第1章 3 个视频就 ~36 分钟：696s/787s/682s）。如果继续用 138****0000 这个账号刷，注意不要频繁重跑视频（会重复计算真实时长）。

---

## 11. 已知坑

1. `knowledge/cards` 的 `mArg = $mArg`（已修，别改回去）。
2. 任务引擎的 `getToStudyUrl` 用 POST + query 参数；`getPlanDataByGroupId`/`getGroupData` 也是 POST + query。
3. `getUserPointsDetail` 要 JSON body；表单会 415。
4. `autoPullChapterScore` 要 JSON body；页面源码确认 `encryTaskUserId` 在 JSON 中使用原始 `eTaskUserId`，参数值不合法时会返回 `参数信息有误` 或 `enc不正确`。
5. 任务引擎完成状态有延迟，验证要等（已封装 `wait_plan_finished`）。
6. 视频打点 `status`：0=开始、1=心跳（每 6 秒）、2=结束/暂停。
7. AI实践的 `main-talk` 返回 SSE，不是普通 JSON。
8. `mooc1-api.chaoxing.com` 的作业/考试列表接口目前返回"无效的用户"，可能需要额外 header/域名（任务 C 一起解决）。
9. 环境里 `web_fetch` 工具会因为域名解析到内网被拦，抓网页请用 `curl` 或 `requests`。
