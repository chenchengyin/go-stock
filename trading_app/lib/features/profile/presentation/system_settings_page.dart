import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../../auth/presentation/auth_view_model.dart';

class SystemSettingsPage extends StatelessWidget {
  const SystemSettingsPage({super.key});

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();

    return Container(
      color: CupertinoColors.systemGroupedBackground,
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
                    color: CupertinoColors.white,
                    child: const Column(
                      children: [
                        _SettingsItem(
                          icon: CupertinoIcons.paintbrush,
                          title: '主题',
                          subtitle: '浅色/深色模式',
                          showDivider: true,
                        ),
                        _SettingsItem(
                          icon: CupertinoIcons.wifi,
                          title: '弱网策略',
                          subtitle: '自动切换数据源/缓存模式',
                          showDivider: true,
                        ),
                        _SettingsItem(
                          icon: CupertinoIcons.clock,
                          title: '数据刷新频率',
                          subtitle: '30秒/60秒/120秒',
                          showDivider: false,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),
                  if (auth.isLoggedIn)
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      child: CupertinoButton(
                        color: CupertinoColors.white,
                        borderRadius: BorderRadius.circular(12),
                        child: const Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Icon(CupertinoIcons.square_arrow_right,
                                size: 20, color: CupertinoColors.systemRed),
                            SizedBox(width: 6),
                            Text('退出登录',
                                style: TextStyle(
                                    fontSize: 16,
                                    color: CupertinoColors.systemRed)),
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
                                  onPressed: () {
                                    Navigator.of(ctx).pop();
                                    auth.logout();
                                    Navigator.of(context).pop();
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
    return Container(
      height: 44,
      color: CupertinoColors.systemGroupedBackground,
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
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final bool showDivider;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              Icon(icon, size: 22, color: CupertinoColors.systemGrey),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title,
                        style: const TextStyle(
                            fontWeight: FontWeight.w500, fontSize: 15)),
                    const SizedBox(height: 2),
                    Text(subtitle,
                        style: const TextStyle(
                            fontSize: 12,
                            color: CupertinoColors.systemGrey)),
                  ],
                ),
              ),
              const Icon(CupertinoIcons.chevron_right,
                  size: 16, color: CupertinoColors.systemGrey3),
            ],
          ),
        ),
        if (showDivider)
          Container(
            margin: const EdgeInsets.only(left: 16, right: 16),
            height: 0.5,
            color: CupertinoColors.systemGrey5,
          ),
      ],
    );
  }
}
