# 对话日志

## 2026-07-03 本地监控机制修复 & 优化

### 一、问题：本地异动"一会儿有，一会儿没"

**现象**: 英杰电气、苏州天脉在监控列表显示异动标签，但过一会儿就消失了。

**根因**: `_refreshData()` 每 10 秒执行 `watchChanges = filtered`，只用服务端数据直接覆盖，之前触发的本地异动全部丢失。本地监控有冷却机制（30~120秒），冷却期内不触发新异动，导致旧异动消失后无法恢复。

**修复 (radar_view_model.dart)**:
1. 新增 `_localAlertsToday` 当日异动池，跨刷新保留
2. 新增 `_mergeLocalAlerts` 去重合并方法
3. `_refreshData`/`loadMonitoredStocks`/`loadWatchChanges` 合并服务端 + 本地异动池
4. 修复 `_detectNewChanges` 只对真正新异动触发通知（避免重复通知）
5. `removeMonitoredStock` 清理对应股票的本地异动

---

### 二、问题：详情页看不到本地异动

**现象**: 盛美上海监控列表有异动标签，但点进去详情页看不到。

**根因**: `filterChanges` 按 `_selectedChangeTypes` 过滤，本地异动 type 9001~9005 不在默认选中类型中，被过滤掉。

**修复**: 将本地监控类型加入 `ChangeTypeConfig` 的 `allTypes` 和 `defaultMonitorIds`，异动设置页自然可勾选/取消。
- `change_type_config.dart`: 新增 5 个本地异动类型（9001~9005）
- `radar_view_model.dart` / `monitor_settings_page.dart`: 老用户一次性迁移

---

### 三、完整逻辑梳理发现的漏洞

**Bug 1（严重）：成交额计算完全错误**
- 文件: `stock_local_monitor.dart` `_check()` 方法
- 原逻辑: 把 `amount`（当日累计成交额）当增量累加，多个累计值相加无意义
- 修复: 用差值法，`curSum = now.amount - then.amount`, `prevSum = then.amount - beforeThen.amount`

**Bug 2（中等）：`_localAlertsToday` 跨天不清理**
- 修复: `_refreshData` 开头对 `_localAlertsToday` 执行 `filterTodayChanges`

**Bug 3（小）：去重精度不够**
- 修复: 去重从 `stockCode+changeType+changeTime`（秒级）改为 `id`（毫秒级）

---

### 四、UI 增强：监控列表条目加异动描述
- `radar_view_model.dart`: 新增 `getLatestAlertDescription(code)` 方法
- `radar_page.dart`: 股票卡片末尾新增一行，显示最新异动描述（橙色警告图标 + 文字，超长省略号）

---

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `stock_local_monitor.dart` | 成交额计算从累加改为差值法 |
| `change_type_config.dart` | 新增 9001~9005 本地异动类型 |
| `radar_view_model.dart` | 新增 `_localAlertsToday` 池、跨天清理、去重优化、`getLatestAlertDescription` |
| `monitor_settings_page.dart` | 老用户一次性迁移逻辑 |
| `radar_page.dart` | 监控列表卡片末尾加异动描述行 |

---

## 2026-07-07 语音播报功能实现

### 需求
1. 监控 Tab 页面右上角添加喇叭按钮，点击进入管理页面。
2. 自选股出现新异动时自动语音播报。
3. 实现播报队列管理，防止过多新异动导致播报混乱。
4. 紧急异动（价格急速波动 9001/9002）插到当前播放的下一条，不打断当前播报。
5. 首页提前进行授权，统一弹窗提示用户开启语音播报。
6. 自动简化播报文本（如 `%` 读作 `百分之`、`爆量` 读作 `成交量放大` 等）。
7. 不需要与服务端同步，本地处理。
8. 避免内存泄漏。

### 实现

#### 1. 新增 `voice_announcement_view_model.dart`
- 封装 `FlutterTts`，负责 TTS 初始化、播报、停止。
- 维护播报队列 `_queue`，提供 `enqueueChange` / `enqueueChanges`。
- 紧急类型通过 `urgent=true` 插入队列第 1 位（当前播放下一条）。
- 使用 `_announcedChangeIds` 去重，避免同一条异动反复播报。
- 文本简化：`%`→`百分之`、`+`→`涨`、`急涨`→`急速上涨`、`爆量`→`成交量放大` 等。
- 持久化：使用 `SharedPreferences` 保存 `enabled` 和 `askedBefore`。
- 内存管理：`dispose` 时停止 TTS 并解绑。

#### 2. 新增 `voice_manager_page.dart`
- 显示播报开关、当前正在播报内容、待播报队列。
- 提供「停止播报」和「清空队列」按钮。
- 队列按入队顺序展示，显示入队时间。

#### 3. 修改 `radar_page.dart`
- AppBar 右上角添加喇叭按钮，根据 `enabled` / `speaking` 状态切换图标颜色。
- `initState` 中绑定 `RadarViewModel.onNewVoiceChange` 到 `VoiceAnnouncementViewModel.enqueueChange`。
- `dispose` 中解绑回调，避免内存泄漏。
- 构建完成后检查 `askedBefore`，未询问则弹出授权申请弹窗。

#### 4. 修改 `radar_view_model.dart`
- 新增 `onNewVoiceChange` 回调。
- `_detectNewChanges` 检测到新异动后，逐个触发语音播报。
- 紧急类型判定：本地价格急速波动（`changeType == 9001 || 9002`）标记 `urgent=true`。

#### 5. 修改 `app_config.dart`
- 注册 `VoiceAnnouncementViewModel` Provider。

#### 6. 修改 `pubspec.yaml`
- 添加依赖 `flutter_tts: ^4.0.2`。

### 验证
- `flutter analyze` 通过（语音播报相关文件无 lint 错误）。
- `flutter build web` 编译成功。

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `voice_announcement_view_model.dart` | 新增：TTS 初始化、队列管理、紧急插队、文本简化、持久化开关 |
| `voice_manager_page.dart` | 新增：播报开关、当前播报、队列展示、停止/清空按钮 |
| `radar_page.dart` | 添加喇叭按钮、授权弹窗、绑定/解绑语音回调 |
| `radar_view_model.dart` | 新增 `onNewVoiceChange` 回调，新异动触发语音入队 |
| `app_config.dart` | 注册 `VoiceAnnouncementViewModel` |
| `pubspec.yaml` | 添加 `flutter_tts` 依赖 |

---

## 2026-07-07 语音播报去重优化

### 需求
已播报过的异动不要重复播报。

### 实现
- `voice_announcement_view_model.dart`：
  - 已播报异动 ID 持久化到 `SharedPreferences`（key: `voice_announced_ids`）。
  - 按日期存储，跨天自动清空旧记录，避免历史 ID 无限增长。
  - `enqueueChange` 时先去重，成功入队后立即保存。
  - `clearAnnouncedHistory` 同时清除内存与本地持久化记录。

### 验证
- `flutter analyze` 通过。
- `flutter build web --no-wasm-dry-run` 编译成功。

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `voice_announcement_view_model.dart` | 已播报 ID 持久化、按天清理、入队即保存 |

---

## 2026-07-07 添加语音播报启动测试

### 需求
程序一启动到首页之后就开始播放当前位置的异动，用于测试语音播报功能。

### 实现
- `voice_announcement_view_model.dart`：新增 `playTestChanges()` 方法，构造 3 条模拟异动（盛美上海、英杰电气、睿能科技），调用正常入队逻辑播放。
- `radar_page.dart`：在 `initState` 中延迟 4 秒调用一次 `playTestChanges()`，等待授权弹窗处理与 TTS 初始化完成。
- 测试数据使用特殊负 ID，避免污染真实异动的已播报去重记录。

### 验证
- `flutter analyze` 通过（测试相关代码无 lint 错误）。
- `flutter build web --no-wasm-dry-run` 编译成功。

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `voice_announcement_view_model.dart` | 新增 `playTestChanges()` 测试方法 |
| `radar_page.dart` | `initState` 延迟触发一次测试播放 |

---

## 2026-07-07 语音播报支持语速调节

### 需求
可以调整语音播报速度。

### 实现
- `voice_announcement_view_model.dart`：
  - 新增 `_speechRate` 状态，默认 `0.55`，范围 `0.3 ~ 1.5`。
  - 持久化到 `SharedPreferences`（key: `voice_announcement_speech_rate`）。
  - TTS 初始化与设置语速时都使用 `_speechRate`。
  - 新增 `setSpeechRate(double)` 方法，拖动滑块时实时生效。
- `voice_manager_page.dart`：
  - 新增「播报语速」卡片，带 Slider 滑块和当前数值显示。
  - 范围 0.3（慢）~ 1.5（快），共 12 档。

### 验证
- `flutter analyze` 通过。
- `flutter build web --no-wasm-dry-run` 编译成功。

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `voice_announcement_view_model.dart` | 语速状态、持久化、`setSpeechRate` 方法 |
| `voice_manager_page.dart` | 新增语速调节滑块卡片 |

---

## 2026-07-07 语音播报增加已播放队列

### 需求
- 播放管理页面待播放列表下方增加已播放队列。
- 未播放列表里的异动播放完成后进入已播放队列。
- 清掉已播放数据，重新启动测试。

### 实现
- `voice_announcement_view_model.dart`：
  - 新增 `_spokenQueue` 列表与 `spokenQueue` getter。
  - `_processQueue` 保存当前播报项 `_currentItem`。
  - `_onSpeakComplete` 将完成项插入已播放队列（最新在前）。
  - `stopSpeaking` 时丢弃当前未完成的播报项。
  - 新增 `clearSpokenQueue()` 方法。
  - `playTestChanges()` 开头清空 `_announcedChangeIds`、`_spokenQueue` 和本地持久化，确保测试每次都重新播放。
- `voice_manager_page.dart`：
  - 待播报队列下方新增「已播报队列」区域。
  - 显示已播报内容、数量、清空按钮。
  - 已播报条目带灰色对勾图标，视觉上与待播报区分。

### 验证
- `flutter analyze` 通过。
- `flutter build web --no-wasm-dry-run` 编译成功。

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `voice_announcement_view_model.dart` | 已播放队列、播放完成归档、清空方法、测试前重置 |
| `voice_manager_page.dart` | 已播报队列 UI、清空按钮 |

---