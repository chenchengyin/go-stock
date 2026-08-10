import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../../core/theme/app_colors.dart';
import '../../../shared/view_state.dart';
import '../domain/short_term_emotion_explain_content.dart';
import '../domain/short_term_emotion_explain_models.dart';
import '../domain/short_term_emotion_models.dart';
import 'short_term_emotion_explain_page.dart';
import 'short_term_emotion_trend_chart.dart';
import 'short_term_emotion_view_model.dart';

class ShortTermEmotionPage extends StatefulWidget {
  const ShortTermEmotionPage({super.key});

  @override
  State<ShortTermEmotionPage> createState() => _ShortTermEmotionPageState();
}

class _ShortTermEmotionPageState extends State<ShortTermEmotionPage>
    with SingleTickerProviderStateMixin {
  late final AnimationController _refreshController;
  Timer? _autoRefreshTimer;

  @override
  void initState() {
    super.initState();
    _refreshController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    );
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<ShortTermEmotionViewModel>().load();
    });
    _autoRefreshTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      if (!mounted) return;
      context.read<ShortTermEmotionViewModel>().refresh();
    });
  }

  @override
  void dispose() {
    _autoRefreshTimer?.cancel();
    _refreshController.dispose();
    super.dispose();
  }

  Future<void> _onRefresh() async {
    _refreshController.repeat();
    await context.read<ShortTermEmotionViewModel>().refresh();
    if (!mounted) return;
    _refreshController.stop();
    _refreshController.reset();
  }

  void _openExplain(ShortTermEmotionExplainPageData data) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => ShortTermEmotionExplainPage(data: data),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<ShortTermEmotionViewModel>();

    return Container(
      color: AppColors.scaffoldBg,
      child: SafeArea(
        bottom: false,
        child: Column(
          children: [
            _Header(
              onRefresh: _onRefresh,
              refreshController: _refreshController,
            ),
            Expanded(child: _buildBody(vm)),
          ],
        ),
      ),
    );
  }

  Widget _buildBody(ShortTermEmotionViewModel vm) {
    if (vm.state.isLoading && vm.emotion == null) {
      return const Center(child: CupertinoActivityIndicator());
    }

    if (vm.state.status == ViewStatus.error && vm.emotion == null) {
      return _ErrorState(message: vm.state.message, onRetry: _onRefresh);
    }

    final emotion = vm.emotion;
    if (emotion == null) {
      return _EmptyState(onRetry: _onRefresh);
    }

    return RefreshIndicator(
      onRefresh: _onRefresh,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 20),
        children: [
          _HeroCard(
            emotion: emotion,
            onExplain: () => _openExplain(ShortTermEmotionExplainContent.score),
          ),
          const SizedBox(height: 10),
          ShortTermEmotionTrendChart(items: emotion.intradayTrend),
          const SizedBox(height: 10),
          _Section(
            title: '盯盘仪表盘',
            subtitle: '先看能不能做',
            onExplain: () =>
                _openExplain(ShortTermEmotionExplainContent.dashboard),
            child: _DashboardGrid(items: emotion.dashboard),
          ),
          const SizedBox(height: 10),
          _Section(
            title: '评分拆解',
            subtitle: '权重固定，后续可校准',
            onExplain: () =>
                _openExplain(ShortTermEmotionExplainContent.components),
            child: _ComponentList(items: emotion.components),
          ),
          const SizedBox(height: 10),
          _Section(
            title: '风险信号',
            subtitle: '触发才显示重点',
            child: _RiskSignalList(items: emotion.riskSignals),
          ),
        ],
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.onRefresh, required this.refreshController});

  final VoidCallback onRefresh;
  final AnimationController refreshController;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 48,
      color: AppColors.cardBg,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Stack(
        alignment: Alignment.center,
        children: [
          Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(
                '超短情绪',
                style: TextStyle(
                  fontSize: 17,
                  fontWeight: FontWeight.w600,
                  color: AppColors.textPrimary,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                '短线避坑',
                style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
              ),
            ],
          ),
          Align(
            alignment: Alignment.centerRight,
            child: GestureDetector(
              onTap: onRefresh,
              child: Padding(
                padding: const EdgeInsets.all(8),
                child: RotationTransition(
                  turns: refreshController,
                  child: Icon(
                    CupertinoIcons.refresh,
                    size: 22,
                    color: AppColors.link,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _HeroCard extends StatelessWidget {
  const _HeroCard({required this.emotion, required this.onExplain});

  final ShortTermEmotion emotion;
  final VoidCallback onExplain;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    GestureDetector(
                      behavior: HitTestBehavior.opaque,
                      onTap: onExplain,
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            '市场情绪分',
                            style: TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textPrimary,
                          ),
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
                    const SizedBox(height: 4),
                    RichText(
                      text: TextSpan(
                        children: [
                          TextSpan(
                            text: '${emotion.score}',
                            style: TextStyle(
                              fontSize: 46,
                              height: 1,
                              fontWeight: FontWeight.w800,
                              color: _scoreColor(emotion.score),
                            ),
                          ),
                          TextSpan(
                            text: '/100',
                            style: TextStyle(
                              fontSize: 15,
                              color: AppColors.textTertiary,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(
                    emotion.phase,
                    style: TextStyle(
                      fontSize: 17,
                      fontWeight: FontWeight.w700,
                      color: AppColors.tagOrange,
                    ),
                  ),
                  const SizedBox(height: 8),
                  _TonePill(text: emotion.action, tone: _actionTone(emotion)),
                ],
              ),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: _SummaryBox(label: '风险', value: emotion.riskLevel),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _SummaryBox(label: '仓位', value: emotion.suggestedWeight),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _SummaryBox(label: '主线', value: emotion.mainTheme),
              ),
            ],
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              _StatusDot(active: emotion.isTrading),
              const SizedBox(width: 6),
              Text(
                emotion.isTrading ? '交易中' : '非交易时段',
                style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
              ),
              const Spacer(),
              Text(
                emotion.updateTime.isEmpty ? '未更新' : '${emotion.updateTime} 更新',
                style: TextStyle(fontSize: 12, color: AppColors.textTertiary),
              ),
            ],
          ),
          const SizedBox(height: 12),
          // Divider(height: 1, color: AppColors.divider),
          // const SizedBox(height: 10),
          Text(
            '短线避坑结论',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
            ),
          ),
          const SizedBox(height: 5),
          Text(
            emotion.explanation.isEmpty ? '暂无结论' : emotion.explanation,
            style: TextStyle(
              fontSize: 13,
              height: 1.5,
              color: AppColors.tagOrange,
            ),
          ),
        ],
      ),
    );
  }
}

class _SummaryBox extends StatelessWidget {
  const _SummaryBox({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minHeight: 58),
      padding: const EdgeInsets.all(9),
      decoration: BoxDecoration(
        color: AppColors.surfaceBg,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
          ),
          const SizedBox(height: 6),
          Text(
            value,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
            ),
          ),
        ],
      ),
    );
  }
}

class _Section extends StatelessWidget {
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

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
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
              if (subtitle != null) ...[
                const Spacer(),
                Text(
                  subtitle!,
                  style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
                ),
              ],
            ],
          ),
          const SizedBox(height: 10),
          child,
        ],
      ),
    );
  }
}

class _DashboardGrid extends StatelessWidget {
  const _DashboardGrid({required this.items});

  final List<ShortTermEmotionMetric> items;

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) return const _NoDataText(text: '暂无仪表盘数据');
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: items.length,
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 2,
        crossAxisSpacing: 4,
        mainAxisSpacing:4,
        childAspectRatio: 2.5,
      ),
      itemBuilder: (_, index) => _MetricTile(item: items[index]),
    );
  }
}

class _MetricTile extends StatelessWidget {
  const _MetricTile({required this.item});

  final ShortTermEmotionMetric item;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.only(top:0, bottom: 0, left: 12, right: 12),
      decoration: BoxDecoration(
        color: AppColors.surfaceBg,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(
            item.name,
            style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
          ),
          const SizedBox(height: 5),
          Text(
            item.value,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w800,
              color: _toneColor(item.tone),
              
            ),
          ),
          const SizedBox(height: 4),
          Text(
            item.note,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
          ),
        ],
      ),
    );
  }
}

class _ComponentList extends StatelessWidget {
  const _ComponentList({required this.items});

  final List<ShortTermEmotionComponent> items;

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) return const _NoDataText(text: '暂无评分拆解');
    return Column(
      children: List.generate(items.length, (index) {
        final item = items[index];
        return Padding(
          padding: EdgeInsets.only(bottom: index == items.length - 1 ? 0 : 10),
          child: _ComponentRow(item: item),
        );
      }),
    );
  }
}

class _ComponentRow extends StatelessWidget {
  const _ComponentRow({required this.item});

  final ShortTermEmotionComponent item;

  @override
  Widget build(BuildContext context) {
    final score = item.score.clamp(0, 100);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              item.name,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: AppColors.textPrimary,
              ),
            ),
            const Spacer(),
            Text(
              '${item.weight}% · $score',
              style: TextStyle(fontSize: 12, color: AppColors.textTertiary),
            ),
          ],
        ),
        const SizedBox(height: 6),
        ClipRRect(
          borderRadius: BorderRadius.circular(99),
          child: LinearProgressIndicator(
            value: score / 100,
            minHeight: 7,
            color: _scoreColor(score),
            backgroundColor: AppColors.border,
          ),
        ),
        if (item.note.isNotEmpty) ...[
          const SizedBox(height: 4),
          Text(
            item.note,
            style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
          ),
        ],
      ],
    );
  }
}

class _RiskSignalList extends StatelessWidget {
  const _RiskSignalList({required this.items});

  final List<ShortTermEmotionSignal> items;

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) return const _NoDataText(text: '暂无风险信号');
    return Column(
      children: List.generate(items.length, (index) {
        final item = items[index];
        return Container(
          margin: EdgeInsets.only(bottom: index == items.length - 1 ? 0 : 8),
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: AppColors.surfaceBg,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: AppColors.border),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 8,
                height: 8,
                margin: const EdgeInsets.only(top: 5),
                decoration: BoxDecoration(
                  color: _toneColor(item.tone),
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      item.name,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textPrimary,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      item.note,
                      style: TextStyle(
                        fontSize: 12,
                        height: 1.35,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              _TonePill(text: item.level, tone: item.tone),
            ],
          ),
        );
      }),
    );
  }
}

class _TonePill extends StatelessWidget {
  const _TonePill({required this.text, required this.tone});

  final String text;
  final String tone;

  @override
  Widget build(BuildContext context) {
    final color = _toneColor(tone);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(99),
        border: Border.all(color: color.withValues(alpha: 0.35)),
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }
}

class _StatusDot extends StatelessWidget {
  const _StatusDot({required this.active});

  final bool active;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 8,
      height: 8,
      decoration: BoxDecoration(
        color: active ? AppColors.success : AppColors.textTertiary,
        shape: BoxShape.circle,
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              CupertinoIcons.exclamationmark_triangle,
              size: 42,
              color: AppColors.textTertiary,
            ),
            const SizedBox(height: 12),
            Text(
              message.isEmpty ? '获取超短情绪失败' : message,
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 13, color: AppColors.textSecondary),
            ),
            const SizedBox(height: 12),
            CupertinoButton.filled(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: CupertinoButton(
        onPressed: onRetry,
        child: const Text('暂无市场统计数据，点击重试'),
      ),
    );
  }
}

class _NoDataText extends StatelessWidget {
  const _NoDataText({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: TextStyle(fontSize: 12, color: AppColors.textTertiary),
    );
  }
}

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

Color _scoreColor(int score) {
  if (score >= 70) return AppColors.textPriceUp;
  if (score >= 45) return AppColors.warning;
  return AppColors.textPriceDown;
}

Color _toneColor(String tone) {
  switch (tone) {
    case 'positive':
    case 'success':
      return AppColors.textPriceUp;
    case 'negative':
    case 'danger':
      return AppColors.error;
    case 'warning':
      return AppColors.warning;
    default:
      return AppColors.info;
  }
}

String _actionTone(ShortTermEmotion emotion) {
  if (emotion.riskLevel == '高' || emotion.action.contains('观察')) {
    return 'danger';
  }
  if (emotion.riskLevel.contains('高') || emotion.action.contains('谨慎')) {
    return 'warning';
  }
  return 'positive';
}
