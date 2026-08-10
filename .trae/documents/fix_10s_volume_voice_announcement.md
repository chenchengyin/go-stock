# 修复：10秒量能异动（9004）被语音播报的问题

## Summary

用户在异动设置页看不到、也取消不了「10秒量能异动（9004）」，但语音播报仍会播报该类型。原因是：

1. 本地监控引擎 `StockLocalMonitorConfig.rules` 中仍然启用了 9004 规则；
2. 语音播报触发路径 `_triggerVoiceAnnouncements` 没有按用户已选类型 `_selectedChangeTypes` 过滤，导致任何新生成的本地异动都会直接入队播报。

本计划将：
- 下架本地监控中的 9004 规则（与设置页保持一致）；
- 给语音播报增加类型过滤，确保只播报用户勾选监控的异动类型。

## Current State Analysis

### 关键文件与现状

1. `trading_app/lib/core/stock_local_monitor/stock_local_monitor_config.dart`
   - `rules` 列表第 50-65 行仍包含 `vol_10s`（9004），启用状态。
   - 该规则会由 `StockLocalMonitor.pushSnapshots` 在每次刷新时检测并产出 9004 异动。

2. `trading_app/lib/features/radar/presentation/radar_list/radar_view_model.dart`
   - `_filteredLocalAlerts`（第 63 行）会经过 `filterChanges`，因此 UI 列表里不会显示 9004。
   - 但 `_triggerVoiceAnnouncements`（第 377-383 行）直接遍历 `changes` 并回调，未做 `selectedChangeTypes` 过滤。
   - 调用链：
     - `_loadWatchChanges` / `_refreshMonitoredStocks` → `newLocalAlerts = _localMonitor.pushSnapshots(...)`
     - `_detectNewChanges(newLocalAlerts)`
     - `_triggerVoiceAnnouncements(newOnes)` → `onNewVoiceChange(change)`
   - 因此 9004 新异动会绕过用户设置直接播报。

3. `trading_app/lib/features/radar/domain/change_type_config.dart`
   - 第 50 行 `ChangeTypeItem(9004, '10秒量能异动')` 已注释，设置页确实不显示该类型。

4. `trading_app/lib/features/radar/domain/voice_announcement_view_model.dart`
   - `enqueueChange` / `enqueueChanges` 本身只负责入队和去重，不感知用户是否勾选该类型；过滤逻辑应在调用方（RadarViewModel）。

## Proposed Changes

### 1. 下架 9004 本地监控规则

文件：`trading_app/lib/core/stock_local_monitor/stock_local_monitor_config.dart`

操作：把 `vol_10s` 规则整体注释掉（与 10 秒价格规则 9001 的处理方式一致）。

```dart
// WaveRuleConfig(
//   id: 'vol_10s',
//   name: '10秒量能异动',
//   changeType: 9004,
//   checkType: CheckType.volume,
//   windowSize: 1,
//   cooldownSec: 60,
//   thresholds: [
//     Threshold('爆量3倍', 3),
//     Threshold('爆量5倍', 5),
//     Threshold('爆量10倍', 10),
//     Threshold('缩量0.3倍', 0.3),
//     Threshold('缩量0.1倍', 0.1),
//     Threshold('缩量0.05倍', 0.05),
//   ],
// ),
```

### 2. 语音播报增加类型过滤

文件：`trading_app/lib/features/radar/presentation/radar_list/radar_view_model.dart`

操作：在 `_triggerVoiceAnnouncements` 中只播报 `_selectedChangeTypes` 包含的类型。

```dart
void _triggerVoiceAnnouncements(List<StockChange> changes) {
  final callback = onNewVoiceChange;
  if (callback == null || changes.isEmpty) return;
  for (final change in changes) {
    if (!_selectedChangeTypes.contains(change.changeType)) continue;
    callback(change, urgent: _isUrgentVoiceChange(change));
  }
}
```

这样即使用户以后在设置页取消其他本地类型（如 9005/9006），语音播报也会同步遵守，避免"设置里关了却还在播报"的类似问题。

## Assumptions & Decisions

- 9004 已从 UI 下架，用户无法手动开关，因此本地规则也应同步下架，而不是仅做过滤。
- 过滤放在 `RadarViewModel._triggerVoiceAnnouncements`，而不是 `VoiceAnnouncementViewModel`，因为后者是通用播报队列，不应感知业务筛选；`RadarViewModel` 已经持有 `_selectedChangeTypes`，是放置过滤的最自然位置。
- 服务端异动同样会走 `_triggerVoiceAnnouncements`，增加过滤后也会遵守用户设置，行为一致。

## Verification Steps

1. 重新编译/热重载 Flutter 应用。
2. 进入「监控」Tab，确认设置页没有「10秒量能异动」。
3. 开启语音播报，等待本地监控刷新。
4. 观察：不再出现 9004 类型的语音播报；已勾选的 9002/9003 等类型正常播报。
5. 在设置页临时取消 9002/9003，确认对应类型也不再播报。
