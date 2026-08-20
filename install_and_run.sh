#!/bin/bash
# 톡맨스 빌드 → /Users/lee/Applications 설치 → 실행
# usage: ./install_and_run.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="/Users/lee/Applications"
mkdir -p "$INSTALL_DIR"

echo "[1/4] xcodegen 프로젝트 생성..."
(cd "$ROOT/apps/macos" && xcodegen generate)

echo "[2/4] Debug 빌드..."
(cd "$ROOT/apps/macos" && xcodebuild -project Talkmance.xcodeproj -scheme Talkmance \
    -configuration Debug -derivedDataPath build/DerivedData build)

APP_SRC="$ROOT/apps/macos/build/DerivedData/Build/Products/Debug/Talkmance.app"
APP_DST="$INSTALL_DIR/Talkmance.app"

echo "[3/4] $APP_DST 로 설치..."
rm -rf "$APP_DST"
cp -Rf "$APP_SRC" "$APP_DST"

echo "[4/4] 실행..."
open "$APP_DST"
echo "설치 완료: $APP_DST"