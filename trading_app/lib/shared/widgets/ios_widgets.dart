/// Common iOS-style widgets for consistent UI across the app.
library;

import 'package:flutter/material.dart';

/// Importance badge widget - iOS style rounded pill.
class ImportanceBadge extends StatelessWidget {
  const ImportanceBadge({
    super.key,
    required this.importance,
    this.small = false,
  });

  final dynamic importance;
  final bool small;

  Color _getColor() {
    switch (importance) {
      case 'high':
      case '重要':
        return const Color(0xffd93025);
      case 'medium':
      case '关注':
        return const Color(0xfff29900);
      case 'low':
      case '普通':
        return const Color(0xff5f6368);
      default:
        return const Color(0xff5f6368);
    }
  }

  String _getLabel() {
    switch (importance) {
      case 'high':
        return '重要';
      case 'medium':
        return '关注';
      case 'low':
        return '普通';
      default:
        return importance.toString();
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = _getColor();
    final label = _getLabel();
    final fontSize = small ? 10.0 : 11.0;
    final padding = small
        ? const EdgeInsets.symmetric(horizontal: 6, vertical: 2)
        : const EdgeInsets.symmetric(horizontal: 8, vertical: 4);

    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: color,
          fontSize: fontSize,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

/// AI summary block - iOS style blue info box.
class AISummaryBlock extends StatelessWidget {
  const AISummaryBlock({
    super.key,
    required this.summary,
    this.maxLines = 3,
  });

  final String summary;
  final int maxLines;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xffe8f0fe),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xffaecbfa)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.auto_awesome_outlined, size: 16, color: const Color(0xff1967d2)),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              summary,
              style: const TextStyle(
                color: Color(0xff1967d2),
                fontSize: 13,
                height: 1.4,
              ),
              maxLines: maxLines,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }
}

/// Time label widget - iOS style compact time display.
class TimeLabel extends StatelessWidget {
  const TimeLabel({
    super.key,
    required this.dateTime,
    this.showSeconds = false,
  });

  final DateTime dateTime;
  final bool showSeconds;

  @override
  Widget build(BuildContext context) {
    final month = dateTime.month.toString().padLeft(2, '0');
    final day = dateTime.day.toString().padLeft(2, '0');
    final hour = dateTime.hour.toString().padLeft(2, '0');
    final minute = dateTime.minute.toString().padLeft(2, '0');
    final second = dateTime.second.toString().padLeft(2, '0');

    final time = showSeconds ? '$month/$day $hour:$minute:$second' : '$month/$day $hour:$minute';

    return Text(
      time,
      style: TextStyle(
        color: const Color(0xff5f6368),
        fontSize: 12,
        fontFeatures: [const FontFeature.tabularFigures()],
      ),
    );
  }
}

/// Source chip widget - iOS style compact chip.
class SourceChip extends StatelessWidget {
  const SourceChip({
    super.key,
    required this.source,
    this.icon,
  });

  final String source;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    return Chip(
      label: Text(source, style: const TextStyle(fontSize: 11)),
      visualDensity: VisualDensity.compact,
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      avatar: icon != null ? Icon(icon, size: 14) : null,
    );
  }
}

/// Section header widget - iOS style section title.
class SectionHeader extends StatelessWidget {
  const SectionHeader({
    super.key,
    required this.title,
  });

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 16, bottom: 8),
      child: Text(
        title,
        style: const TextStyle(
          fontSize: 13,
          fontWeight: FontWeight.w600,
          color: Color(0xff5f6368),
          letterSpacing: 0.3,
        ),
      ),
    );
  }
}

/// Key points list widget - iOS style bullet list.
class KeyPointsList extends StatelessWidget {
  const KeyPointsList({
    super.key,
    required this.points,
  });

  final List<String> points;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: points.map((point) {
        return Padding(
          padding: const EdgeInsets.only(bottom: 4),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('• ', style: TextStyle(fontSize: 16, color: Color(0xff1967d2))),
              Expanded(
                child: Text(
                  point,
                  style: const TextStyle(fontSize: 13, height: 1.4),
                ),
              ),
            ],
          ),
        );
      }).toList(),
    );
  }
}

/// iOS style divider.
class IOSDivider extends StatelessWidget {
  const IOSDivider({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Divider(height: 1, color: const Color(0xffe0e0e0)),
    );
  }
}

/// Expandable/collapsible section - iOS style disclosure.
class ExpandableSection extends StatefulWidget {
  const ExpandableSection({
    super.key,
    required this.title,
    required this.children,
    this.initialExpanded = false,
  });

  final String title;
  final List<Widget> children;
  final bool initialExpanded;

  @override
  State<ExpandableSection> createState() => _ExpandableSectionState();
}

class _ExpandableSectionState extends State<ExpandableSection> {
  late bool _expanded;

  @override
  void initState() {
    super.initState();
    _expanded = widget.initialExpanded;
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          onTap: () => setState(() => _expanded = !_expanded),
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: Row(
              children: [
                Icon(
                  _expanded ? Icons.keyboard_arrow_down : Icons.keyboard_arrow_right,
                  size: 20,
                  color: const Color(0xff1967d2),
                ),
                const SizedBox(width: 4),
                Text(
                  widget.title,
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                    color: Color(0xff1967d2),
                  ),
                ),
              ],
            ),
          ),
        ),
        if (_expanded) ...widget.children,
      ],
    );
  }
}
