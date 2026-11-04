# 超星助手 · 统一设计语言 v3（与「雨课堂刷课助手」对齐 · unslop）

> 用户要求：**两个桌面工具（超星助手 chaoxing / 雨课堂刷课助手 yuketang）保持统一风格，
> 用 unslop 去掉 AI 味，做成成熟、精致、符合 macOS 设计的界面。**
>
> 参考实现（只读）：`/Users/songsong/code/yuketang-jiaoben/frontend/app.css`（2026-11-04 快照）
> 与 `frontend/index.html`。本文件把它提炼为**共享设计语言**；数值能一致就一致，
> 语义色、圆角、行高、控件尺寸直接沿用，两个 App 并排看应像同一套产品的两个模块。
>
> 方法论：unslop（Damir Sokolovsky，CC BY 4.0）——安静 chrome、单一强调色、真实内容承载视觉、
> 逐层修间距/颜色/图标/文案/字体/布局/细节/视觉/动效、最后按清单验收。
> 禁止项：色相渐变、发光、彩色阴影、emoji 图标、渐变文字、全大写宽字距小标签、脉冲装饰、hover 位移 > 2px。

## 0. 方向

- **产品**：两个「学生自己用的课程自动化」工具，本地运行、数据在 `~/.xxx`，不联网除学习平台本身。
- **性格**：安静、精确、可靠（calm, exact, dependable）。
- **世界**：macOS 系统设置——分组卡片列表、语义色小图标、SF 字体、细滚动条。
- **重点**：不靠装饰。真实数据（课程、进度、日志、复核正文）就是全部视觉内容。
- **签名**（全站一致，两 App 共用）：① 30px 彩色 squircle 行图标（系统设置同款）；
  ② 页头 h1 + 一行说明 + 右侧操作；③ 侧栏底部状态圆点 + 版本号；④ 快捷键提示 ⌘1…。

## 1. 设计令牌（与 ykt 完全一致，亮/暗）

```css
:root {
  --font-ui: -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Helvetica Neue", Arial, sans-serif;
  --font-mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;

  /* 亮色 */
  --bg: #f5f5f7;
  --sidebar: #ececf0;
  --sidebar-glass: rgba(246, 246, 248, 0.72);
  --sidebar-border: rgba(0, 0, 0, 0.08);
  --card: #ffffff;
  --card-border: rgba(0, 0, 0, 0.04);
  --card-shadow: 0 0 0 0.5px rgba(0, 0, 0, 0.05), 0 1px 2px rgba(0, 0, 0, 0.04);
  --inset: rgba(120, 120, 128, 0.08);
  --row-hover: rgba(0, 0, 0, 0.03);
  --label: #1d1d1f;
  --label-2: #6e6e73;
  --label-3: #8e8e93;
  --separator: #e5e5ea;
  --fill: rgba(120, 120, 128, 0.08);
  --fill-2: rgba(120, 120, 128, 0.14);
  --accent: #007aff;
  --accent-hover: #0a84ff;
  --accent-soft: rgba(0, 122, 255, 0.12);
  --green: #34c759;  --green-text: #248a3d;  --green-soft: rgba(52, 199, 89, 0.14);
  --orange: #ff9500; --orange-text: #b25000; --orange-soft: rgba(255, 149, 0, 0.14);
  --red: #ff3b30;    --red-text: #d70015;    --red-soft: rgba(255, 59, 48, 0.12);
  --purple: #af52de; --indigo: #5856d6;      --gray: #8e8e93;
  --field-bg: #ffffff; --field-border: #d1d1d6;
  --btn-bg: rgba(0, 0, 0, 0.05); --btn-hover: rgba(0, 0, 0, 0.08); --btn-active: rgba(0, 0, 0, 0.12);
  --switch-off: #e9e9ea;
  --focus-ring: rgba(0, 122, 255, 0.32);
  --scroll-thumb: rgba(0, 0, 0, 0.20); --scroll-thumb-hover: rgba(0, 0, 0, 0.32);
  --toast-bg: rgba(28, 28, 30, 0.94); --toast-border: rgba(255, 255, 255, 0.10); --toast-text: #f5f5f7;
  --modal-shadow: 0 20px 60px rgba(0, 0, 0, 0.24);
  --sk-base: rgba(0, 0, 0, 0.05); --sk-shine: rgba(255, 255, 255, 0.6);
}

/* 暗色（prefers-color-scheme: dark，或 data-theme="dark"） */
--bg: #1c1c1e;        --sidebar: #232326;      --sidebar-glass: rgba(35, 35, 38, 0.72);
--sidebar-border: rgba(255,255,255,0.08);      --card: #2c2c2e;
--card-border: rgba(255,255,255,0.06);         --card-shadow: 0 1px 2px rgba(0,0,0,0.30);
--inset: rgba(255,255,255,0.06);               --row-hover: rgba(255,255,255,0.04);
--label: #ffffff; --label-2: #98989d; --label-3: #6c6c70; --separator: #3a3a3c;
--fill: rgba(255,255,255,0.07); --fill-2: rgba(255,255,255,0.12);
--accent: #0a84ff; --accent-hover: #3d9bff; --accent-soft: rgba(10,132,255,0.22);
--green: #30d158; --orange: #ff9f0a; --red: #ff453a; --purple: #bf5af2; --indigo: #7d7aff;
--field-bg: #1c1c1e; --field-border: #48484a;
--btn-bg: rgba(255,255,255,0.08); --btn-hover: rgba(255,255,255,0.12); --btn-active: rgba(255,255,255,0.16);
--switch-off: #39393d; --focus-ring: rgba(10,132,255,0.45);
--scroll-thumb: rgba(255,255,255,0.20); --scroll-thumb-hover: rgba(255,255,255,0.32);
--modal-shadow: 0 20px 60px rgba(0,0,0,0.5);
```

字体：正文 13px/1.45；页头 h1 22px/600 letter-spacing -0.015em；page-sub 12px `--label-2`；
row-title 13px；row-desc 11px `--label-2`；mono 数字 tabular-nums；日志 11.5px mono。
圆角：控件 6/7px，分组卡 10px，弹层 12px，行图标 8px，小图标按钮 5px。
阴影：只用 `--card-shadow` 与 `--modal-shadow`，禁止彩色阴影/发光。

## 2. 窗口外壳（两个 App 相同）

```
┌───────────────────────────────────────────────────────────┐
│ ●●●   [toolbar 52px：右侧状态点 + 刷新等操作，可拖动窗口]     │
├──────────┬────────────────────────────────────────────────┤
│ sidebar  │ main（padding-top 52px，滚动区 12px 32px 20px） │
│ 220px    │   page max-width 660px 居中                     │
│ 58px 顶部 │   page-wide 用于日志/复核双栏                   │
│ 留交通灯  │                                                │
└──────────┴────────────────────────────────────────────────┘
```

- `html/body` 在真实窗口里**透明**（`html.platform-darwin:not(.mock) body { background: transparent }`），
  主内容区 `.main` 自己铺 `--bg`；侧栏 `--sidebar-glass` + `backdrop-filter: blur(28px) saturate(1.6)`，
  透出桌面毛玻璃（与 ykt 一致）。
- toolbar：`position: fixed; top:0; left:0; right:0; height:52px; padding:0 20px 0 72px;`，
  `--wails-draggable: drag`；左侧 72px 为交通灯空间；右侧放：状态点 + 文案、刷新按钮。
- sidebar：220px（≤900px 时 204px，≤760px 收成 64px 图标栏）；`padding: 58px 10px 12px`；
  nav-label 11px/600 `--label-3`（如「工作台」「系统」）；nav-item 30px 高、圆角 7、图标 17px、
  右侧 kbd 提示 9.5px mono；active = `--accent` 底白字。
- 侧栏底部：mini-status（7px 状态点 + 11px 文案）、版本号 10px `--label-3`。
- 页头：`page-head` flex，左 h1 + page-sub，右 `page-head-side`（按钮/分段/未保存提示）；
  页头下 18px 接分组列表。**不要**旧的「01 / 04 索引 + 墨线」签名（那是 v2 的，已废弃）。

## 3. 分组列表（核心组件，直接复用 ykt 数值）

- `.group` 下边距 22；`.group-title` margin `0 12px 6px`，13px/500 `--label-2`；
  `.group-box` 背景 `--card`、圆角 10、`--card-shadow`、overflow hidden。
- `.row`：min-height 40、padding 5px 14px、gap 12，hover `--row-hover`；
  行间分隔用 `::before` 1px `--separator`（left 14px；带图标时 56px）。
- `.row-icon`：30px squircle 圆角 8、白图标 16px、内高光；
  语义色块：蓝 `#3b82f6`、绿 `#2fb45c`、橙 `#e8940f`、紫 `#9a6bd8`、靛 `#6c6bd9`、灰 `#8e8e93`。
- `.row-copy`：row-title 13px，可带 `row-title-extra` 11.5px `--label-2`；row-desc 11px `--label-2`。
- `.row-control`：右侧控件，gap 8；`.row-value` 12px `--label-2`。
- `.chips/.chip`：10px mono 标签；`.badge`：18px 高胶囊，语义色底 + 同色文字 + 5px 圆点
  （success/warning/danger/running）。
- `.step-row/.step-num`：22px 圆圈编号；完成态绿底白勾（用于刷课阶段、安装步骤）。
- `.settings-layout`：两栏 `1.05fr / 1fr`、gap 22，≤900px 单栏（设置页用）。

## 4. 控件（数值照抄 ykt）

- `.btn`：26px 高、圆角 6、字号 12.5、padding 0 11、hover/active/focus/disabled/busy 齐全；
  `.btn-sm` 24px；`.btn-primary` = `--accent` 底白字 500；`.btn-danger` = 红底白字；
  `.btn-danger-quiet` 红字；`.icon-btn` 24×24 圆角 5；`.link-btn` accent 文字 12px。
- `.switch`：38×22 胶囊，开启 `--green`，knob 18px，200ms 位移；`.switch-sm` 32×19。
- `.appearance-seg` + `.seg-btn`：inset 底分段控件，24px 高（复核过滤、可选参数用它）。
- `.input`：28px 高、圆角 6、12.5px；`.input.mono` 11.5px；focus 3px `--focus-ring`；
  `.select-wrap select` 28px + 自绘箭头；`.input-md` 220px、`.input-wide` 340px。
- `.icon-sprite` 内联 SVG + `<use>`；图标 18px、stroke 1.7、圆头圆角；`.icon-fill` 用于实心播放/停止。
- `--wails-draggable: no-drag` 必须加在所有交互控件上（button/input/select/switch/nav 等）。

## 5. 复合组件

- **日志卡**（与 ykt 运行日志页同款，我们的刷课运行日志直接复用）：
  `.log-card` 白卡 10px 圆角；`.log-card-head` 36px（live-dot + 计数 + 路径 mono 10px）；
  `.log-row` 三列 grid `62px 50px 1fr`、mono 11.5、行间 hairline、hover `--fill`；
  level：info accent、warn orange-text、error red-text、success green-text、debug label-3；
  `.empty-state`（42px 圆底图标 + 标题 13/500 + 说明 11 + 按钮）覆盖空态/错误态。
- **Toast**：底部居中胶囊，深色半透明 + blur，240ms 入场，3s 消失。
- **Modal**：320px 宽、12px 圆角、图标圈 + 标题 13.5/600 + 说明 11.5 + 右侧按钮；遮罩 18% 黑 + 1.5px blur。
- **Skeleton**：`.page-skeleton` + `.sk-line/.sk-box` shimmer 1.5s，首屏加载用。
- **About 头**：40px 圆角 9 图标 + 名称 14/600 + 版本 11 mono。

## 6. 页面映射（chaoxing 版）

- 侧栏：`工作台` → 概览(⌘1)/刷课(⌘2)/复核(⌘3)；`系统` → 设置(⌘4)。底部引擎状态点 + 版本。
- toolbar 右侧：引擎状态（sync-dot + 「引擎就绪/未就绪」）+「刷新」按钮（刷新当前页数据）。
- **概览**：h1 概览 / 副标题「当前账号 · N 个账号」；
  分组「运行状态」rows-iconed：上次运行（row-value mono）、账号数、今日复核、引擎（Python 版本）；
  分组「快捷操作」：开始刷课（btn-primary）、查看复核、打开数据目录；
  分组「最近复核」：5 行列表（kind + 课程 + 时间 mono + 状态 badge），空态用 empty-state；
  无数据时整页 skeleton。**不要**旧的大数字卡/渐变 hero。
- **刷课**：h1 刷课 / 右侧「刷新课程」「开始刷课」；
  分组「账号」：可选行（右侧对勾/单选，选中行 accent-soft 底）；
  分组「课程」：搜索 input + 全选/清空按钮 + 可滚动课程行（复选框自绘为圆形对勾）+ 计数 badge；
  分组「范围」：章节/任务中心 switch 行，两都关时给 group-note 警告并禁用开始；
  分组「参数」：速度 select（1.0×–2.0×）、并发 select（1–8）、每门上限 select（全部/1/2/3/5/10）、
  遇到关闭任务点 select（跳过/重试）、串行视频 switch、章节次数 switch+input、重做 switch+input；
  确认：modal 摘要 + 「开始」。
  运行中：分组「进度」用 step-row（登录/课程/章节/任务中心/收尾，is-done 绿勾）；
  状态行（阶段 badge + 已用时 mono + 停止按钮 btn-danger-quiet）；
  日志卡（同 §5，默认 320px 高、级别过滤用 seg、自动滚动 switch）；停止用 modal 二次确认。
  结果：分组「结果」：完成状态徽标 + 课程/错误/耗时 row-value；按钮「查看复核」「再刷一轮」。
- **复核**：page-wide；h1 复核 / 副标题「AI 生成并已提交给平台的文字」；
  右侧 seg 过滤（今天/近 3 天/全部）+「打开记录文件夹」；两栏 `1fr / 1.6fr`：
  左 group-box 列表行（kind + 课程 + 时间 mono + 状态 badge，选中 accent-soft 底）；
  右 group-box：about-head 式头部（kind + 课程 + 状态 + 复制按钮）+ 正文 15px/1.85 最大 660 + 底部路径 mono。
- **设置**：settings-layout 两栏；分组：账号（行 + 删除 danger-quiet + 新增按钮）、
  刷课参数（同参数行）、引擎与数据（仓库路径 mono、Python 版本、状态 badge、选择目录/重新检测、打开数据目录/config.ini）、
  关于（about 头 + 版本/协议/许可证）。新增账号与删除账号用 modal（370px 表单变体）。
- 兼容状态：启动骨架、无 Wails 运行时的兜底页、空态、错误态（带原因 + 重试）。

## 7. 文案（与 ykt 语气一致）

- 平静、具体、可执行；无感叹号；按钮/标签不加句号；不用「一键/智能/赋能/超级」。
- 复核页必须准确：AI 文字是**先提交后留痕**，文案只说明「已提交给平台、可回看/复制」，
  绝不暗示可提交前编辑。
- 数据必须真实；无数据显示「暂无记录」并说明来源。

## 8. 动效

- 页面入场 180ms fade+4px；hover/active 120–160ms；modal 180ms scale .98→1；toast 240ms。
- 唯一允许的循环：运行中状态点 breathing（语义 = 正在运行），源自 ykt 的 dot-pulse；
  非运行状态禁止脉冲。
- reduced-motion 全部关闭。禁止弹跳、位移 >4px、布局属性动画。

## 9. 验收清单

- [ ] 与 ykt 并排：字体/字号/行高/圆角/分隔线/控件高度一致，像同一产品的两个模块
- [ ] 无 v2 遗留：无「01 / 04」索引墨线、无大写数字卡、无旧渐变/发光/emoji
- [ ] 亮/暗两套 + 880×620/1280×800 不破版；侧栏毛玻璃在真机 .app 内正常
- [ ] 所有交互五态齐全；键盘 ⌘1–⌘4 可切换；focus-visible 可见
- [ ] 日志/空态/错误态/骨架 与 ykt 组件同款
- [ ] 无外链资源、无 console 报错、无默认控件外观
- [ ] 模板测试：把「超星助手」换成「雨课堂刷课助手」也不违和 → 设计语言统一成功

---

设计语言参考：yuketang-jiaoben/frontend/app.css（同作者项目，2026-11-04）；
方法论：unslop by Damir Sokolovsky（CC BY 4.0, https://github.com/heydamir/unslop）。
