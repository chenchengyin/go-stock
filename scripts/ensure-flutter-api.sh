#!/usr/bin/env bash
# 保活脚本：服务已在跑则直接退出，否则后台拉起 go run ./cmd/server。
# 交易日早盘前由 launchd 调用，也可手动执行。
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PORT="${GO_STOCK_API_PORT:-8080}"
LOG_DIR="${GO_STOCK_LOG_DIR:-$HOME/Library/Logs}"
LOG_FILE="$LOG_DIR/go-stock-flutter-api.log"
READY_TIMEOUT_SEC="${GO_STOCK_READY_TIMEOUT_SEC:-90}"

mkdir -p "$LOG_DIR"

log() {
  printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" | tee -a "$LOG_FILE"
}

port_listening() {
  lsof -tiTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1
}

if port_listening; then
  log "[ensure] 端口 $PORT 已在监听，服务运行中，跳过启动"
  exit 0
fi

# go 可能不在 launchd 的精简 PATH 里，补上常见安装位置。
export PATH="$PATH:/usr/local/go/bin:/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin"

if ! command -v go >/dev/null 2>&1; then
  log "[ensure] 未找到 go 命令，无法启动服务。请检查 PATH 或安装 Go"
  exit 1
fi

log "[ensure] 端口 $PORT 未监听，开始启动服务: go run ./cmd/server"
nohup go run ./cmd/server >>"$LOG_FILE" 2>&1 &
START_PID=$!
log "[ensure] 启动进程 pid=$START_PID，等待端口就绪（最长 ${READY_TIMEOUT_SEC}s）"

for ((i = 0; i < READY_TIMEOUT_SEC; i++)); do
  if port_listening; then
    log "[ensure] 服务已就绪，端口 $PORT 正在监听"
    exit 0
  fi
  if ! kill -0 "$START_PID" 2>/dev/null; then
    log "[ensure] 启动进程已退出且端口未就绪，详情见 $LOG_FILE"
    exit 1
  fi
  sleep 1
done

log "[ensure] 等待 ${READY_TIMEOUT_SEC}s 后端口仍未就绪，详情见 $LOG_FILE"
exit 1
