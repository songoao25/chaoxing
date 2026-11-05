# 超星助手（chaoxing-desktop）

超星/学习通刷课脚本的桌面图形界面。Go + [Wails v2](https://wails.io) 外壳，复用本仓库已有的
Python 引擎（`main.py` / `api/`），不重写任何刷课逻辑。

> 这是本仓库 `desktop/` 目录的说明。CLI 本身的使用方法见[仓库根 README](../README.md)。

## 设计语言

界面与同作者的「雨课助手」（`yuketang-jiaoben`）**共用同一套 macOS 系统设置风格设计语言**：
纯白底黑字 / 纯黑底白字的双主题、隐藏式标题栏 + 毛玻璃侧栏、分组卡片列表（`group/group-box/row`）、SF 系统字体与语义色、
ykt 同款控件（开关/分段/下拉/徽标/日志卡/toast/modal/骨架屏）。两个 App 的设计令牌与组件类名
同名同值，并排看是同一产品的两个模块。

- 视觉规范：[docs/DESIGN.md](docs/DESIGN.md)（含令牌、组件、页面映射、unslop 验收清单）
- 参考实现（只读）：`../yuketang-jiaoben/frontend/app.css`（同作者项目）
- 方法论：[unslop](https://github.com/heydamir/unslop)（Damir Sokolovsky，CC BY 4.0）——
  安静 chrome、单一强调色、真实内容承载视觉、无渐变/发光/emoji。

## 功能（v1）

- **概览**：引擎状态、账号、今日复核数量、最近复核。
- **刷课**：选账号 → 拉取课程列表勾选 → 选范围（章节 / 任务中心）与参数 → 一键开刷；
  运行中实时阶段、计时、可随时停止；结束后结果摘要。
- **复核**：翻阅 AI 生成并已提交的实质性文字（`~/.chaoxing/reviews/`）。
- **设置**：账号管理、常用参数、引擎路径探测、打开数据目录/配置文件。

v1 不支持：讨论区挑帖（仍用 `./cx discuss`）、题库人工答题模式、逐次提交确认
（GUI 固定 `task_center_submit_mode=auto`，启动确认由应用自己的弹窗代替）。

## 运行要求

- macOS 13+（本机验证 macOS 15 / Apple Silicon）。
- Python 3.13+ 及仓库依赖：优先使用仓库下的 `.venv`；没有时用 PATH 里的 `python3`。
  引擎路径可在「设置 → 引擎」里手动指定。
- 只读取/复用 `~/.chaoxing/` 下的现有配置、账号和复核记录，GUI 不额外保存密码。

## 安装与自动更新

```bash
# 构建并安装到 /Applications（构建产物本身不进 git，所以 clone 下来不会自带 .app）
make desktop-install
```

- 安装位置：`/Applications/超星助手.app`（Finder/Launchpad/Spotlight 可见）。
- 首次打开会显示 5 步新手教程（确认运行环境 → 添加账号 → 开始刷课 → 查看复核），
  零基础可照着点；完成或跳过后不再打扰，之后可在「设置 → 帮助 → 新手教程」重新打开。
- 自动更新：应用启动时会静默检查 GitHub Releases；发现新版本会在设置页「版本更新」提示，
  用户点「下载并安装」后自动下载 → sha256 校验 → 校验 bundle id/版本/代码签名 → 替换 → 重启。
  安装必须由用户点击触发；启动检查失败不会打扰使用。可用 `CX_NO_UPDATE_CHECK=1` 关闭启动检查。
- 更新源：本仓库 GitHub Releases（tag 前缀 `desktop-v`）。发版流程：
  1. 把 `desktop/version.go` 的 `appVersion` 与 `wails.json` 的 `info.productVersion` 改成同一版本；
  2. `go test ./...` 通过后提交；
  3. `git tag desktop-vX.Y.Z && git push origin desktop-vX.Y.Z`；
  4. CI（`.github/workflows/desktop-release.yml`）自动构建 `chaoxing-desktop-macos-arm64.zip` 并发 Release。
- 限制：更新包只覆盖 App 本身（Go/Wails 外壳 + 前端）；Python 引擎与 `~/.chaoxing` 数据不受影响。
  当前只构建 Apple Silicon（arm64）包，Intel Mac 会提示没有对应安装包。

## 开发与构建

```bash
# 构建 .app（不需要 Xcode，Command Line Tools 即可；不需要 npm）
cd desktop
~/go/bin/wails build            # 产物：build/bin/chaoxing-desktop.app

# 无头自检（不打开窗口；mock 模式不需要 Python 引擎）
CX_DESKTOP_MOCK=1 ./build/bin/chaoxing-desktop.app/Contents/MacOS/chaoxing-desktop --self-test

# Go 侧检查
go vet ./... && go test ./...

# 前端开发：起静态服务器用浏览器 mock 调试
cd frontend && python3 -m http.server 8123
# 打开 http://127.0.0.1:8123/dev/

# 截图验收（WKWebView 无头截图，和 .app 同渲染引擎）
bash tools/build.sh
tools/bin/webshot http://127.0.0.1:8123/dev/ /tmp/ui.png 1000 720 --dark
```

前端是**无构建步骤的静态资源**：`frontend/dist/` 直接入库并由 Go 的 `embed` 打包，
`wails.json` 里的 `frontend:install` / `frontend:build` 为空。改完前端重新 `wails build` 即可。

## 目录结构

```
desktop/
├── main.go / app.go / bridge.go / engine.go / models.go    # Wails 外壳与 Python 桥客户端
├── build/                      # 图标与平台打包资源（Info.plist 等）
├── frontend/dist/              # 生产前端（入库，静态资源）
├── frontend/dev/               # 浏览器开发用 mock（不打包）
├── docs/INTERFACE.md           # Python 桥 ⇄ Go ⇄ 前端 三端接口契约
└── tools/                      # 开发辅助：无头截图 webshot、图标生成 makeicon
```

## 架构

```
前端（WKWebView，window.go / window.runtime）
   ↕ Wails 绑定与事件
Go 后端（App 结构体，进程管理 + 引擎探测）
   ↕ NDJSON over stdin/stdout
Python 桥 api/desktop_bridge.py（进程内调用 main.py，复用 api/*）
   ↕
现有引擎与 ~/.chaoxing 数据
```

- 协议细节、字段、错误码、事件名以 [`docs/INTERFACE.md`](docs/INTERFACE.md) 为准。
- 刷课任务在 Python 桥的独立线程里跑，停止走 `api.interrupt.request_stop()`；
  GUI 模式禁止任何 `input()` 交互，遇到需要交互的配置会明确报错而不是挂住。
- 跨平台：Go/Wails 与前端本身是跨平台的（Windows/Linux 需各自装 Python 与依赖），
  v1 只在 macOS 上构建与验证。

## 测试

```bash
# Python 桥离线测试（临时 CX_DATA_HOME，不碰真实数据、不联网）
CX_DATA_HOME=$(mktemp -d) ../.venv/bin/python -m unittest tests.test_desktop_bridge -v

# 全仓库回归（366+ 用例）
cd .. && make test

# Go 测试
cd desktop && go test ./...
```

## 许可

GPL-3.0，与上游 `Samueli924/chaoxing` 一致。本目录仅为个人 fork 的桌面外壳，
不改变引擎行为。
