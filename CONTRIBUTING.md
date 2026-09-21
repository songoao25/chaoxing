# 贡献指南（Contributing）

感谢你考虑为本项目贡献！以下是指南，请先阅读再提交。

## 如何贡献

### 报告 Bug

- 先搜索 [Issues](https://github.com/songoao25/chaoxing/issues) 是否已存在；
- 新建 Issue 请使用 Bug 模板，并包含：复现步骤、期望行为、实际行为、运行环境；
- 日志片段请先脱敏，**不要**粘贴 cookie、账号、API Key 或完整日志。

### 提出新功能

- 先在 Issues 中说明用途和场景，避免重复劳动；
- 讨论通过后再实现。

### 提交代码

1. Fork 本仓库并创建分支：`git checkout -b feature/xxx`
2. 遵循 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)：
   - `feat: 新功能`
   - `fix: 修复`
   - `docs: 文档`
   - `test: 测试`
   - `chore: 杂项`
3. 提交信息用英文或中文均可，但要能说清改动；
4. 通过 Pull Request 提交，按模板写清改动内容和验证方式。

### 提交前请自测

```bash
make lint     # 编译检查 + 全量离线单测（当前 245 项）
```

改行为必须带离线单测（`tests/`，参考 `tests/test_task_center.py` 的 FakeSession 写法）。测试不联网、不碰真实账号。

## 必须遵守的硬规矩

这些是本项目踩过坑才定下的，评审和 CI 都会检查：

- **不许假装成功**：拿不到平台证据（接口返回成功 / 状态复查为完成）就不能返回成功，更不能把「跳过」记成「完成」。
- **尊重平台真实时间限制**：有观看时长要求的视频必须 1 倍速真实播放（不够回看）；文档必须按 30 秒节奏打点。不要试图加速、伪造心跳或跳过时长。
- **顺序解锁**：任务中心只能按分组顺序推进，完成一组后重新拉取分组状态。
- **风控**：接口之间保持 1~2 秒间隔，不并发轰炸任务引擎。
- **不提交密钥与 cookie**：`~/.chaoxing/` 下的内容、抓包样例里的 token/cookie 都不许入库；新增抓包样例前先脱敏。
- **面向同学的文本去 AI 味**：作业简答 / 讨论 / 思考题必须走 `api/ai_writer.py`，并过两遍真人化审计（`python tools/audit/01_真人化审计.py` + 独立上下文复核）。
- **失败软着陆**：任务中心整段失败不得影响章节刷课。

完整铁律见 [`AGENTS.md`](AGENTS.md)。

## 开发环境

- Python 3.13+；没有 ruff / pytest，单测使用标准库 `unittest`；
- 常用命令：

| 命令 | 作用 |
| --- | --- |
| `make test` | 离线单测 |
| `make lint` | 编译检查 + 单测（提交前必跑） |
| `make status` | 只读的任务中心进度快照 |
| `make doctor` | 环境自检 |

- 用户数据在 `~/.chaoxing/`（配置、cookie、缓存、日志），不在仓库里。

## 合并与发布

- 外部贡献者的 PR 需要维护者 review 后手动合并；
- 合并进 `main` 后由维护者决定版本号与发布；
- 版本记录见 [CHANGELOG.md](CHANGELOG.md)。

## 行为准则

请遵守[行为准则](CODE_OF_CONDUCT.md)。参与本项目即表示你同意遵守它。

## 许可证

贡献的代码将采用与本项目相同的 [GPL-3.0 许可证](LICENSE)，并保留上游 [Samueli924/chaoxing](https://github.com/Samueli924/chaoxing) 的出处。
