# Flutter API 保活与股票池留档（launchd）

登录加载时、每 5 分钟，以及周一到周五 08:00 检查 Flutter API（`:8080`）是否在运行；未运行则用 `go run ./cmd/server` 拉起，已在运行立即退出。

## 安装

```bash
REPO_ROOT="$HOME/aiproject/go-stock"
PLIST="$HOME/Library/LaunchAgents/com.colin.go-stock.flutter-api.plist"

sed "s|__REPO_ROOT__|$REPO_ROOT|g" \
  "$REPO_ROOT/scripts/launchd/com.colin.go-stock.flutter-api.plist" > "$PLIST"

launchctl unload "$PLIST" 2>/dev/null
launchctl load "$PLIST"
```

## 验证

```bash
# 立即跑一次（不等 08:00）
launchctl start com.colin.go-stock.flutter-api

# 看日志
tail -n 50 ~/Library/Logs/go-stock-flutter-api.log
tail -n 50 /tmp/go-stock-flutter-api-launchd.log
```

也可以直接手动执行，行为与 launchd 调用一致：

```bash
./scripts/ensure-flutter-api.sh
```

## 卸载

```bash
launchctl unload "$HOME/Library/LaunchAgents/com.colin.go-stock.flutter-api.plist"
rm "$HOME/Library/LaunchAgents/com.colin.go-stock.flutter-api.plist"
```

## 说明

- `launchd` 的 PATH 很精简，`ensure-flutter-api.sh` 内部已补 `/usr/local/go/bin`、`/opt/homebrew/bin`。
- 睡眠中错过 08:00 时，`launchd` 会在唤醒后补跑一次。
- 服务进程自身在**周一到周五 00:00~09:00** 会主动预热当日 T0 日线；09:00 之后由请求触发预热（09:25 前）。
- 前台开发启动用 `./scripts/start-flutter-api.sh`。


## 股票池留档和历史审计

- 后台在工作日 00:00～15:05 独立采集当天完整股票池，失败后每 5 分钟重试；09:25 后启动也会留档。日线下载和前一交易日核验失败，不再丢弃已获取股票池。
- 独立文件在 `backend/data/cache/t0/pools/pool_YYYY-MM-DD.json`，包含来源、归档时间和完整性标记。共享日线 GOB 丢失其历史池时，可从独立文件恢复；冲突或损坏会明确报错，不静默覆盖。
- 历史下拉只展示有完整股票池或已确认正式结果的日期。价格日线存在并不证明当天股票池已保存。
- 离线审计及可信池留档：

```bash
go run ./cmd/t0-history-audit -cache backend/data/cache -recover-pools -output reports/t0-history-audit.json
```

不加 `-recover-pools` 只审计；加 `-require-complete` 时，只要仍有缺池或错误就返回退出码 2。旧缓存缺版本/完整标记、当前股票池和近似回测归档均不能冒充准确历史池。

launchd 只能在本机已登录、唤醒且联网时执行。整天关机或睡眠无法保证当日采集；持续在线的云服务器应运行同样的服务代码和数据留档机制。现有正式竞价提醒不变。
