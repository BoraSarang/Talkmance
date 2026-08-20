#!/bin/bash
# build-dmg.sh — macOS DMG 빌드 (T-40)
# 사용법: ./scripts/build-dmg.sh [버전]
set -euo pipefail

VERSION="${1:-1.0.0}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_DIR="$ROOT/apps/macos"
DERIVED="$APP_DIR/build/DerivedData"
DMG_NAME="Talkmance-${VERSION}.dmg"
DIST_DIR="$APP_DIR/dist"

echo "[DMG] xcodebuild Release 빌드 (version=$VERSION)"
xcodebuild -project "$APP_DIR/Talkmance.xcodeproj" \
  -scheme Talkmance -configuration Release \
  -derivedDataPath "$DERIVED" \
  CODE_SIGNING_ALLOWED=NO build

APP_PATH="$DERIVED/Build/Products/Release/Talkmance.app"
if [ ! -d "$APP_PATH" ]; then
  echo "[DMG] 빌드 산출물 없음: $APP_PATH" >&2
  exit 1
fi

mkdir -p "$DIST_DIR" "$DIST_DIR/stage"
rm -rf "$DIST_DIR/stage/Talkmance.app" "$DIST_DIR/stage/Applications"
cp -R "$APP_PATH" "$DIST_DIR/stage/Talkmance.app"
ln -s /Applications "$DIST_DIR/stage/Applications"

echo "[DMG] hdiutil로 DMG 생성"
hdiutil create -volname "톡맨스 Talkmance" \
  -srcfolder "$DIST_DIR/stage" \
  -ov -format UDZO \
  "$DIST_DIR/$DMG_NAME"

rm -rf "$DIST_DIR/stage"
echo "[DMG] 완료: $DIST_DIR/$DMG_NAME"
ls -lh "$DIST_DIR/$DMG_NAME"