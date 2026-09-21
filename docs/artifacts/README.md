# docs/artifacts 说明

这里存放**脱敏后**的抓包与探测样例，作为接口实现的证据。

| 文件 | 内容 | 备注 |
| --- | --- | --- |
| `capture_*.txt` | 浏览器抓包原文（已脱敏） | 账号/uid/课程参数/token 均为占位符 |
| `ac_mark_samples.json` | 文档阅读打点真实请求样例 | uid 为占位值，enc 按同一算法重算 |
| `pan_markdata_sample.json` | 阅读器 markDataStr 样例 | passportUID/token/oId/signature 为占位值 |
| `group_plans_sample.json` | 分组/任务点结构样例 | encryTaskUserId 已替换为占位符 |
| `classification.json` | **历史**结构分类快照（某次运行的 finish 状态） | ⚠️ **不代表当前进度**，要看当前状态请跑 `make status` |

脱敏规则见 `docs/handoff/CAPTURE-PROTOCOL.md`；提交前请再自查一遍敏感值。
