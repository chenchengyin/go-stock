import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';

/// A presentational shell: callers compose their own header and content panels.
class StrategyCard extends StatelessWidget {
  const StrategyCard({
    super.key,
    required this.header,
    this.sections = const [],
    this.onTap,
  });

  final Widget header;
  final List<Widget> sections;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => GestureDetector(
    behavior: HitTestBehavior.opaque,
    onTap: onTap,
    child: Container(
      margin: const EdgeInsets.only(bottom: 4),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [header, ...sections],
      ),
    ),
  );
}

/// Information and actions may be independently composed or reordered.
class StrategyCardHeader extends StatelessWidget {
  const StrategyCardHeader({
    super.key,
    required this.information,
    this.actions = const [],
    this.wrap = false,
  });

  final List<Widget> information;
  final List<Widget> actions;
  final bool wrap;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final informationWrap = Wrap(
        spacing: 6,
        runSpacing: 4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: information,
      );
      if (constraints.maxWidth < 680 || wrap) {
        return Wrap(
          spacing: 6,
          runSpacing: 4,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [...information, ...actions],
        );
      }
      return Row(
        children: [
          Expanded(child: informationWrap),
          if (actions.isNotEmpty) const SizedBox(width: 8),
          for (var i = 0; i < actions.length; i++) ...[
            actions[i],
            if (i < actions.length - 1) const SizedBox(width: 8),
          ],
        ],
      );
    },
  );
}
