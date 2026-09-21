# 运行手册（Runbook）

## 1. 日常命令

```bash
make doctor      # 环境自检（python/venv/数据目录/依赖）
make lint        # 提交前必跑：编译 + 全量单测
make status      # 任务中心进度快照（只读，用 173****6569 的 cookie；账号已脱敏）
make classify    # 平台结构分类总表（只读，抽样请求，耗时约 1 分钟）
make probes      # 编译检查所有探针
```

## 2. 真实刷课

```bash
./.venv/bin/python main.py -c ~/.chaoxing/config.ini     # 用配置文件
./cx --yes                                               # 交互式向导 + 跳过确认
./.venv/bin/python main.py -u <手机号> -p <密码> -l <课程ID> --no-task-center   # 只刷章节
./.venv/bin/python main.py -u <手机号> -p <密码> -l <课程ID> --no-chapters     # 只刷任务中心
```

注意：
- 刷课范围：
  - 章节（目录）：配置 `[common] chapter_study = true/false`，或 `--chapters/--no-chapters`；
  - 任务中心教学任务：配置 `[common] task_center = true/false`，或 `--task-center/--no-task-center`；
  - 两者不能同时关掉（启动即停止并提示）；交互向导 `cx` 登录后会直接问一次。
- 每门课刷几个：章节 `max_points_per_course`（`-n/--max-points`）、
  任务中心 `max_tasks_per_course`（`--max-tasks`）；`0`=全部，支持 `课程ID:数量`。
- 作业：任务中心作业会自动作答并提交（选择题/判断题/填空题走题库，简答题走去 AI 味写作器）；
  提交模式默认 `auto`（`task_center_submit_mode`），需要逐次确认时改成 `confirm`。
- 刷课参数（倍速/并发/重做/未开放）已按推荐值写死，向导不再询问；改参数用命令行或配置文件。
- 控制台策略：刷课全程（含任务中心）控制台静音，内部 INFO/DEBUG 只进
  `~/.chaoxing/chaoxing.log`；控制台只留用户能看懂的进度行与 WARNING 以上。
- 刷课期间按 `q` 可随时停止；已完成的任务点下次自动跳过。
- **真实学习时间不可压缩**：有观看时长的视频按 1 倍速，文档按分钟打点。

## 3. 排查手册

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 「任务点页面里找不到 mArg」 | 平台改版 / 登录失效 | 先确认没有登录失效；查看 `docs/artifacts` 与 `api/decode.py`；新版 `num>=1` 页本就无 JSON，属正常跳过 |
| 「任务点未解锁，不允许学习」 | 分组闯关未完成 | 按顺序先做当前分组；解锁有延迟，等 30 秒再试 |
| 视频打点被拒绝 | 频率/倍速异常 | 检查是否对有时长要求的视频用了倍速；看 `~/.chaoxing/chaoxing.log` |
| 任务点做完但状态不变 | 完成状态有延迟 | 用 `wait_plan_finished` 等待复查；章节类见任务 B |
| 触发验证码 / 进度回退 | 风控 | 停 10~30 分钟，降低频率；不要短时间重跑同一视频 |
| 任务中心整段报错 | 接口变更 | 不影响章节；用 `tools/probe/01` 重新分类，更新 `docs/任务中心与章节分类.md` |

## 4. 日志与现场

- 全量日志：`~/.chaoxing/chaoxing.log`（控制台只显示 WARNING 以上）。
- 缓存：`~/.chaoxing/cache.json`（题库答案缓存）。
- cookie：`~/.chaoxing/accounts/cookies/<手机号>.txt`（**不要提交/外发**）。
- 任务中心状态：`make status` 或 `tools/probe/02_当前任务状态快照.py`。

## 5. 多账号

- 每个手机号独立 cookie 文件，由 `api/cookies.py` 隔离（防串号）。
- 探针脚本默认账号写在文件顶部（`173****6569`）；换账号改这一行即可。
