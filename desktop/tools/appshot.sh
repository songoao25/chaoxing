#!/usr/bin/env bash
# appshot.sh —— 启动 .app、截取应用窗口、再关闭（只截窗口，不截整个屏幕）
#
# 用法：
#   bash tools/appshot.sh [app 路径] [输出 png] [等待秒数]
# 默认：build/bin/chaoxing-desktop.app → /tmp/cx-app.png，等待 1.2s
#
# 说明：
# * 用 windowid 找窗口（只读窗口元数据），再用 screencapture -l 截窗口；
# * 截图需要系统"屏幕录制"权限（首次可能弹授权，仅授权一次即可）；
# * 截完用 SIGTERM 关闭应用，不弹确认框。

set -euo pipefail
cd "$(dirname "$0")/.."

APP="${1:-build/bin/chaoxing-desktop.app}"
OUT="${2:-/tmp/cx-app.png}"
WAIT="${3:-1.2}"

if [ ! -d "$APP" ]; then
  echo "找不到应用：$APP" >&2
  exit 1
fi

open "$APP"

WINDOW_ID=""
for _ in $(seq 1 40); do
  sleep 0.25
  WINDOW_ID="$(./tools/bin/windowid 超星助手 2>/dev/null | head -1 | cut -f1)"
  [ -n "$WINDOW_ID" ] && break
done

if [ -z "$WINDOW_ID" ]; then
  echo "没有等到窗口出现" >&2
  pkill -f "chaoxing-desktop.app/Contents/MacOS/chaoxing-desktop" 2>/dev/null || true
  exit 1
fi

sleep "$WAIT"
screencapture -x -o -l "$WINDOW_ID" "$OUT"
echo "$OUT"
pkill -f "chaoxing-desktop.app/Contents/MacOS/chaoxing-desktop" 2>/dev/null || true
