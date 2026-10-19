import 'package:flutter/material.dart';
import 't0_strategy_view_model.dart';

String redRuleNumbers(T0RedRuleStats? stats, {bool verified = true}) =>
    verified && stats != null && stats.sampleCount > 0
    ? '(${stats.profitWinRate.round()}/${stats.targetRate.round()}/${stats.sampleCount})'
    : '(—/—/—)';

class RedRuleOptionLabel extends StatelessWidget {
  const RedRuleOptionLabel({super.key, required this.option});
  final T0RedRuleHistoryOption option;
  @override
  Widget build(BuildContext context) => Tooltip(
    message:
        '${option.name}\n${option.description}\n真赚率/达标率/样本数 ${redRuleNumbers(option.stats)}',
    child: Row(
      children: [
        Expanded(
          child: Text(
            option.name,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        const SizedBox(width: 8),
        Text(
          redRuleNumbers(option.stats),
          style: const TextStyle(fontSize: 12),
        ),
      ],
    ),
  );
}

void showRedRuleMatches(BuildContext context, List<T0RedRuleHit> hits) {
  if (hits.isEmpty) return;
  showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    builder: (context) => SizedBox(
      height: MediaQuery.sizeOf(context).height * .72,
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    '命中${hits.length}组条件',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                IconButton(
                  onPressed: () => Navigator.pop(context),
                  icon: const Icon(Icons.close),
                  tooltip: '关闭命中条件',
                ),
              ],
            ),
          ),
          Expanded(
            child: ListView.separated(
              padding: const EdgeInsets.only(bottom: 24),
              itemCount: hits.length,
              separatorBuilder: (_, index) => const Divider(height: 1),
              itemBuilder: (_, index) {
                final h = hits[index];
                final s = h.stats;
                final v =
                    h.statsStatus == 'verified' &&
                    s != null &&
                    s.sampleCount > 0 &&
                    s.exitHorizon == h.exitHorizon &&
                    (s.entrySource == h.entrySource ||
                        (h.entrySource == 'realtime_open' &&
                            s.entrySource == 'daily_open') ||
                        (h.entrySource == 'realtime_close' &&
                            s.entrySource == 'daily_close'));
                return ListTile(
                  title: Text(h.name),
                  subtitle: Text(
                    '${redRuleNumbers(s, verified: v)} · 真赚/达标/样本\n${v
                        ? '真赚率 ${s.profitWinRate.toStringAsFixed(2)}% · 达标率 ${s.targetRate.toStringAsFixed(2)}% · ${s.sampleCount}样本\n覆盖：${s.dateStart}～${s.dateEnd}'
                        : h.statsStatus == 'preopen'
                        ? '待竞价确认'
                        : '统计待核验'}\n${h.description}',
                  ),
                );
              },
            ),
          ),
        ],
      ),
    ),
  );
}

class RedRuleMatchBadge extends StatelessWidget {
  const RedRuleMatchBadge({super.key, required this.hits});
  final List<T0RedRuleHit> hits;
  @override
  Widget build(BuildContext context) => InkWell(
    onTap: () => showRedRuleMatches(context, hits),
    onLongPress: () => showRedRuleMatches(context, hits),
    child: Padding(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 4),
      child: Text(
        '命中${hits.length}组',
        style: TextStyle(
          color: Theme.of(context).colorScheme.primary,
          fontSize: 12,
        ),
      ),
    ),
  );
}
