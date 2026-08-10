# Short Term Emotion Explain Pages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 Flutter App 的 `超短情绪` 页面增加可点击说明详情页，让用户点 `市场情绪分`、`盯盘仪表盘`、`评分拆解` 时能看到完整解释、分数含义、阶段说明和指标口径。

**Architecture:** Flutter 端新增一个纯静态说明内容模型和一个统一说明详情页。`ShortTermEmotionPage` 只负责从三个入口跳转到详情页，不新增后端接口，不改变现有评分 JSON。说明内容先内置在 App 端，后续如果要远程配置再迁移到接口。

**Tech Stack:** Flutter、Provider 现有页面结构、`Navigator.of(context).push`、现有 `AppColors` 主题系统、Dart 静态常量内容。

## Global Constraints

- 只改 Flutter App 端，不改 Wails/Vue 前端。
- 不改后端评分算法和接口字段。
- 新页面风格要融入当前 App：白底、轻卡片、蓝色品牌色、紧凑信息密度。
- 详情页必须可上下滑动，避免长说明在小屏手机上显示不全。
- 点击入口要克制，不把当前盯盘页做得像教程页；用小图标或整块轻提示表达“可查看说明”。
- 文案必须中文，面向超短交易者，重点讲清楚“怎么理解”和“怎么避坑”。

---

### Task 1: 说明内容模型和静态文案

**Files:**
- Create: `trading_app/lib/features/short_term_emotion/domain/short_term_emotion_explain_models.dart`
- Create: `trading_app/lib/features/short_term_emotion/domain/short_term_emotion_explain_content.dart`

**Interfaces:**
- Produces: `ShortTermEmotionExplainPageData`
- Produces: `ShortTermEmotionExplainSection`
- Produces: `ShortTermEmotionExplainRow`
- Produces: `ShortTermEmotionExplainContent.score`
- Produces: `ShortTermEmotionExplainContent.dashboard`
- Produces: `ShortTermEmotionExplainContent.components`

- [x] **Step 1: 创建说明内容模型**

Create `trading_app/lib/features/short_term_emotion/domain/short_term_emotion_explain_models.dart`:

```dart
class ShortTermEmotionExplainPageData {
  const ShortTermEmotionExplainPageData({
    required this.title,
    required this.subtitle,
    required this.summary,
    required this.sections,
  });

  final String title;
  final String subtitle;
  final String summary;
  final List<ShortTermEmotionExplainSection> sections;
}

class ShortTermEmotionExplainSection {
  const ShortTermEmotionExplainSection({
    required this.title,
    this.body,
    this.rows = const [],
  });

  final String title;
  final String? body;
  final List<ShortTermEmotionExplainRow> rows;
}

class ShortTermEmotionExplainRow {
  const ShortTermEmotionExplainRow({
    required this.label,
    required this.value,
    this.note,
  });

  final String label;
  final String value;
  final String? note;
}
```

- [x] **Step 2: 创建市场情绪分说明内容**

Create `trading_app/lib/features/short_term_emotion/domain/short_term_emotion_explain_content.dart` and import the model file.

Add `ShortTermEmotionExplainContent.score` with:

```dart
static const score = ShortTermEmotionExplainPageData(
  title: '市场情绪分说明',
  subtitle: '综合分不是买卖信号，是短线环境温度计',
  summary: '市场情绪分把宽度、涨跌停质量、连板生态、异动强弱、板块主线和指数量能合成 0-100 分。它主要用于判断今天适不适合出手，以及应该主动还是防守。',
  sections: [
    ShortTermEmotionExplainSection(
      title: '分数怎么看',
      rows: [
        ShortTermEmotionExplainRow(label: '0-34', value: '冰点/退潮', note: '少做或不做，优先处理持仓风险。'),
        ShortTermEmotionExplainRow(label: '35-44', value: '弱修复', note: '只看最强核心，普通机会容易冲高回落。'),
        ShortTermEmotionExplainRow(label: '45-59', value: '分歧震荡', note: '可以观察，仓位要轻，避免追后排。'),
        ShortTermEmotionExplainRow(label: '60-74', value: '活跃偏强', note: '短线可参与，但仍要看炸板和跌停风险。'),
        ShortTermEmotionExplainRow(label: '75-100', value: '高潮/强一致', note: '赚钱效应强，但次日分歧风险也会上升。'),
      ],
    ),
    ShortTermEmotionExplainSection(
      title: '情绪阶段',
      rows: [
        ShortTermEmotionExplainRow(label: '数据不足', value: '接口没有足够数据', note: '不输出交易倾向。'),
        ShortTermEmotionExplainRow(label: '退潮防守', value: '跌停、空头异动或宽度明显恶化', note: '超短优先避坑。'),
        ShortTermEmotionExplainRow(label: '活跃偏分歧', value: '有赚钱效应但风险信号没有消失', note: '适合轻仓试错。'),
        ShortTermEmotionExplainRow(label: '强势活跃', value: '宽度、涨停、主线和量能共同支持', note: '可提高关注度，但仍要防一致后分歧。'),
      ],
    ),
    ShortTermEmotionExplainSection(
      title: '仓位建议含义',
      body: '仓位只是风控参考，不代表必须买入。对超短来说，低分环境最重要的是少亏，高分环境也要避免情绪高潮后的后排接力。',
    ),
  ],
);
```

- [x] **Step 3: 创建盯盘仪表盘说明内容**

In `ShortTermEmotionExplainContent`, add `dashboard` with:

```dart
static const dashboard = ShortTermEmotionExplainPageData(
  title: '盯盘仪表盘说明',
  subtitle: '先看市场能不能做，再看个股值不值得做',
  summary: '盯盘仪表盘展示的是盘中最需要快速扫一眼的风险指标。它不追求解释所有行情，只帮你在下单前确认今天有没有明显短线陷阱。',
  sections: [
    ShortTermEmotionExplainSection(
      title: '核心指标',
      rows: [
        ShortTermEmotionExplainRow(label: '红盘率', value: '上涨家数 / 上涨下跌总数', note: '低于 40% 通常说明赚钱效应偏弱。'),
        ShortTermEmotionExplainRow(label: '涨跌停', value: '涨停数 / 跌停数', note: '跌停明显增多时，超短接力风险会快速上升。'),
        ShortTermEmotionExplainRow(label: '炸板率', value: '打开涨停板 / 封板数量', note: '炸板率高说明封板质量差，追板更容易吃面。'),
        ShortTermEmotionExplainRow(label: '最高连板', value: '市场最高空间板', note: '空间越高，接力生态越活跃，但高位一致也可能次日分歧。'),
        ShortTermEmotionExplainRow(label: '异动强弱', value: '多头异动 / 空头异动', note: '空头异动超过多头时，要降低进攻欲望。'),
        ShortTermEmotionExplainRow(label: '指数量能', value: '两市成交额和量能分', note: '缩量上涨容易虚强，放量杀跌也要防风险释放。'),
      ],
    ),
    ShortTermEmotionExplainSection(
      title: '超短使用方式',
      body: '开盘后先看红盘率和跌停，确认环境是否极端；再看炸板率和异动强弱，判断追高是否危险；最后看主线和量能，判断机会是否集中。',
    ),
  ],
);
```

- [x] **Step 4: 创建评分拆解说明内容**

In `ShortTermEmotionExplainContent`, add `components` with:

```dart
static const components = ShortTermEmotionExplainPageData(
  title: '评分拆解说明',
  subtitle: '看清楚分数从哪里来，也看清楚哪里在拖后腿',
  summary: '评分拆解把总分拆成六个固定权重维度。它的价值不是追求绝对准确，而是让你知道当前市场到底是宽度好、涨停强、主线强，还是只是某一个指标短暂好看。',
  sections: [
    ShortTermEmotionExplainSection(
      title: '默认权重',
      rows: [
        ShortTermEmotionExplainRow(label: '宽度情绪', value: '25%', note: '看红盘率，判断赚钱效应是否扩散。'),
        ShortTermEmotionExplainRow(label: '涨跌停质量', value: '25%', note: '看涨停、跌停、炸板率，是超短避坑的核心。'),
        ShortTermEmotionExplainRow(label: '连板生态', value: '20%', note: '看最高连板和 3 板以上数量。'),
        ShortTermEmotionExplainRow(label: '异动强弱', value: '15%', note: '看盘中多空异动谁更占优。'),
        ShortTermEmotionExplainRow(label: '板块主线', value: '10%', note: '看有没有清晰主线承接资金。'),
        ShortTermEmotionExplainRow(label: '指数量能', value: '5%', note: '看两市成交额是否支持行情延续。'),
      ],
    ),
    ShortTermEmotionExplainSection(
      title: '为什么偏避坑',
      body: '模型会额外惩罚炸板率、跌停数量和空头异动。也就是说，就算某些指标好看，只要风险信号变重，总分和动作建议也会被压下来。',
    ),
  ],
);
```

- [x] **Step 5: 验证 Task 1**

Run:

```bash
cd trading_app && dart analyze lib/features/short_term_emotion/domain
```

Expected: no new analyzer errors.

---

### Task 2: 统一说明详情页

**Files:**
- Create: `trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_explain_page.dart`

**Interfaces:**
- Consumes: `ShortTermEmotionExplainPageData`
- Produces: `ShortTermEmotionExplainPage(data: ShortTermEmotionExplainPageData)`

- [x] **Step 1: 创建详情页骨架**

Create `short_term_emotion_explain_page.dart`:

```dart
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../domain/short_term_emotion_explain_models.dart';

class ShortTermEmotionExplainPage extends StatelessWidget {
  const ShortTermEmotionExplainPage({super.key, required this.data});

  final ShortTermEmotionExplainPageData data;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.scaffoldBg,
      appBar: AppBar(
        backgroundColor: AppColors.cardBg,
        elevation: 0,
        centerTitle: true,
        leading: IconButton(
          icon: Icon(CupertinoIcons.chevron_left, color: AppColors.textPrimary),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: Text(
          data.title,
          style: TextStyle(
            fontSize: 17,
            fontWeight: FontWeight.w700,
            color: AppColors.textPrimary,
          ),
        ),
      ),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(12, 12, 12, 24),
        children: [
          _SummaryCard(data: data),
          const SizedBox(height: 10),
          ...data.sections.map((section) => Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: _ExplainSection(section: section),
              )),
        ],
      ),
    );
  }
}
```

- [x] **Step 2: 实现摘要卡片**

Add in same file:

```dart
class _SummaryCard extends StatelessWidget {
  const _SummaryCard({required this.data});

  final ShortTermEmotionExplainPageData data;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            data.subtitle,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            data.summary,
            style: TextStyle(
              fontSize: 13,
              height: 1.55,
              color: AppColors.textSecondary,
            ),
          ),
        ],
      ),
    );
  }
}
```

- [x] **Step 3: 实现说明 Section 和 Row**

Add in same file:

```dart
class _ExplainSection extends StatelessWidget {
  const _ExplainSection({required this.section});

  final ShortTermEmotionExplainSection section;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            section.title,
            style: TextStyle(
              fontSize: 15,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
            ),
          ),
          if (section.body != null) ...[
            const SizedBox(height: 8),
            Text(
              section.body!,
              style: TextStyle(
                fontSize: 13,
                height: 1.5,
                color: AppColors.textSecondary,
              ),
            ),
          ],
          if (section.rows.isNotEmpty) ...[
            const SizedBox(height: 10),
            ...section.rows.map((row) => _ExplainRow(row: row)),
          ],
        ],
      ),
    );
  }
}

class _ExplainRow extends StatelessWidget {
  const _ExplainRow({required this.row});

  final ShortTermEmotionExplainRow row;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: AppColors.surfaceBg,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.border),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 72,
            child: Text(
              row.label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w700,
                color: AppColors.link,
              ),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  row.value,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                  ),
                ),
                if (row.note != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    row.note!,
                    style: TextStyle(
                      fontSize: 12,
                      height: 1.35,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}
```

- [x] **Step 4: 实现卡片装饰函数**

Add in same file:

```dart
BoxDecoration _cardDecoration() {
  return BoxDecoration(
    color: AppColors.cardBg,
    borderRadius: BorderRadius.circular(12),
    border: Border.all(color: AppColors.border),
    boxShadow: [
      BoxShadow(
        color: Colors.black.withValues(alpha: 0.03),
        blurRadius: 8,
        offset: const Offset(0, 2),
      ),
    ],
  );
}
```

- [x] **Step 5: 验证 Task 2**

Run:

```bash
cd trading_app && dart analyze lib/features/short_term_emotion/presentation/short_term_emotion_explain_page.dart
```

Expected: no analyzer errors.

---

### Task 3: 接入三个点击入口

**Files:**
- Modify: `trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_page.dart`

**Interfaces:**
- Consumes: `ShortTermEmotionExplainContent.score`
- Consumes: `ShortTermEmotionExplainContent.dashboard`
- Consumes: `ShortTermEmotionExplainContent.components`
- Consumes: `ShortTermEmotionExplainPage`

- [x] **Step 1: 添加 import**

At top of `short_term_emotion_page.dart`, add:

```dart
import '../domain/short_term_emotion_explain_content.dart';
import 'short_term_emotion_explain_page.dart';
```

- [x] **Step 2: 添加跳转方法**

Inside `_ShortTermEmotionPageState`, add:

```dart
void _openExplain(ShortTermEmotionExplainPageData data) {
  Navigator.of(context).push(
    MaterialPageRoute(
      builder: (_) => ShortTermEmotionExplainPage(data: data),
    ),
  );
}
```

Also import the model if the type is required:

```dart
import '../domain/short_term_emotion_explain_models.dart';
```

- [x] **Step 3: 市场情绪分入口接入**

Change:

```dart
_HeroCard(emotion: emotion),
```

to:

```dart
_HeroCard(
  emotion: emotion,
  onExplain: () => _openExplain(ShortTermEmotionExplainContent.score),
),
```

Change `_HeroCard` constructor:

```dart
const _HeroCard({required this.emotion, required this.onExplain});

final ShortTermEmotion emotion;
final VoidCallback onExplain;
```

Wrap the `市场情绪分` label row with a compact tap target:

```dart
GestureDetector(
  behavior: HitTestBehavior.opaque,
  onTap: onExplain,
  child: Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(
        '市场情绪分',
        style: TextStyle(fontSize: 12, color: AppColors.textTertiary),
      ),
      const SizedBox(width: 4),
      Icon(
        CupertinoIcons.info_circle,
        size: 14,
        color: AppColors.link,
      ),
    ],
  ),
),
```

- [x] **Step 4: Section 支持说明入口**

Change `_Section` constructor:

```dart
const _Section({
  required this.title,
  required this.child,
  this.subtitle,
  this.onExplain,
});

final String title;
final String? subtitle;
final Widget child;
final VoidCallback? onExplain;
```

In the title row, replace the title `Text` with:

```dart
GestureDetector(
  behavior: HitTestBehavior.opaque,
  onTap: onExplain,
  child: Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(
        title,
        style: TextStyle(
          fontSize: 15,
          fontWeight: FontWeight.w700,
          color: AppColors.textPrimary,
        ),
      ),
      if (onExplain != null) ...[
        const SizedBox(width: 5),
        Icon(
          CupertinoIcons.info_circle,
          size: 15,
          color: AppColors.link,
        ),
      ],
    ],
  ),
),
```

- [x] **Step 5: 盯盘仪表盘和评分拆解入口接入**

Change dashboard section:

```dart
_Section(
  title: '盯盘仪表盘',
  subtitle: '先看能不能做',
  onExplain: () => _openExplain(ShortTermEmotionExplainContent.dashboard),
  child: _DashboardGrid(items: emotion.dashboard),
),
```

Change components section:

```dart
_Section(
  title: '评分拆解',
  subtitle: '权重固定，后续可校准',
  onExplain: () => _openExplain(ShortTermEmotionExplainContent.components),
  child: _ComponentList(items: emotion.components),
),
```

Leave `短线避坑结论` and `风险信号` unchanged for this task.

- [x] **Step 6: 验证 Task 3**

Run:

```bash
cd trading_app && dart analyze lib/features/short_term_emotion
```

Expected: no new analyzer errors.

Manual checks:

- Tap `市场情绪分` info icon opens `市场情绪分说明`.
- Tap `盯盘仪表盘` info icon opens `盯盘仪表盘说明`.
- Tap `评分拆解` info icon opens `评分拆解说明`.
- Detail page can scroll on small screen.
- Back button returns to `超短情绪` page without losing bottom Tab state.

---

## Implementation Notes

- 这次只做静态说明页，不新增接口；原因是说明内容变化不频繁，放 App 端更快、更稳定。
- 后续如果你想在后台动态调整说明文案，可以把 `ShortTermEmotionExplainContent` 的内容迁移成 `/api/short-term-emotion/explain`。
- 当前三个入口只加 `info_circle`，不在页面里增加大段说明，避免盯盘页变啰嗦。

## Self Review

- Spec coverage: 覆盖了市场情绪分、盯盘仪表盘、评分拆解三个入口和详情页。
- Placeholder scan: 没有 `TBD`、`TODO` 或未定义步骤。
- Type consistency: `ShortTermEmotionExplainPageData`、`ShortTermEmotionExplainContent`、`ShortTermEmotionExplainPage` 命名一致。
