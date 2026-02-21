#!/usr/bin/env bash

set -euo pipefail

OUT_DIR="${1:-dist/release}"
BIN_NAME="agent-manager"

mkdir -p "${OUT_DIR}"
rm -f "${OUT_DIR}/${BIN_NAME}-linux-amd64" \
      "${OUT_DIR}/${BIN_NAME}-windows-amd64.exe" \
      "${OUT_DIR}/checksums.txt"

echo "[release] building linux amd64 binary..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o "${OUT_DIR}/${BIN_NAME}-linux-amd64" ./cmd/session-manager

echo "[release] building windows amd64 binary..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o "${OUT_DIR}/${BIN_NAME}-windows-amd64.exe" ./cmd/session-manager

(
  cd "${OUT_DIR}"
  sha256sum "${BIN_NAME}-linux-amd64" "${BIN_NAME}-windows-amd64.exe" > checksums.txt
)

echo "[release] artifacts ready in ${OUT_DIR}"
