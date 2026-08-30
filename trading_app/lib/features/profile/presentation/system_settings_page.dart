import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/theme_manager.dart';
import '../../../core/utils/stock_launcher.dart';
import '../../auth/presentation/auth_view_model.dart';

class SystemSettingsPage extends StatelessWidget {
  const SystemSettingsPage({super.key});

  Future<void> _openInFlush(BuildContext context) async {
    final opened = await StockLauncher.openTongHuaShun(code: '601318');
    if (!opened && context.mounted) {
      showCupertinoDialog<void>(
        context: context,
        builder: (ctx) => CupertinoAlertDialog(
          title: const Text('无法打开'),
          content: const Text('未能跳转到同花顺或浏览器。'),
          actions: [
            CupertinoDialogAction(
              child: const Text('知道了'),
              onPressed: () => Navigator.of(ctx).pop(),
            ),
          ],
        ),
      );
    }
  }

  void _showThemePicker(BuildContext context) {
    final tm = context.read<ThemeManager>();
    final current = tm.variant;

    showCupertinoModalPopup(
      context: context,
      builder: (ctx) => CupertinoActionSheet(
        title: const Text('选择主题'),
        actions: [
          CupertinoActionSheetAction(
            isDefaultAction: current == AppThemeVariant.light,
            onPressed: () {
              tm.setVariant(AppThemeVariant.light);
              Navigator.of(ctx).pop();
            },
            child: const Text('浅色'),
          ),
          CupertinoActionSheetAction(
            isDefaultAction: current == AppThemeVariant.dark,
            onPressed: () {
              tm.setVariant(AppThemeVariant.dark);
              Navigator.of(ctx).pop();
            },
            child: const Text('耀夜'),
          ),
          CupertinoActionSheetAction(
            isDefaultAction: current == AppThemeVariant.grey,
            onPressed: () {
              tm.setVariant(AppThemeVariant.grey);
              Navigator.of(ctx).pop();
            },
            child: const Text('灰色'),
          ),
        ],
        cancelButton: CupertinoActionSheetAction(
          child: const Text('取消'),
          onPressed: () => Navigator.of(ctx).pop(),
        ),
      ),
    );
  }

  String _themeLabel(AppThemeVariant v) {
    switch (v) {
      case AppThemeVariant.light:
        return '浅色';
      case AppThemeVariant.dark:
        return '耀夜';
      case AppThemeVariant.grey:
        return '灰色';
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();
    final tm = context.watch<ThemeManager>();
    AppColors.of(context);

    return Container(
      color: AppColors.scaffoldBg,
      child: SafeArea(
        bottom: false,
        child: Column(
          children: [
            _SettingsHeader(onBack: () => Navigator.of(context).pop()),
            Expanded(
              child: ListView(
                padding: EdgeInsets.zero,
                children: [
                  const SizedBox(height: 8),
                  Container(
                    color: AppColors.cardBg,
                    child: Column(
                      children: [
                        _SettingsItem(
                          icon: CupertinoIcons.paintbrush,
                          title: '主题',
                          subtitle: _themeLabel(tm.variant),
                          showDivider: true,
                          onTap: () => _showThemePicker(context),
                        ),
                        const _SettingsItem(
                          icon: CupertinoIcons.wifi,
                          title: '弱网策略',
                          subtitle: '自动切换数据源/缓存模式',
                          showDivider: true,
                        ),
                        const _SettingsItem(
                          icon: CupertinoIcons.clock,
                          title: '数据刷新频率',
                          subtitle: '30秒/60秒/120秒',
                          showDivider: true,
                        ),
                        _SettingsItem(
                          icon: CupertinoIcons.arrow_right_arrow_left,
                          title: '跳转同花顺',
                          subtitle: '中国平安(601318)',
                          showDivider: false,
                          onTap: () => _openInFlush(context),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),
                  if (auth.isLoggedIn)
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      child: CupertinoButton(
                        color: AppColors.cardBg,
                        borderRadius: BorderRadius.circular(12),
                        child: const Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Icon(
                              CupertinoIcons.square_arrow_right,
                              size: 20,
                              color: CupertinoColors.systemRed,
                            ),
                            SizedBox(width: 6),
                            Text(
                              '退出登录',
                              style: TextStyle(
                                fontSize: 16,
                                color: CupertinoColors.systemRed,
                              ),
                            ),
                          ],
                        ),
                        onPressed: () {
                          showCupertinoDialog(
                            context: context,
                            builder: (ctx) => CupertinoAlertDialog(
                              title: const Text('确认退出'),
                              content: const Text('确定要退出当前账号吗？'),
                              actions: [
                                CupertinoDialogAction(
                                  child: const Text('取消'),
                                  onPressed: () => Navigator.of(ctx).pop(),
                                ),
                                CupertinoDialogAction(
                                  isDestructiveAction: true,
                                  onPressed: () async {
                                    Navigator.of(ctx).pop();
                                    await auth.logout();
                                    if (context.mounted &&
                                        Navigator.canPop(context)) {
                                      Navigator.of(context).pop();
                                    }
                                  },
                                  child: const Text('退出'),
                                ),
                              ],
                            ),
                          );
                        },
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SettingsHeader extends StatelessWidget {
  const _SettingsHeader({required this.onBack});
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return Container(
      height: 44,
      color: AppColors.appBarBg,
      child: Row(
        children: [
          CupertinoButton(
            onPressed: onBack,
            padding: EdgeInsets.zero,
            child: const Row(
              children: [
                SizedBox(width: 8),
                Icon(CupertinoIcons.back, size: 22),
                SizedBox(width: 4),
                Text('我的', style: TextStyle(fontSize: 17)),
              ],
            ),
          ),
          const Expanded(
            child: Text(
              '系统设置',
              style: TextStyle(fontSize: 17, fontWeight: FontWeight.w600),
              textAlign: TextAlign.center,
            ),
          ),
          const SizedBox(width: 60),
        ],
      ),
    );
  }
}

class _SettingsItem extends StatelessWidget {
  const _SettingsItem({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.showDivider,
    this.onTap,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final bool showDivider;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return Column(
      children: [
        GestureDetector(
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Icon(icon, size: 22, color: AppColors.textTertiary),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        title,
                        style: const TextStyle(
                          fontWeight: FontWeight.w500,
                          fontSize: 15,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        subtitle,
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  CupertinoIcons.chevron_right,
                  size: 16,
                  color: AppColors.textTertiary,
                ),
              ],
            ),
          ),
        ),
        if (showDivider)
          Container(
            margin: const EdgeInsets.only(left: 16, right: 16),
            height: 0.5,
            color: AppColors.divider,
          ),
      ],
    );
  }
}
