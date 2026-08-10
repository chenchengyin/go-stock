#!/usr/bin/env bash
# 前台启动 Flutter API 服务（:8080），开发调试用。Ctrl-C 结束。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PORT="${GO_STOCK_API_PORT:-8080}"

if lsof -tiTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "端口 $PORT 已被占用，服务可能已在运行。先停掉再启动："
  echo "  lsof -tiTCP:$PORT -sTCP:LISTEN | xargs kill"
  exit 1
fi

echo "启动 Flutter API 服务 (端口 $PORT) ..."
exec go run ./cmd/server
