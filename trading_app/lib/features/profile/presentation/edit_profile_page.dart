import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../../../shared/view_state.dart';
import '../../auth/presentation/auth_view_model.dart';

class EditProfilePage extends StatefulWidget {
  const EditProfilePage({super.key});

  @override
  State<EditProfilePage> createState() => _EditProfilePageState();
}

class _EditProfilePageState extends State<EditProfilePage> {
  late final TextEditingController _nicknameController;

  @override
  void initState() {
    super.initState();
    final user = context.read<AuthViewModel>().user;
    _nicknameController = TextEditingController(text: user?.nickname ?? '');
  }

  @override
  void dispose() {
    _nicknameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();
    final user = auth.user;
    if (user == null) {
      return const CupertinoPageScaffold(
        navigationBar: CupertinoNavigationBar(middle: Text('编辑资料')),
        child: Center(child: Text('用户未登录')),
      );
    }

    return CupertinoPageScaffold(
      backgroundColor: CupertinoColors.systemGroupedBackground,
      navigationBar: const CupertinoNavigationBar(
        middle: Text('编辑资料'),
      ),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            // 头像区域
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: CupertinoColors.white,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Row(
                children: [
                  Container(
                    width: 64,
                    height: 64,
                    decoration: BoxDecoration(
                      color: CupertinoColors.systemGrey5,
                      borderRadius: BorderRadius.circular(32),
                    ),
                    child: const Icon(CupertinoIcons.person,
                        size: 32, color: CupertinoColors.systemGrey),
                  ),
                  const SizedBox(width: 12),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(user.nickname,
                          style: const TextStyle(
                              fontWeight: FontWeight.w600, fontSize: 18)),
                      const SizedBox(height: 4),
                      Text(
                        '${user.phone} · ${user.role}',
                        style: const TextStyle(
                            fontSize: 14,
                            color: CupertinoColors.systemGrey),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),

            // 昵称编辑
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: CupertinoColors.white,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('昵称',
                      style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: CupertinoColors.systemGrey)),
                  const SizedBox(height: 8),
                  CupertinoTextField(
                    controller: _nicknameController,
                    placeholder: '请输入昵称',
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: CupertinoColors.systemGrey6,
                      borderRadius: BorderRadius.circular(8),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 32),

            // 保存按钮
            CupertinoButton.filled(
              onPressed: auth.state.isLoading
                  ? null
                  : () async {
                      final name = _nicknameController.text.trim();
                      if (name.isEmpty) {
                        showCupertinoDialog(
                          context: context,
                          builder: (ctx) => CupertinoAlertDialog(
                            title: const Text('昵称不能为空'),
                            actions: [
                              CupertinoDialogAction(
                                child: const Text('确定'),
                                onPressed: () => Navigator.of(ctx).pop(),
                              ),
                            ],
                          ),
                        );
                        return;
                      }
                      await auth.updateNickname(name);
                      if (context.mounted) {
                        Navigator.of(context).pop();
                      }
                    },
              child: auth.state.isLoading
                  ? const CupertinoActivityIndicator(
                      color: CupertinoColors.white)
                  : const Text('保存资料'),
            ),

            if (auth.state.status == ViewStatus.error)
              Padding(
                padding: const EdgeInsets.only(top: 16),
                child: Text(
                  auth.state.message,
                  style: const TextStyle(
                      color: CupertinoColors.systemRed, fontSize: 14),
                  textAlign: TextAlign.center,
                ),
              ),
          ],
        ),
      ),
    );
  }
}
