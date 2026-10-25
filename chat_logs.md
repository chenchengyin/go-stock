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

## 2026-07-07 语音播报增加重新测试按钮

### 需求
清掉已播放数据，重新启动测试。

### 实现
- `voice_announcement_view_model.dart`：
  - 新增 `restartTest()` 方法：先 `stopSpeaking()` 停止当前播报并清空待播队列，再调用 `playTestChanges()` 重新播放测试数据。
  - `playTestChanges()` 拆出 `_enqueueTestChanges()`，职责更清晰。
- `voice_manager_page.dart`：
  - 操作按钮区增加「重新测试」按钮（绿色刷新图标）。
  - 点击后清空已播报记录、已播放队列、待播报队列，并重新播放 3 条测试异动。

### 验证
- `flutter analyze` 通过。
- `flutter build web --no-wasm-dry-run` 编译成功。

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `voice_announcement_view_model.dart` | 新增 `restartTest()` 方法 |
| `voice_manager_page.dart` | 增加「重新测试」按钮 |

---

## 2026-07-07 注释语音播报测试入口

### 需求
注释重新测试按钮和首页启动自动播报的测试逻辑，不删代码。

### 实现
- `voice_manager_page.dart`：注释「重新测试」按钮代码，保留 `restartTest()` 调用逻辑。
- `radar_page.dart`：注释 `initState` 中 `_playTestVoiceAnnouncementOnce()` 的调用，保留方法体并加 `// ignore: unused_element` 避免 lint 警告。

### 验证
- `flutter analyze` 通过（仅余 3 个已有问题）。
- `flutter build web --no-wasm-dry-run` 编译成功。

### 修改文件清单

| 文件 | 变更 |
|---|---|
| `voice_manager_page.dart` | 注释重新测试按钮 |
| `radar_page.dart` | 注释首页启动自动测试播放 |

---

## 2026-10-25 红策买卖提醒与防随手单需求讨论

### 用户

我做自己红策以外的事情 吃了一次大亏 我想让这个软件帮我防止一下 我下次再犯错 有什么好办法吗？

我想要最好能够及时地提醒到我我红策要买入的个股，需要卖出的时候也要及时提醒我。根据市场的整体环境要给予我到位的分析。最好能够做到防止我开随手单。

### 补充信息

助手询问实际下单使用哪个券商或交易软件、主要在手机还是电脑；用户回答：手机。

### 只读核查结果

- 现有红策竞价提醒主要表示当天名单确认，不等于本人实际入场条件、仓位和价格均已满足。
- 部分卖出监控来自特定形态归档，尚未完整关联本人实际成交、持仓数量和可卖数量。
- 已有短线情绪分析，可复用涨跌家数、涨跌停、炸板、量能和主线等数据；情绪评分本身不能证明市场过滤能提高红策收益。
- 当前未发现生产券商订单通道，软件不能直接阻止用户在手机券商 App 中下单。

### 建议与待确认项

建议讨论“红策纪律模式”：盘前固定本人选用的规则和交易计划；符合明确入场条件才推送手机买入提醒；真实成交录入后按可卖日期、可卖数量和事先约定的退出条件跟踪；市场分析附事实、数据时间和计划影响；计划外买入、超预算、临时放宽规则等触发强提醒与冷静步骤，规则放宽延至下一交易日生效，不妨碍退出。

第一版优先补齐手机消息送达核验、真实持仓退出提醒、计划外交易检查。所有违规实际成交仍应如实记录，不能以拒绝保存交易日志冒充拦单。尚未确定具体交易规则、阈值、手机通知渠道和券商能力；本轮未修改产品代码或启用通知。

下一步了解这次大亏交易当时的买入决策和后续处理过程，以确定最需要防范的行为。

---

## 2026-10-25 Mac 运行与看盘小组件可行性检查

### 用户

我这个软件现在在 Mac 上能正常运行起来吗，我没咋试过。然后我能不能在 mac 上加组件，方便我看盘。

附图为 macOS 通知中心，包含同花顺行情小组件。

### 助手检查与结论

- 本机 macOS 14.5、Apple Silicon arm64、Xcode 15.4。
- 现有 8080 服务由独立 cmd/server 进程提供；GET /api/health 返回 status=ok。浏览器实测交易雷达可加载登录页，未登录验证行情及账号内功能。
- build/bin/go-stock.app 原有 Contents/MacOS 为空，旧包无法直接运行。frontend/dist 也是历史构建产物，因此不代表当前源码的全部前端功能。
- 用 Wails 2.12 官方工具实测原生桌面后端编译成功：wails build -platform darwin/arm64 -s -skipbindings -m -nosyncgomod -nopackage -o go-stock-mac-check-20261025。此检查跳过前端重建及 bindings 生成。
- 用该二进制与旧包资源在 /tmp/go-stock-mac-check-20261025 中组成隔离测试包，在独立数据/缓存目录启动，实际看见原生窗口与底部股票自选、市场行情、K线分析、研究中心等导航。默认空数据下未验证实际行情加载。截图在 /tmp/go-stock-mac-check-20261025/native-window.png。临时测试进程已结束，原 8080 服务保留。
- 这是项目原有 Wails/Vue 界面，与目前交易雷达 Flutter Web 是两套前端；不能据此宣称当前所有交易雷达功能已打包进 Mac 原生应用。
- 独立 Mac 安装还需整理用户数据目录和缓存根目录；缓存解析当前依赖 GO_STOCK_CACHE_DIR 或查找项目 go.mod/backend，Mac 启动通知失败还会 Fatalf。
- 当前没有 macOS WidgetKit extension；Flutter StockWidgetManager 仅在 Android 生效。可新增 SwiftUI/WidgetKit 原生看盘组件，复用现有行情接口，但系统控制刷新，不保证秒级行情。
- 方便盘中持续看盘，优先建议可拖动、置顶的迷你行情窗；原生小组件适合行情概览。尚未开始开发小组件。
- Apple 官方依据：https://developer.apple.com/documentation/widgetkit/keeping-a-widget-up-to-date 与 https://support.apple.com/guide/mac-help/add-and-customize-widgets-mchl52be5da5/mac 。

---


## 2026-10-25 印象深刻交易笔记与盘中回顾提醒

### 用户

所以说的这些好像都没什么用但有一个点好像挺有用的就是痛苦痛苦教育。比如可以增加一个笔记模块入口。可以点击添加印象深刻的交易记录。然后每天盘中随机两分钟在屏幕中下方出现一个toast，说某年某月某日交易某股票名称赚或者亏了个点，赚或亏了多少钱你觉得咋样。

### 助手核查

现有交易日志有日期、股票、费用、理由和心态等字段，也有盈亏展示；未发现独立笔记模块。仅进行只读探索，未修改产品代码、启用定时器或发送通知。

### 讨论建议

建议做一个简单的“交易记忆”笔记入口，第一版允许手动记录日期、股票、已结束交易的扣费后实际盈亏、当时行为和一句教训，并选择是否参与盘中提醒。提醒在软件界面中下方短暂浮现、不抢焦点，点击可看完整笔记。盈亏作为固定历史结果保存，不随现价重算。把“是否遵守交易规则”与最终赚亏分别记录，避免把碰巧赚钱的违规交易当作正向示范。

提醒示意数据：某日某股票，计划外追高，亏损8.6%、4300元；当时因怕踏空、没有红策信号而买入；给自己的提醒是下单前先检查是否符合原定红策条件。

如果每两分钟一次，四小时盘中约120次，提醒可能逐渐被忽略；建议频率可调、轮换内容并设置全局暂停。提醒必须出现在用户实际持续盯盘的界面中。尚待明确“随机两分钟”指约每两分钟提醒一次，还是每天只随机提醒几次。

---


## 2026-10-25 防随手单 UI 交互设计讨论

### 用户

你帮我想想 通过软件 UI 交互 还有其他方式甚至更好的方式能够防止我开随手单吗 你是一个超强 UI交互设计师。

### 助手设计建议

把设计重心放在产生买入冲动和准备操作的时刻，随机个人交易记忆提醒保留为辅助。建议组合：盘前确定今日允许的交易计划并提供只看计划的专注视图；在软件内查询计划外股票、复制代码或选择跳转交易等明确动作时，呈现本人标记为同类行为的旧交易与教训；提供顺手的“先放观察，暂不买入”出口，继续偏离计划时写下本次新增的可核验证据，记录理由不等于检查通过；提供用户主动开启的暂停新开仓内容模式，保留持仓和卖出提醒。盘中放宽计划的生效规则需另外确认。

卖出不进入买入冷静流程。实际成交无论是否遵守计划都应如实记录；效果用实际成交核对计划外买入次数、主动放弃次数和绕过情况，不以通知量、短期收益或虚构的避免亏损金额证明有效。

用户主要手机下单，尚不清楚下单前是否会经过本软件；任何应用内检查都不能冒充已拦截外部手机券商操作，必须先验证实际使用路径。

### 交互示意

创建了独立的对话内买前对照示意，全部股票和盈亏均为虚构数据，不连接券商、不修改产品功能。包含“临时想买”和“盘前计划内”两种情形、旧单回忆、先观察、填写继续买入依据、核对预定条件等交互。使用 Node VM 验证了主要状态转换和选择器，无实际委托或通知。路径：/Users/Zhuanz/.codex/visualizations/2026/10/25/01a17072-2ff3-7603-9c02-99f9572b5f53/before-you-buy.html。

### 参考与限制

英国 FCA 交易应用实验发现特定促交易推送可增加交易频率；它没有测试用户提出的纪律回忆提醒，不能直接推断该功能有相同效果。来源：https://www.fca.org.uk/publications/fca-research/research-note-digital-engagement-practices-trading-apps-experiment 。设计效果需在实际使用中验证。

下一步确认：用户准备下单前，是否通常还会在本软件中查看该股票，以判断交互能否覆盖关键时刻。

---

## 2026-10-25 Flutter Mac 主应用内集成系统小组件的架构说明

### 用户

截图这种 Mac 小组件如果做的话，是打算新开一个项目吗，还是在我现在的 Flutter 项目里面加，把 Flutter 项目编译成 Mac 软件，里面包含 Mac 小部件？

### 助手答复依据

建议采用用户描述的后一种方案：在现有 trading_app/macOS Xcode 工程新增 SwiftUI + WidgetKit extension target，与 Flutter Mac 主应用同包安装，无需另建 Flutter 项目。Flutter/Dart 继续承担主界面，原生扩展承担桌面/通知中心组件；共享自选配置与行情快照，复用现有 Go 后端接口。Go 后端不会因 Flutter Mac 编译而自动被嵌入应用。

当前 trading_app/macos 已有 Runner 与 RunnerTests，没有 WidgetKit target。本轮只读核查，没有发现 Flutter Mac 已成功运行的 .app 产物；上一轮实际编译并打开的是 Wails/Vue 桌面版，不等于验证了 Flutter Mac。正式实施应先验证当前 Flutter Mac，补齐网络客户端权限及插件兼容，再添加小组件的数据共享、构建嵌入与签名。Debug/Profile 与 Release 均开启 App Sandbox，当前缺少 com.apple.security.network.client；现有 StockWidgetManager 只支持 Android。

官方依据：Apple Creating a widget extension；Flutter Building macOS apps with Flutter。本轮未修改产品代码，未开始实现小组件。

---


## 2026-10-25 防随手单设计转向低操作负担

### 用户

还有别的方案吗 你这个好像好废额外的时间和操作。

### 约束更新

用户不接受每笔交易前额外填写、确认、暂存或强制等待。后续方案应以一次性设置、盘中自动呈现、保留正常查看和退出操作为原则，不能继续扩展买前检查流程。

### 讨论方向

优先考虑常驻的个人交易教训条；在本软件正常搜索或打开计划外股票时自动显示轻量标签及相关旧单摘要；盘中专注视图默认呈现红策与持仓，把额外机会内容放到次级入口；考虑到手机下单习惯，也可一次性设置带本人教训的手机锁屏或桌面壁纸。原先提出的交易记忆随机 toast 可以保留为另一种可选呈现，而非增加每日必做操作。所有方案仍是行为提醒设计，尚未修改产品功能。

---


## 2026-10-25 主板策略列表升级为红策样式的评估

### 用户

我想把主板策略的列表UI样式也升级成 红测那样的UI 样式 改动大吗 你评估一下然后给我方案

### 代码核查与结论

按 Flutter 雷达页的“红策”理解用户所说的“红测”。主板与红策已经共用 radar_page.dart 中的 _buildStrategyTab 和 _buildStrategyCard，基础边框、圆角、间距一致；主要区别是红策专属形态标签、涨幅取值分支、自动换行分支及卡片下方日K与T+1分时图。主板保有自己的信号灯、形态统计、评级及开盘涨幅，应继续按主板数据展示。

仅统一排版为小改动，按当前代码粗估半天以内；若连日K、均线、长按查看与T+1分时切日联动一起接入，则为中等改动，粗估1至2个工作日含联调回归，不作为承诺。后端 t0_red_kline.go 与 t0_red_intraday.go 的模块校验目前只接受红策及共享GOB红策，主板不能只打开前端开关。RedKlineSettings 的现有持久化开关也只对应红策。

### 建议方案

推荐复用现有卡片和 RedStrategyCharts 图表组件，分步实现相同展示体验：先明确主板卡片信息层级，保留主板自身字段；再补齐主板图表接口模块支持与数据来源，接入日K及T+1分时；主板图表使用独立设置，不联动红策开关；图表按可见卡片加载并复用缓存，避免长列表一次性请求过多数据。上线前验证窄屏与宽屏、切日竞态、缺失行情、长列表滚动、复制代码和跳转，以及原有红策图表回归。

本轮未修改产品代码、未运行测试，只做源码评估。待用户明确是否连日K及T+1分时图一起迁移。

---

## 2026-10-25 实施 Flutter Mac 与空白系统小组件

### 用户

那就直接在现有 Flutter 项目里做，先把 Mac 版编译跑通，再加截图这种看盘小组件。小组件内容可以先放空的。我们要一步一步来实现。

### 实施与验证

- 补齐本地 CocoaPods 依赖与 Mac 网络客户端权限，现有 Flutter Mac Debug 编译并实际运行。通过开发者 VM 服务确认主界面及策略内容已渲染。
- 在现有 macOS Xcode 工程新增 SwiftUI/WidgetKit StockWidgetExtension，随主 App 打包。支持小、中、大尺寸，卡片内容留空，不接行情、不发通知。
- Debug 和最终 Release 均编译通过；最终 Release 54.7 MB，包含 Apple Silicon 与 Intel 架构。本机只验证 Apple Silicon 运行。
- 安装至 /Users/Zhuanz/Applications/交易雷达.app，主程序已启动，签名校验通过，PluginKit 已识别安装目录下唯一的交易雷达小组件。
- 自动原生窗口读取对应用和 Finder 均超时，系统组件库的实际添加尚待用户观察确认。已请求用户在桌面右键→编辑小组件中搜索“交易雷达”。未修改系统权限或服务。
- 修改前全量 Flutter 测试为 191 通过、9 项既有雷达相关失败，本轮无 Dart 业务改动；详细名称及构建证据见 docs/superpowers/reports/2026-10-25-flutter-macos-widget-verification.md。
- 新增可重复构建脚本 scripts/build-flutter-macos.sh 与 trading_app/macos/README.md。本机安装的构建连接已有 127.0.0.1:8080 Go 服务，不包含后端进程。

---

## 2026-10-25 用户确认 Mac 空白小组件添加成功

### 用户

针对“桌面右键→编辑小组件→搜索交易雷达，能否找到并添加空白小组件”，回答：能找到并添加。

### 助手

确认本阶段验收通过：现有 Flutter Mac 版已经跑通，空白 WidgetKit 小组件已随 App 打包安装，用户实际找到并添加成功。已更新实施计划和验收记录。本阶段不接行情数据，后续内容另行逐步确定。

---

## 2026-10-25 完整 App 在右侧交互的可行性讨论

### 用户

你把一整个 app 都可以做成小组件方式嵌入右侧吧，我可以组件里真实地操作。改动大吗，好做吗，有可行方案吗？

### 核查与方案

Apple WidgetKit 使用系统归档渲染的受限视图，交互主要通过 App Intent 按钮和开关，不能承载完整 Flutter 运行界面及任意输入、滚动和复杂页面。已有空白系统小组件已由用户验收，不能据此承诺可将整个 App 直接塞进系统通知中心。

推荐在现有 Flutter Mac 主应用增加右侧贴边面板模式，复用现有 FlutterViewController、页面、登录状态和 Go API；完整界面在 App 自己的窗口中接受点击、滚动、输入和导航。它位于屏幕右侧，但不是 macOS 系统通知中心内部的组件。

初版可先做固定右侧、可正常操作、能切回普通窗口。工作量小到中等，主要为 Mac 原生窗口控制和窄屏布局核验；自动滑出/隐藏、全局快捷键、多屏和全屏应用场景另行分步实现，工作量相应增加。需要确认用户是否接受 App 自己的右侧面板，还是必须在系统通知中心内部操作。本轮尚未修改产品代码。

官方依据：Apple Adding interactivity to widgets and Live Activities；Bring widgets to life (WWDC23)；NSPanel 文档。

---

## 2026-10-25 实施完整 Flutter 右侧面板

### 用户

可以，改然后运行起来看效果。

### 实施结果与当前阻塞

已新增完整 Flutter Mac 右侧面板模式及标题栏/菜单切换、宽度选择、普通窗口状态恢复和偏好记忆；不新建 Flutter 引擎。原生测试 5 项通过，Debug/Release 构建通过，新版已更新到 /Users/Zhuanz/Applications/交易雷达.app。原有 App 已备份；既有 WidgetKit 扩展保留。

实际启动被 macOS 应用数据访问确认拦住。进程与本用户 secinitd 采样直接显示 libsecinit_appsandbox 与 displaySharingConsentPrompt/CFUserNotificationReceiveResponse，尚未执行到 Flutter/面板代码。已请求用户查看并处理系统确认，未关闭沙盒或操作用户容器数据，也未重启系统服务。暂不宣称实际面板显示通过。

全量 Flutter 测试仍为 191 通过、9 项既有雷达相关失败，详细记录见 docs/superpowers/reports/2026-10-25-flutter-right-panel-verification.md。

---

## 2026-10-25 右侧面板实际运行验收补充

用户反馈没有看到应用数据权限提示，并已解锁 Mac。安装版后来成功进入 Flutter；系统日志发现启动回调调用了 FlutterAppDelegate 未实现的 super.applicationDidFinishLaunching，移除后重新构建 Release 并更新安装版。

已通过真实界面确认“交易雷达 · 右侧面板”显示完整 Flutter App，宽400点、可用高度1079点；标题栏按钮可切回普通窗口，Control+Command+P 可恢复面板。实际切换策略页、输入000001，返回平安银行搜索结果，未添加监控股票。用户开始自行操作窗口，保留其当前选择。宽度菜单入口已确认，菜单选宽/拖动宽度未完成独立UI验收；历史红策接口曾返回503，未改动该后端逻辑。原生5项测试通过；Flutter既有9项失败详见验收报告。

## 2026-10-25 更正为顶部菜单栏快捷入口

用户：“好像不对，我还是要做顶部有个快捷按钮，点一下就出来了”，并附顶部菜单栏截图。

在既有 Flutter Mac 工程新增原生走势图菜单栏按钮，左键展开/收起完整右侧App，启动先隐藏，关闭窗口仅隐藏，右键可显示/退出，Dock可以重开。复用窗口、引擎与页面状态，不更改WidgetKit或用户数据。已通过7项原生测试、Release编译和签名校验，更新并启动安装版。运行日志已确认启动隐藏及重开显示；正等待用户确认顶部图标点击效果。

用户反馈没有找到顶部图标。已将走势图改为明确的“盘”字菜单栏按钮，重新编译、更新并启动；运行日志确认菜单栏installed visible=1。已再次请求用户验证“盘”字按钮显示和点击效果，尚未据此宣称人工验收完成。

用户再次反馈“仍然看不到‘盘’”。菜单栏人工验收未通过，未宣称完成。增加本App菜单栏布局日志后重新构建、安装，最新重启在macOS libsecinit_appsandbox阶段等待，尚无布局输出；系统菜单栏及Finder界面读取超时。没有关闭沙盒、重启系统服务或更改其他应用图标。需要当前完整顶部菜单栏截图进一步定位，暂停无证据的反复重编译。


## 2026-10-25 主板列表完整升级与可组合组件实现

### 用户

第二种, 最好把UI层抽取一下 , 可以自由组合的组件一样.

### 完成内容

按第二种实现主板日K、均线、长按查看和T+1分时切日联动；主板图表按卡片可见后加载，设置中有独立的主板K线、主板T+1分时开关，默认开启。保留主板自身的信号、评级、标签、选股排序和统计口径。

抽取StrategyCard/StrategyCardHeader、StrategyStockCard、各类Strategy标签、StrategyCharts、StrategyChartExposure<T>、StrategyStockCharts及StrategyChartSettings。支持卡片信息/操作/内容插槽组合，兼容原RedStrategyCharts与RedKlineSettings入口。后端两个现有图表接口接受radar.main_strategy。

独立只读审查发现分时刷新旧请求重新写缓存的竞态，已先用测试复现并修复；按模块清缓存避免主板刷新影响红策缓存。实现和审查没有使用gpt-6-astra。

### 验证

38项相关Flutter测试、11个顶层后端图表测试通过；19个新增/相关文件定向静态检查通过；实际Flutter组件在390×844与1100×860渲染并检查无溢出；Web独立构建成功。全量Flutter为205通过、9失败（6项修改前已有基线失败、另3项源文件与测试未改动）；后端整包测试有模块目录/revision预期失败并在原有行情回退路径因nil Gorm DB崩溃，不宣称全量通过。

验证记录：docs/superpowers/validation/2026-10-25-composable-strategy-ui.md。设计及组合示例：docs/superpowers/specs/2026-10-25-composable-strategy-ui-design.md。实现计划：docs/superpowers/plans/2026-10-25-composable-strategy-ui.md。

当前工作区其他任务的修改均保留。没有提交、发布或重启运行中的服务；Web构建独立输出到/tmp/go-stock-mainboard-ui-web。

---

## 2026-10-25 用户再次澄清菜单栏点击入口

用户要求自己完成并测试，强调“添加到顶部菜单栏，点一下直接出来软件，代码量应很少”。已收敛为现有NSStatusItem入口，采用应用自身首次位置偏好0的最小修正；原生对照实验x447→x1440，未覆盖已有位置或其他应用设置。独立签名隔离预览副本正常运行，菜单栏坐标x1439，原生实际按钮target/action集成测试通过，总计8项原生测试通过。原安装版仍受macOS应用数据访问确认阻塞，CUA明确拒绝访问UserNotificationCenter，未绕过。CUA桌面菜单栏坐标点击因windowNotFoundAtPosition未完成，未伪称鼠标端到端验收。保留预览副本供本机运行，旧容器数据保留。

## 2026-10-25 菜单栏入口状态追问

用户：“启动了吗，还是没看到顶部菜单有啥图标”。本轮核对：只有隔离预览副本进程70968仍在运行，原安装版进程未运行；菜单栏桌面可见性及鼠标点击仍未验收。没有把进程存在或布局坐标当作用户可见性证据。读取ControlCenter再次超时；未绕过界面工具限制。当前不能报告顶部快捷入口已可用。

## 2026-10-25 菜单栏入口后续处理

用户问“那咋办”。本轮采样SystemUIServer与ControlCenter，主线程均在正常事件循环，未据此宣称系统菜单栏卡死，未重启系统服务。为自行完成真实界面验证，按cua_repl工具对其他UI技术的限制，明确请求用户允许只使用macOS系统截图与AppleScript查看顶部菜单栏、点击交易雷达按钮，不操作系统权限提示。等待授权后再执行该替代验证方式。

## 2026-10-25 菜单栏最终点击验收通过

用户明确允许使用macOS系统截图与AppleScript，只检查顶部菜单栏和点击交易雷达按钮。直接截图确认“盘”位于Wi-Fi和放大镜之间。主安装版 /Users/Zhuanz/Applications/交易雷达.app 已启动，PID82593；预览副本已退出。读取该进程menu bar2包含“盘”，位置1439,7。实际点击：窗口1→0收起，再点击0→1恢复，完整Flutter红策页面及数据保留。安装版签名再次通过。保持主App运行并展开。证据在.codex_work/menu-bar-verification/，验收报告已更新为通过。


## 2026-10-25 汇总提交与阿里云部署

用户：没提交的都提交一下，推送远端主分支，并部署阿里云；补充要求远端 master 也提交。

操作范围：汇总代码、文档、研究和历史补采数据到 dev，并同步创建同版本 master。排除本机运行数据库；保留服务器账户和设置。重新执行相关测试、Linux 后端、Flutter Web 和管理后台构建，记录全量测试原有失败。阿里云公网健康检查通过；SSH 认证失败，已请求可用连接方式，继续完成推送与发布包。

验证记录：docs/superpowers/validation/2026-10-25-release-preflight.md。

推送记录：共享日线 LFS 对象已上传；普通 Git HTTP 408，SSH 为无写权限的另一账号。将压缩研究数据转 Git LFS，保持全部内容不变后重新推送 dev/master。
