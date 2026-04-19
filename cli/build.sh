#!/usr/bin/env bash
# Build aico for macOS (arm64 + amd64) and Windows (amd64)
set -euo pipefail

BINARY="aico"
SRC="./src"
OUT="./bin"

mkdir -p "$OUT"

echo "==> Building $BINARY..."

# macOS Apple Silicon
echo "    darwin/arm64..."
GOOS=darwin GOARCH=arm64 go build -o "$OUT/${BINARY}-darwin-arm64" "$SRC"

# Windows
echo "    windows/amd64..."
GOOS=windows GOARCH=amd64 go build -o "$OUT/${BINARY}-windows-amd64.exe" "$SRC"

# Symlink / copy current-platform binary to plain "aico" (macOS only in CI)
CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)
if [[ "$CURRENT_ARCH" == "arm64" ]]; then
    NATIVE="${BINARY}-${CURRENT_OS}-arm64"
else
    NATIVE="${BINARY}-${CURRENT_OS}-amd64"
fi

if [[ -f "$OUT/$NATIVE" ]]; then
    cp "$OUT/$NATIVE" "$OUT/$BINARY"
    echo "==> Copied $NATIVE → $OUT/$BINARY (native)"
fi

echo ""
echo "Done. Artifacts in $OUT/:"
ls -lh "$OUT/"
