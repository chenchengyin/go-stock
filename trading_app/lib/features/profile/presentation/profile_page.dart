import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';

import '../../auth/presentation/auth_view_model.dart';
import '../../auth/presentation/login_page.dart';
import '../../auth/presentation/register_page.dart';
import 'edit_profile_page.dart';
import 'qgqp_bid_page.dart';
import 'system_settings_page.dart';

class ProfilePage extends StatefulWidget {
  const ProfilePage({super.key});

  @override
  State<ProfilePage> createState() => _ProfilePageState();
}

class _ProfilePageState extends State<ProfilePage> {
  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();
    AppColors.of(context);
    return Container(
      color: AppColors.scaffoldBg,
      child: SafeArea(
        bottom: false,
        child: Column(
          children: [
            const _Header(),
            Expanded(
              child: ListView(
                padding: EdgeInsets.zero,
                children: [
                  if (auth.isLoggedIn)
                    _ProfileCard(auth: auth)
                  else
                    _LoginPrompt(),
                  const _SectionHeader(title: '设置'),
                  Container(
                    color: AppColors.cardBg,
                    child: Column(
                      children: [
                        _SettingsRow(
                          icon: CupertinoIcons.bell,
                          title: '推送设置',
                          subtitle: '环信/厂商推送适配预留',
                          showDivider: true,
                        ),
                        _SettingsRow(
                          icon: CupertinoIcons.chart_bar,
                          title: '东财标识',
                          subtitle: '用于选股功能',
                          showDivider: true,
                          onTap: () {
                            Navigator.of(context).push(
                              CupertinoPageRoute(
                                builder: (_) => const _QgqpBidPageWrapper(),
                              ),
                            );
                          },
                        ),
                        _SettingsRow(
                          icon: CupertinoIcons.tray_full,
                          title: '本地缓存',
                          subtitle: '登录态、自选股、资讯、热榜、通知历史',
                          showDivider: true,
                        ),
                        _SettingsRow(
                          icon: CupertinoIcons.settings,
                          title: '系统设置',
                          subtitle: '主题、弱网策略、数据刷新频率',
                          showDivider: false,
                          onTap: () {
                            Navigator.of(context).push(
                              CupertinoPageRoute(
                                builder: (_) => const SystemSettingsPage(),
                              ),
                            );
                          },
                        ),
                      ],
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

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.title});
  final String title;

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 24, 16, 8),
      child: Text(
        title,
        style: TextStyle(
          fontSize: 13,
          fontWeight: FontWeight.w600,
          color: AppColors.textSecondary,
          letterSpacing: 0.3,
        ),
      ),
    );
  }
}

class _SettingsRow extends StatelessWidget {
  const _SettingsRow({
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
    return GestureDetector(
      onTap: onTap,
      child: Column(
        children: [
          Padding(
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
                        style: TextStyle(
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
          if (showDivider)
            Container(
              margin: const EdgeInsets.only(left: 16, right: 16),
              height: 0.5,
              color: AppColors.divider,
            ),
        ],
      ),
    );
  }
}

class _LoginPrompt extends StatelessWidget {
  const _LoginPrompt();

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return Container(
      color: AppColors.cardBg,
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // const Text(
          //   '登录掌盘',
          //   style: TextStyle(fontWeight: FontWeight.w600, fontSize: 18),
          // ),
          const SizedBox(height: 16),
          CupertinoButton.filled(
            color: AppColors.buttonPrimary,
            onPressed: () {
              Navigator.of(
                context,
              ).push(CupertinoPageRoute(builder: (_) => const LoginPage()));
            },
            child: const Text('登录'),
          ),
          const SizedBox(height: 8),
          CupertinoButton(
            onPressed: () {
              Navigator.of(
                context,
              ).push(CupertinoPageRoute(builder: (_) => const RegisterPage()));
            },
            child: const Text('没有账号？去注册'),
          ),
        ],
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header();

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return Container(
      height: 44,
      color: AppColors.appBarBg,
      alignment: Alignment.center,
      child: const Text(
        '我的',
        style: TextStyle(fontSize: 17, fontWeight: FontWeight.w600),
      ),
    );
  }
}

class _ProfileCard extends StatefulWidget {
  const _ProfileCard({required this.auth});

  final AuthViewModel auth;

  @override
  State<_ProfileCard> createState() => _ProfileCardState();
}

class _ProfileCardState extends State<_ProfileCard> {
  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    final user = widget.auth.user!;
    return GestureDetector(
      onTap: () {
        // Navigator.of(context).push(
        //   PageRouteBuilder(
        //     pageBuilder: (context, animation, secondaryAnimation) => EditProfilePage(),
        //   ),
        // );
        Navigator.of(
          context,
        ).push(CupertinoPageRoute(builder: (_) => const EditProfilePage()));
      },
      child: Container(
        color: AppColors.cardBg,
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: AppColors.surfaceBg,
                borderRadius: BorderRadius.circular(24),
              ),
              child: Icon(
                CupertinoIcons.person,
                size: 24,
                color: AppColors.textTertiary,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    user.nickname,
                    style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${user.phone} · ${user.role}',
                    style: TextStyle(
                      fontSize: 13,
                      color: AppColors.textSecondary,
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

/// 包装 QgqpBidPage 以兼容 profile_page 的样式
class _QgqpBidPageWrapper extends StatelessWidget {
  const _QgqpBidPageWrapper();

  @override
  Widget build(BuildContext context) {
    return const QgqpBidPage();
  }
}
