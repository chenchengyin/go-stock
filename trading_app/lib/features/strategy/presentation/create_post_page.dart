import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import 'strategy_view_model.dart';

class CreatePostPage extends StatefulWidget {
  const CreatePostPage({super.key});

  @override
  State<CreatePostPage> createState() => _CreatePostPageState();
}

class _CreatePostPageState extends State<CreatePostPage> {
  final _contentController = TextEditingController();
  final _titleController = TextEditingController();
  final List<String> _images = [];
  bool _isSubmitting = false;

  @override
  void dispose() {
    _contentController.dispose();
    _titleController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_contentController.text.trim().isEmpty && _images.isEmpty) {
      _showToast('请输入内容或选择图片');
      return;
    }

    setState(() => _isSubmitting = true);

    final vm = context.read<StrategyViewModel>();
    final post = await vm.createPost(
      title: _titleController.text.trim(),
      content: _contentController.text.trim(),
      images: _images,
    );

    setState(() => _isSubmitting = false);

    if (post != null && mounted) {
      _showToast('发布成功');
      Navigator.of(context).pop(true);
    } else if (mounted) {
      _showToast('发布失败');
    }
  }

  void _showToast(String msg) {
    showCupertinoDialog(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: Text(msg),
        actions: [CupertinoDialogAction(child: const Text('确定'), onPressed: () => Navigator.of(ctx).pop())],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return CupertinoPageScaffold(
      backgroundColor: CupertinoColors.white,
      navigationBar: CupertinoNavigationBar(
        backgroundColor: CupertinoColors.white,
        leading: GestureDetector(
          onTap: () => Navigator.of(context).pop(),
          child: const Padding(
            padding: EdgeInsets.all(8),
            child: Icon(CupertinoIcons.clear_thick, size: 20),
          ),
        ),
        middle: const Text('发布策略', style: TextStyle(fontWeight: FontWeight.w600)),
        trailing: GestureDetector(
          onTap: _isSubmitting ? null : _submit,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
            decoration: BoxDecoration(
              color: _isSubmitting ? CupertinoColors.systemGrey5 : const Color(0xff2364aa),
              borderRadius: BorderRadius.circular(16),
            ),
            child: _isSubmitting
                ? const SizedBox(width: 16, height: 16, child: CupertinoActivityIndicator())
                : const Text('发布', style: TextStyle(color: CupertinoColors.white, fontSize: 14)),
          ),
        ),
      ),
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 标题
            CupertinoTextField(
              controller: _titleController,
              placeholder: '标题（可选）',
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: CupertinoColors.systemGrey6,
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            const SizedBox(height: 12),
            // 内容
            CupertinoTextField(
              controller: _contentController,
              placeholder: '分享你的策略思路...',
              maxLines: 8,
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: CupertinoColors.systemGrey6,
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            const SizedBox(height: 16),
            // 图片（暂时不支持上传，后续对接）
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: CupertinoColors.systemGrey6,
                borderRadius: BorderRadius.circular(8),
              ),
              child: const Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(CupertinoIcons.photo_on_rectangle, color: CupertinoColors.systemGrey, size: 20),
                  SizedBox(width: 8),
                  Text('图片功能开发中', style: TextStyle(fontSize: 13, color: CupertinoColors.systemGrey)),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
