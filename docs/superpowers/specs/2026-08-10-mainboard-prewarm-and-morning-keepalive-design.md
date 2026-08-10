# 主板策略：00:00～09:00 主动预热 + 交易日 08:00 本机保活

## 背景

盘达「主板策略」依赖 T0 日线预热（`tryStartT0Prewarm` / `/api/t0-selection?prewarm=1`）。现有逻辑是：

- 有请求打进来、且早于当日 09:25 时，会自动走预热；
- **进程空闲时不会主动预热**。

服务跑在开发者本机 Mac（`cmd/server` → `:8080`）。若过夜没人访问、或早盘前服务没开，09:25 前后才会开始拉日线，影响策略可用性。

Cursor Automation 的定时任务跑在云端 Agent，**默认无法直接拉起本机进程**，因此启动保活与主动预热要拆职责。

## 目标

1. 服务进程在跑时：周一到周五、上海时区 **00:00～09:00**，主动触发当日 T0 日线预热（已缓存 / 正在预热则跳过）。
2. 本机周一到周五 **08:00**：若 `:8080` 未监听，用 `go run ./cmd/server` 拉起服务。
3. 提供仓库内快捷启动 / 保活脚本，并配 `launchd` plist 说明。
4. 一条 Cursor Automation：周一到周五 08:00 做检查 + 提醒 / 补救尝试；真正保活以本机 `launchd` 为准。

## 非目标

- 不做 A 股真实节假日日历（本轮按 **周一到周五** 近似交易日）。
- 不改选股过滤阈值与 09:25 竞价确认语义。
- 不把编译产物二进制作为默认启动方式（统一 `go run ./cmd/server`）。
- 不依赖云端 Agent 单独完成「本机必启」（云端只做巡检与补救尝试）。

## 决策摘要

| 项 | 选择 |
|----|------|
| 预热窗口 | 上海时区 00:00～09:00 |
| 交易日近似 | 周一到周五 |
| 启动方式 | `go run ./cmd/server` |
| 架构 | 方案 A：服务端定时预热 + 本机 launchd 保活 + Cursor 早检 |

## 1. 服务端主动预热

### 行为

在 `backend/flutter_api/server.go` 的 `Start()` 中新增 goroutine：

- Tick 间隔建议 **60s**（与现有市场快照采集一致即可）。
- 每个 tick 调用判定函数（可单测）：

```text
shouldAutoPrewarmT0(now) ==
  weekday ∈ {Mon..Fri}  AND
  上海时区 minutes ∈ [0, 9*60)  AND
  // 即 00:00 ≤ t < 09:00
```

- 若为真：`tradeDate = now.In(Asia/Shanghai).Format("2006-01-02")`，调用已有 `tryStartT0Prewarm(tradeDate)`。
- `tryStartT0Prewarm` 已保证：warming 中不重复启动、日线文件已存在则标 ready 且不重复拉。

### 边界

| 场景 | 行为 |
|------|------|
| 周末 | 不触发 |
| 09:00 整及之后 | 本定时器不触发；现有「请求 &lt; 09:25 自动预热」仍保留 |
| 服务 08:00 才启动 | 启动后第一个 tick 起，在 09:00 前仍会主动预热 |
| 当天已预热完 | `tryStartT0Prewarm` no-op |

### 日志

- 真正 `started==true` 时打 Info：`[定时任务] T0 日线主动预热已启动 date=...`
- 其余情况默认不刷屏（可 Debug）；失败路径沿用预热 job 现有日志。

### 测试

- 纯函数测试：`shouldAutoPrewarmT0` 覆盖 周日 / 周一 08:59 / 周一 09:00 / 周五 00:00 / 周六 01:00。

## 2. 快捷启动与保活脚本

新增：

| 路径 | 职责 |
|------|------|
| `scripts/start-flutter-api.sh` | 快捷启动：`cd` 到仓库根，`go run ./cmd/server`（前台，便于开发） |
| `scripts/ensure-flutter-api.sh` | 保活：探测 `127.0.0.1:8080`；已监听则退出 0；否则后台 `go run ./cmd/server`，等待就绪后写日志 |
| `scripts/launchd/com.colin.go-stock.flutter-api.plist` | 周一到周五 08:00 调 `ensure-flutter-api.sh` 的模板 |

### `ensure-flutter-api.sh` 要点

1. 解析仓库根（脚本相对路径）。
2. `curl -s -o /dev/null -w '%{http_code}' --max-time 2 http://127.0.0.1:8080/` 或对已知健康路径探测（若根路径无响应，可用任意已注册 API 的连通性 / TCP 探测 `nc -z`）。
3. 已就绪 → 日志「already running」→ exit 0。
4. 未就绪 → `nohup go run ./cmd/server >> "$LOG" 2>&1 &`，轮询最多约 60s。
5. 日志默认：`~/Library/Logs/go-stock-flutter-api.log`（或 `/tmp/go-stock-flutter-api.log`）。

### launchd

- 文档说明：`launchctl load` / `unload` 步骤；StartCalendarInterval 周一到周五 Hour=8 Minute=0。
- **本轮不强制替用户装入系统**；实现后在 README/规格旁注安装命令，用户确认后再装。

## 3. Cursor Automation（早检）

| 字段 | 值 |
|------|----|
| 名称 | 主板策略服务早检 |
| 描述 | 交易日 08:00 检查本机 Flutter API 是否在跑，必要时按仓库脚本补救并记录结果 |
| 触发 | 周一到周五 08:00（cron `0 8 * * 1-5`） |
| 指令 | 检查约定地址 `:8080`；若未启动则按 `scripts/ensure-flutter-api.sh` 意图尝试补救或明确报告失败；若已启动可 `curl` 一次 `?prewarm=1` 确认预热入口可达；结果写入 run 日志 |
| 真正保活 | 以本机 launchd + `ensure-flutter-api.sh` 为准 |

云端 Agent 若无法访问本机 `127.0.0.1`，Automation 应在 run 中写明「无法探测本机，依赖 launchd」——不假装成功。

## 4. 验收标准

1. 服务在周一 01:00 空闲运行时，无需 HTTP 请求也会出现主动预热日志，且当日 gob 缓存生成。
2. 周一 09:01 空闲时，定时器不再发起新的主动预热。
3. `scripts/start-flutter-api.sh` 可前台启动服务。
4. `scripts/ensure-flutter-api.sh`：已运行时二次执行不重复起第二个长期进程；未运行时能拉起并在日志可见。
5. Cursor Automation 草稿可在 Automations 编辑器中保存；早检 run 有可读结论。

## 5. 实现顺序建议

1. `shouldAutoPrewarmT0` + 单测 → 接入 `Start()` ticker。
2. `start-flutter-api.sh` + `ensure-flutter-api.sh` + plist 模板 + 简短使用说明。
3. 创建 Cursor Automation「主板策略服务早检」。
