import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';

import '../../../shared/view_state.dart';
import '../../auth/presentation/auth_view_model.dart';
import '../../auth/presentation/register_page.dart';

class LoginPage extends StatefulWidget {
  const LoginPage({super.key});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _phoneController = TextEditingController(text: '13800000000');
  final _passwordController = TextEditingController(text: 'secret123');

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _showToast('请先登录');
    });
  }

  @override
  void dispose() {
    _phoneController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  void _showToast(String msg) {
    if (!mounted) return;
    showCupertinoDialog(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: Text(msg),
        actions: [
          CupertinoDialogAction(
            child: const Text('确定'),
            onPressed: () => Navigator.of(ctx).pop(),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();
    AppColors.of(context);

    return Scaffold(
      backgroundColor: AppColors.scaffoldBg,
      appBar: CupertinoNavigationBar(
        backgroundColor: AppColors.appBarBg,
        middle: Text('登录'),
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: 40),
              const Text(
                '欢迎回来',
                style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 40),
              CupertinoTextField(
                controller: _phoneController,
                placeholder: '请输入手机号/账号',
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: AppColors.inputFill,
                  borderRadius: BorderRadius.circular(12),
                ),
              ),
              const SizedBox(height: 16),
              CupertinoTextField(
                controller: _passwordController,
                placeholder: '请输入密码',
                obscureText: true,
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: AppColors.inputFill,
                  borderRadius: BorderRadius.circular(12),
                ),
              ),
              const SizedBox(height: 24),
              if (auth.state.status == ViewStatus.error)
                Padding(
                  padding: const EdgeInsets.only(bottom: 16),
                  child: Text(
                    auth.state.message,
                    style: const TextStyle(color: CupertinoColors.systemRed),
                    textAlign: TextAlign.center,
                  ),
                ),
              CupertinoButton.filled(
                color: AppColors.buttonPrimary,
                onPressed: auth.state.isLoading
                    ? null
                    : () async {
                        await auth.login(
                          phone: _phoneController.text,
                          password: _passwordController.text,
                        );
                        if (context.mounted && auth.isLoggedIn) {
                          Navigator.of(context).pop();
                        }
                      },
                child: auth.state.isLoading
                    ? const CupertinoActivityIndicator()
                    : const Text('登录'),
              ),
              const SizedBox(height: 16),
              CupertinoButton(
                onPressed: () {
                  Navigator.of(context).push(
                    CupertinoPageRoute(builder: (_) => const RegisterPage()),
                  );
                },
                child: const Text('还没有账号？去注册'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
