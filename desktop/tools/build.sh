#!/usr/bin/env bash
# 构建桌面端开发辅助工具（截图/窗口定位）。
# 需要 macOS Command Line Tools（swiftc），不需要 Xcode。
# 产物在 desktop/tools/bin/（已 gitignore），不随 .app 分发。
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p bin

swiftc -O -o bin/webshot webshot.swift -framework AppKit -framework WebKit
echo "built tools/bin/webshot"

swiftc -O -o bin/makeicon makeicon.swift -framework AppKit
echo "built tools/bin/makeicon"

if [ -f windowid.swift ]; then
  swiftc -O -o bin/windowid windowid.swift -framework CoreGraphics
  echo "built tools/bin/windowid"
fi
