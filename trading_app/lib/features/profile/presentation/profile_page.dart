import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../../../shared/view_state.dart';
import '../../auth/presentation/auth_view_model.dart';

class ProfilePage extends StatefulWidget {
  const ProfilePage({super.key});

  @override
  State<ProfilePage> createState() => _ProfilePageState();
}

class _ProfilePageState extends State<ProfilePage> {
  final _phoneController = TextEditingController(text: '13800000000');
  final _passwordController = TextEditingController(text: 'secret123');
  final _nicknameController = TextEditingController(text: 'Colin');
  bool _isRegister = false;

  @override
  void dispose() {
    _phoneController.dispose();
    _passwordController.dispose();
    _nicknameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();
    return CupertinoPageScaffold(
      navigationBar: const CupertinoNavigationBar(
        middle: Text('我的'),
      ),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          children: [
            if (auth.isLoggedIn)
              _ProfileCard(auth: auth, nicknameController: _nicknameController)
            else
              _AuthCard(
                isRegister: _isRegister,
                phoneController: _phoneController,
                passwordController: _passwordController,
                nicknameController: _nicknameController,
                isLoading: auth.state.isLoading,
                errorMessage: auth.state.status == ViewStatus.error
                    ? auth.state.message
                    : '',
                onToggleMode: () =>
                    setState(() => _isRegister = !_isRegister),
                onSubmit: () {
                  if (_isRegister) {
                    auth.register(
                      phone: _phoneController.text,
                      password: _passwordController.text,
                      nickname: _nicknameController.text,
                    );
                  } else {
                    auth.login(
                        phone: _phoneController.text,
                        password: _passwordController.text);
                  }
                },
              ),
            const SizedBox(height: 12),
            // 设置列表
            Container(
              decoration: BoxDecoration(
                color: CupertinoColors.white,
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: CupertinoColors.systemGrey5),
              ),
              child: Column(
                children: [
                  _SettingsRow(
                    icon: CupertinoIcons.bell,
                    title: '推送设置',
                    subtitle: '环信/厂商推送适配预留',
                  ),
                  CupertinoTheme(
                    data: const CupertinoThemeData(brightness: Brightness.light),
                    child: Container(height: 1,
                        color: CupertinoColors.systemGrey5),
                  ),
                  _SettingsRow(
                    icon: CupertinoIcons.tray_full,
                    title: '本地缓存',
                    subtitle: '登录态、自选股、资讯、热榜、通知历史',
                  ),
                  Container(height: 1,
                      color: CupertinoColors.systemGrey5),
                  _SettingsRow(
                    icon: CupertinoIcons.settings,
                    title: '系统设置',
                    subtitle: '主题、弱网策略、数据刷新频率',
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

class _SettingsRow extends StatelessWidget {
  const _SettingsRow({
    required this.icon,
    required this.title,
    required this.subtitle,
  });

  final IconData icon;
  final String title;
  final String subtitle;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(14),
      child: Row(
        children: [
          Icon(icon, size: 24, color: CupertinoColors.systemGrey),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title,
                    style: const TextStyle(
                        fontWeight: FontWeight.w600, fontSize: 15)),
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
    );
  }
}

class _AuthCard extends StatelessWidget {
  const _AuthCard({
    required this.isRegister,
    required this.phoneController,
    required this.passwordController,
    required this.nicknameController,
    required this.isLoading,
    required this.errorMessage,
    required this.onToggleMode,
    required this.onSubmit,
  });

  final bool isRegister;
  final TextEditingController phoneController;
  final TextEditingController passwordController;
  final TextEditingController nicknameController;
  final bool isLoading;
  final String errorMessage;
  final VoidCallback onToggleMode;
  final VoidCallback onSubmit;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: CupertinoColors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: CupertinoColors.systemGrey5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            isRegister ? '注册交易雷达账号' : '登录交易雷达',
            style: const TextStyle(
                fontWeight: FontWeight.w700, fontSize: 18),
          ),
          const SizedBox(height: 16),
          CupertinoTextField(
            controller: phoneController,
            placeholder: '手机号/账号',
            prefix: const Padding(
              padding: EdgeInsets.only(left: 8),
              child: Icon(CupertinoIcons.phone, size: 20),
            ),
          ),
          const SizedBox(height: 10),
          CupertinoTextField(
            controller: passwordController,
            obscureText: true,
            placeholder: '密码',
            prefix: const Padding(
              padding: EdgeInsets.only(left: 8),
              child: Icon(CupertinoIcons.lock, size: 20),
            ),
          ),
          if (isRegister) ...[
            const SizedBox(height: 10),
            CupertinoTextField(
              controller: nicknameController,
              placeholder: '昵称',
              prefix: const Padding(
                padding: EdgeInsets.only(left: 8),
                child: Icon(CupertinoIcons.person, size: 20),
              ),
            ),
          ],
          if (errorMessage.isNotEmpty) ...[
            const SizedBox(height: 10),
            Text(errorMessage,
                style: const TextStyle(
                    color: CupertinoColors.systemRed)),
          ],
          const SizedBox(height: 16),
          CupertinoButton.filled(
            onPressed: isLoading ? null : onSubmit,
            child: isLoading
                ? const CupertinoActivityIndicator(
                    color: CupertinoColors.white)
                : Text(isRegister ? '注册并登录' : '登录'),
          ),
          CupertinoButton(
            onPressed: isLoading ? null : onToggleMode,
            child: Text(isRegister ? '已有账号，去登录' : '没有账号，去注册'),
          ),
        ],
      ),
    );
  }
}

class _ProfileCard extends StatefulWidget {
  const _ProfileCard({
    required this.auth,
    required this.nicknameController,
  });

  final AuthViewModel auth;
  final TextEditingController nicknameController;

  @override
  State<_ProfileCard> createState() => _ProfileCardState();
}

class _ProfileCardState extends State<_ProfileCard> {
  @override
  Widget build(BuildContext context) {
    final user = widget.auth.user!;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: CupertinoColors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: CupertinoColors.systemGrey5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: CupertinoColors.systemGrey5,
                  borderRadius: BorderRadius.circular(24),
                ),
                child: const Icon(CupertinoIcons.person,
                    size: 24, color: CupertinoColors.systemGrey),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(user.nickname,
                        style: const TextStyle(
                            fontWeight: FontWeight.w700, fontSize: 16)),
                    Text('${user.phone} · ${user.role}',
                        style: const TextStyle(
                            fontSize: 13,
                            color: CupertinoColors.systemGrey)),
                  ],
                ),
              ),
              CupertinoButton(
                padding: const EdgeInsets.symmetric(horizontal: 12),
                onPressed: widget.auth.logout,
                child: const Text('退出'),
              ),
            ],
          ),
          const SizedBox(height: 16),
          CupertinoTextField(
            controller: widget.nicknameController,
            placeholder: '修改昵称',
            prefix: const Padding(
              padding: EdgeInsets.only(left: 8),
              child: Icon(CupertinoIcons.pencil, size: 20),
            ),
          ),
          const SizedBox(height: 10),
          CupertinoButton.filled(
            onPressed: widget.auth.state.isLoading
                ? null
                : () =>
                    widget.auth.updateNickname(widget.nicknameController.text),
            child: Text(widget.auth.state.isLoading ? '保存中...' : '保存资料'),
          ),
        ],
      ),
    );
  }
}
