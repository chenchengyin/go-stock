#!/usr/bin/env bash
# Ubuntu 远端：重启 Go 服务（:8080，同时托管 API + 已构建好的 Flutter Web）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PORT="${GO_STOCK_API_PORT:-8080}"
LOG="${GO_STOCK_LOG_DIR:-$HOME/logs/go-stock}/server.log"

export PATH="$PATH:/usr/local/go/bin:/usr/lib/go/bin:$HOME/go/bin"

mkdir -p "$(dirname "$LOG")"

# 停掉占用端口的旧进程
if command -v fuser >/dev/null 2>&1; then
  fuser -k "${PORT}/tcp" 2>/dev/null || true
elif command -v lsof >/dev/null 2>&1; then
  lsof -tiTCP:"$PORT" -sTCP:LISTEN | xargs -r kill 2>/dev/null || true
fi
sleep 1

echo "启动服务 :$PORT → $LOG"
nohup go run ./cmd/server >>"$LOG" 2>&1 &
echo "pid=$!  查看日志: tail -f $LOG"
