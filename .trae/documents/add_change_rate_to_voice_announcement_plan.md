# 异动语音播报追加当前涨幅 - 实施计划

## 1. 摘要

当前语音播报的文本由 `VoiceAnnouncementViewModel._buildSpeechText` 组装，分为两种情况：

- `description` 为空时走 `_buildDefaultDesc`，会带上 `changeRate`；
- `description` 不为空时直接使用原始描述，**不一定包含涨幅**，导致部分异动播报缺少涨幅信息。

本计划在所有异动语音播报的末尾统一追加 "当前涨/跌 X.XX%"，确保用户听到的每条异动都带涨幅。

## 2. 当前状态分析

### 2.1 关键文件

- `trading_app/lib/features/radar/domain/voice_announcement_view_model.dart`
  - 第 362~367 行：`_buildSpeechText` 负责组装播报文本。
  - 第 369~374 行：`_buildDefaultDesc` 在 description 为空时使用，已包含 `changeRate`。
  - 第 376~399 行：`_simplifyDescription` 对文本做 TTS 友好处理（如 `%` -> `个点`）。

- `trading_app/lib/features/radar/domain/radar_models.dart`
  - `StockChange.changeRate` 字段：double，表示异动发生时的涨跌幅，单位 %。

- `trading_app/lib/core/stock_local_monitor/stock_local_monitor.dart`
  - 本地监控产生的 `StockChange` 中，`description` 形如 `10秒急速波动 急涨 1.23% [爆量3倍]`，`changeRate` 为窗口内价格变化率。

- `trading_app/lib/shared/widgets/stock_change_card.dart`
  - UI 卡片中涨幅展示为 `${isUp ? "+" : ""}${change.changeRate.toStringAsFixed(2)}%`。

### 2.2 问题点

`_buildSpeechText` 当 `description` 不为空时，直接返回 `$name，$desc`，不再追加涨幅。例如服务端返回的异动若描述里没有涨幅，语音里就听不到。

## 3. 拟议变更

### 3.1 修改 `voice_announcement_view_model.dart` 的 `_buildSpeechText`

将：

```dart
String _buildSpeechText(StockChange change) {
  final name = change.stockName.isNotEmpty ? change.stockName : change.stockCode;
  final desc = _simplifyDescription(change.description ?? _buildDefaultDesc(change));
  return '$name，$desc';
}
```

改为：

```dart
String _buildSpeechText(StockChange change) {
  final name = change.stockName.isNotEmpty ? change.stockName : change.stockCode;
  final desc = _simplifyDescription(change.description ?? _buildDefaultDesc(change));
  final rate = change.changeRate;
  final rateStr = rate >= 0 ? '涨${rate.toStringAsFixed(2)}%' : '跌${rate.abs().toStringAsFixed(2)}%';
  return '$name，$desc，当前$rateStr';
}
```

### 3.2 说明

- `changeRate` 在 `StockChange` 中始终存在（本地监控和服务端都会填充），无需额外查询行情。
- 追加部分会经过 `_simplifyDescription` 之后的文本处理，其中 `%` 会被替换为 `个点`，因此 TTS 实际读出为 "当前涨 2.35 个点"，符合中文语音习惯。
- 当 `description` 本身已含涨幅时，会在末尾再次追加，形成冗余但信息一致；考虑到用户明确要求 "最后都加上当前涨幅"，统一追加比重度解析 description 更稳定可靠。

## 4. 假设与决策

- **假设**：用户所说的 "当前涨幅" 指的是异动记录中的 `changeRate`（异动发生时的涨跌幅），而非播报时刻的实时最新涨幅。若需要实时最新涨幅，需额外接入行情推送并修改 `StockChange` 结构，本计划不包含。
- **决策**：不在 `_simplifyDescription` 中特殊处理最后的涨幅符号，保持现有 `%` -> `个点` 映射，实现最小改动。

## 5. 验证步骤

1. 修改后运行 `flutter analyze` 检查无静态错误。
2. 在 `VoiceAnnouncementViewModel.playTestChanges` 中测试数据包含正负 `changeRate`，手动触发测试播报，确认语音末尾包含 "当前涨 X.XX 个点" 或 "当前跌 X.XX 个点"。
3. （可选）在真实运行中触发一条本地或服务端异动，验证播报内容末尾带涨幅。
