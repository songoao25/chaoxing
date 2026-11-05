#!/usr/bin/env bash
# 构建 macOS Sparkle 发布产物（本机 / CI 共用）。
#
# 用法：
#   SPARKLE_DIR=/path/to/sparkle-tools bash tools/package-release.sh desktop-v0.3.1
# 环境变量：
#   SPARKLE_DIR    Sparkle 发行包目录（含 Sparkle.framework 与 bin/generate_appcast）
#   SPARKLE_PRIVATE_KEY  EdDSA 私钥（CI 用；未设置时从 Keychain 读取）
#   IDENTITY       代码签名身份（默认 Apple Development: ...）
#   FEED_URL       Sparkle appcast 地址
#   OUT            产物目录（默认 desktop/dist/release）
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="$(sed -n 's/var appVersion = "\(.*\)"/\1/p' version.go)"
[ -n "$VERSION" ] || { echo "✖ 无法读取 version.go"; exit 1; }
TAG="${1:-desktop-v$VERSION}"
if [ "$TAG" != "desktop-v$VERSION" ]; then
  echo "✖ tag $TAG 与版本 $VERSION 不一致" >&2
  exit 1
fi
SPARKLE_DIR="${SPARKLE_DIR:?需要 SPARKLE_DIR 指向 Sparkle 发行包目录}"
FEED_URL="${FEED_URL:-https://github.com/songoao25/chaoxing/releases/latest/download/appcast.xml}"
OUT="${OUT:-$PWD/dist/release}"
WAILS_BIN="${WAILS_BIN:-$HOME/go/bin/wails}"
IDENTITY="${IDENTITY:-Apple Development: crabapplebandit@icloud.com (MG8SD67SC5)}"

echo "› 构建 v$VERSION 并嵌入 Sparkle"
"$WAILS_BIN" build >/dev/null
IDENTITY="$IDENTITY" bash tools/embed-sparkle.sh build/bin/chaoxing-desktop.app "$SPARKLE_DIR" "$FEED_URL"

rm -rf "$OUT"
mkdir -p "$OUT"
/usr/bin/ditto -c -k --keepParent build/bin/chaoxing-desktop.app "$OUT/chaoxing-desktop-$VERSION.zip"

echo "› 生成并签名 appcast"
URL_PREFIX="https://github.com/songoao25/chaoxing/releases/download/$TAG/"
if [ -n "${SPARKLE_PRIVATE_KEY:-}" ]; then
  printf '%s' "$SPARKLE_PRIVATE_KEY" | "$SPARKLE_DIR/bin/generate_appcast" --ed-key-file - \
    --download-url-prefix "$URL_PREFIX" "$OUT"
else
  "$SPARKLE_DIR/bin/generate_appcast" --download-url-prefix "$URL_PREFIX" "$OUT"
fi

(cd "$OUT" && shasum -a 256 ./*.zip > SHA256SUMS.txt)
echo "✔ 发布产物："
ls -lh "$OUT"
