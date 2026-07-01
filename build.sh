#!/usr/bin/env bash
set -euo pipefail

APP=infer-tester
SRC_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT_DIR="${SRC_DIR}/dist"

mkdir -p "$OUT_DIR"

targets=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

echo "Building ${APP}..."

for target in "${targets[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"
  EXT=""
  [[ "$OS" == "windows" ]] && EXT=".exe"
  OUT="${OUT_DIR}/${APP}-${OS}-${ARCH}${EXT}"

  printf "  %-20s -> %s\n" "${OS}/${ARCH}" "$OUT"
  GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT" "${SRC_DIR}/cmd/${APP}"
done

echo ""
echo "Copying supporting files..."
cp "${SRC_DIR}/config.json" "${OUT_DIR}/"
cp "${SRC_DIR}/USAGE.md" "${OUT_DIR}/"

echo ""
echo "Done:"
ls -lh "$OUT_DIR"
