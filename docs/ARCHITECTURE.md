# 架构说明

## 1. 分层

```
main.py                     CLI / 配置 / 编排 / 通知 / 进度
 ├─ 章节链路（原有）
 │   └─ JobProcessor（并发）→ process_chapter → api/base.Chaoxing
 │        ├─ get_course_point / get_job_list   （knowledge/cards）
 │        └─ study_video / study_document / study_read / study_work / live
 ├─ 任务中心链路（新增）
 │   └─ run_task_center_phase → api/task_center.TaskCenter
 │        ├─ 读取：taskSignupList / jumpStudyPlanList / getGroupData / getPlanDataByGroupId
 │        ├─ 视频：videoStudy/learnPage → videoDataLog + planUserSchedule
 │        ├─ 文档：documentStudy/learnPage → data-xxt readPoint → readEnd
 │        ├─ AI实践：入口参数 → load-data → main-talk SSE → 分数复查
 │        └─ 章节：knowledgeId → 已保留章节链路，但等待真实 autoPull 参数后再接入
 └─ 文案：api/ai_writer.HumanLikeWriter（问答/讨论，去 AI 味）
```

## 2. 关键模块

| 模块 | 职责 | 扩展点 |
| --- | --- | --- |
| `api/base.py · Chaoxing` | 登录、章节任务点、mooc 侧学习动作 | 新增卡片类型时加 `study_xxx` |
| `api/task_center.py · TaskCenter` | 任务引擎读写、真实状态复查、提交门禁 | **新增 planType 就在这里加分派** |
| `api/ai_writer.py` | 面向人的文案 | 需要新文体（如作业简答）时加方法 |
| `main.py · run_task_center_phase` | 任务中心编排、进度打印、统计 | 汇总口径变化时改这里 |

## 3. 加一种新任务点类型（标准步骤）

1. 在 `api/task_center.py` 的 `PLAN_TYPE_*` 常量与 `PLAN_TYPE_NAMES` 注册类型；
2. 若可自动完成：加入 `SUPPORTED_PLAN_TYPES`，实现 `study_<type>(study_url, plan)`；
3. 在 `main.py · _complete_teaching_plan` 里加分派（章节类型走 `process_chapter`）；
4. 在 `tools/probe/` 加一个只读探针，验证学习地址/上报接口；
5. 补离线单测（`tests/test_task_center.py` 的 FakeSession/FakeTC 写法）；
6. 更新 `docs/任务中心与章节分类.md` 的类型表与支持状态。

## 4. 编排与容错

- 章节与任务中心**串行**：先章节（并发快），再任务中心（串行闯关）。见 `DECISIONS.md D1/D2`。
- 任务中心内部：教学任务 → 分组（只处理 `groupAllowStudy=true`）→ 任务点；
  每轮重新拉分组，等解锁延迟；支持 `interrupt` 随时终止。
- 会改变课程记录的 AI 实践/作业/讨论提交经过 `confirm` / `auto` 门禁；没有交互终端时 `confirm` 安全停止。
- 任何一步失败都返回 `False`，由上层记录为未完成，**不影响其他任务点**；锁定、等待确认、暂不支持不会转成完成。

## 5. 测试策略

- 全部离线：`FakeSession`（按 URL 路由）+ `FakeTC`（内存态分组/任务点）；
- 覆盖：解析、打点节奏、时长回看、SSE/失败分支、分数重答、提交确认、分组解锁、开关配置；
- 真实账号验证只在 `docs/RUNBOOK.md` 的流程里做，且要慢。
