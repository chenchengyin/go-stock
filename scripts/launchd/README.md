# 交易日早盘保活（launchd）

周一到周五 08:00 检查 Flutter API（`:8080`）是否在运行，未运行则用 `go run ./cmd/server` 拉起。

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
