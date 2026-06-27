import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../../../shared/view_state.dart';
import '../../../auth/presentation/view_models/auth_view_model.dart';

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
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      children: [
        AppBar(title: const Text('我的')),
        const SizedBox(height: 8),
        if (auth.isLoggedIn) _ProfileCard(auth: auth) else _AuthCard(
          isRegister: _isRegister,
          phoneController: _phoneController,
          passwordController: _passwordController,
          nicknameController: _nicknameController,
          isLoading: auth.state.isLoading,
          errorMessage: auth.state.status == ViewStatus.error ? auth.state.message : '',
          onToggleMode: () => setState(() => _isRegister = !_isRegister),
          onSubmit: () {
            if (_isRegister) {
              auth.register(
                phone: _phoneController.text,
                password: _passwordController.text,
                nickname: _nicknameController.text,
              );
            } else {
              auth.login(phone: _phoneController.text, password: _passwordController.text);
            }
          },
        ),
        const SizedBox(height: 8),
        const Card(
          child: Column(
            children: [
              ListTile(leading: Icon(Icons.notifications_outlined), title: Text('推送设置'), subtitle: Text('环信/厂商推送适配预留')),
              Divider(height: 1),
              ListTile(leading: Icon(Icons.storage_outlined), title: Text('本地缓存'), subtitle: Text('登录态、自选股、资讯、热榜、通知历史')),
              Divider(height: 1),
              ListTile(leading: Icon(Icons.settings_outlined), title: Text('系统设置'), subtitle: Text('主题、弱网策略、数据刷新频率')),
            ],
          ),
        ),
      ],
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
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(isRegister ? '注册交易雷达账号' : '登录交易雷达', style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700)),
            const SizedBox(height: 12),
            TextField(
              controller: phoneController,
              keyboardType: TextInputType.phone,
              decoration: const InputDecoration(labelText: '手机号/账号', prefixIcon: Icon(Icons.phone_android_outlined)),
            ),
            const SizedBox(height: 10),
            TextField(
              controller: passwordController,
              obscureText: true,
              decoration: const InputDecoration(labelText: '密码', prefixIcon: Icon(Icons.lock_outline)),
            ),
            if (isRegister) ...[
              const SizedBox(height: 10),
              TextField(
                controller: nicknameController,
                decoration: const InputDecoration(labelText: '昵称', prefixIcon: Icon(Icons.badge_outlined)),
              ),
            ],
            if (errorMessage.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text(errorMessage, style: const TextStyle(color: Color(0xffd93025))),
            ],
            const SizedBox(height: 14),
            FilledButton.icon(
              onPressed: isLoading ? null : onSubmit,
              icon: isLoading ? const SizedBox.square(dimension: 16, child: CircularProgressIndicator(strokeWidth: 2)) : const Icon(Icons.login),
              label: Text(isRegister ? '注册并登录' : '登录'),
            ),
            TextButton(onPressed: isLoading ? null : onToggleMode, child: Text(isRegister ? '已有账号，去登录' : '没有账号，去注册')),
          ],
        ),
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
  late final TextEditingController _nicknameController;

  @override
  void initState() {
    super.initState();
    _nicknameController = TextEditingController(text: widget.auth.user?.nickname ?? '');
  }

  @override
  void dispose() {
    _nicknameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final user = widget.auth.user!;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                const CircleAvatar(child: Icon(Icons.person_outline)),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(user.nickname, style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700)),
                      Text('${user.phone} · ${user.role}'),
                    ],
                  ),
                ),
                TextButton(onPressed: widget.auth.logout, child: const Text('退出')),
              ],
            ),
            const SizedBox(height: 14),
            TextField(
              controller: _nicknameController,
              decoration: const InputDecoration(labelText: '修改昵称', prefixIcon: Icon(Icons.edit_outlined)),
            ),
            const SizedBox(height: 10),
            OutlinedButton.icon(
              onPressed: widget.auth.state.isLoading ? null : () => widget.auth.updateNickname(_nicknameController.text),
              icon: const Icon(Icons.save_outlined),
              label: const Text('保存资料'),
            ),
          ],
        ),
      ),
    );
  }
}
