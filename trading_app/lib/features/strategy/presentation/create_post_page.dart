import 'dart:io';
import 'package:flutter/cupertino.dart';
import 'package:image_picker/image_picker.dart';
import 'package:dio/dio.dart';
import 'package:provider/provider.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/network/api_client.dart';
import 'strategy_view_model.dart';

class CreatePostPage extends StatefulWidget {
  const CreatePostPage({super.key});

  @override
  State<CreatePostPage> createState() => _CreatePostPageState();
}

class _CreatePostPageState extends State<CreatePostPage> {
  final _contentController = TextEditingController();
  final _titleController = TextEditingController();
  final List<XFile> _selectedImages = [];
  bool _isSubmitting = false;
  bool _isUploading = false;

  @override
  void dispose() {
    _contentController.dispose();
    _titleController.dispose();
    super.dispose();
  }

  Future<void> _pickImage() async {
    final picker = ImagePicker();
    final file = await picker.pickImage(
      source: ImageSource.gallery,
      maxWidth: 1920,
      maxHeight: 1920,
    );
    if (file != null && _selectedImages.length < 6) {
      setState(() => _selectedImages.add(file));
    }
  }

  Future<String?> _uploadImage(XFile file) async {
    try {
      final dio = createApiClient();
      final formData = FormData.fromMap({
        'file': await MultipartFile.fromFile(file.path, filename: file.name),
      });
      final resp = await dio.post('/api/upload', data: formData);
      return resp.data['url'] as String?;
    } catch (e) {
      debugPrint('上传失败: $e');
      return null;
    }
  }

  Future<void> _submit() async {
    if (_titleController.text.trim().isEmpty) {
      _showToast('请输入标题');
      return;
    }
    if (_contentController.text.trim().isEmpty && _selectedImages.isEmpty) {
      _showToast('请输入内容');
      return;
    }

    final vm = context.read<StrategyViewModel>();

    setState(() {
      _isSubmitting = true;
      _isUploading = _selectedImages.isNotEmpty;
    });

    final uploadedUrls = <String>[];
    for (final img in _selectedImages) {
      final url = await _uploadImage(img);
      if (url != null) {
        uploadedUrls.add(url);
      }
    }
    setState(() => _isUploading = false);

    final post = await vm.createPost(
      title: _titleController.text.trim(),
      content: _contentController.text.trim(),
      images: uploadedUrls,
    );

    setState(() => _isSubmitting = false);

    if (!mounted) return;

    if (post != null && mounted) {
      Navigator.of(context).pop(true);
    } else if (mounted) {
      _showToast('发布失败，请检查网络连接后重试');
    }
  }

  void _showToast(String msg) {
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
    AppColors.of(context);
    return Stack(
      children: [
        CupertinoPageScaffold(
          backgroundColor: AppColors.scaffoldBg,
          navigationBar: CupertinoNavigationBar(
            backgroundColor: AppColors.appBarBg,
            leading: GestureDetector(
              onTap: () => Navigator.of(context).pop(),
              child: const Padding(
                padding: EdgeInsets.all(8),
                child: Icon(CupertinoIcons.clear_thick, size: 20),
              ),
            ),
            middle: const Text(
              '发布策略',
              style: TextStyle(fontWeight: FontWeight.w600),
            ),
            trailing: GestureDetector(
              onTap: _isSubmitting ? null : _submit,
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 6,
                ),
                decoration: BoxDecoration(
                  color: _isSubmitting ? AppColors.surfaceBg : AppColors.brand,
                  borderRadius: BorderRadius.circular(16),
                ),
                child: const Text(
                  '发布',
                  style: TextStyle(color: CupertinoColors.white, fontSize: 14),
                ),
              ),
            ),
          ),
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                CupertinoTextField(
                  controller: _titleController,
                  placeholder: '标题（必填）',
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: AppColors.inputFill,
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                const SizedBox(height: 12),
                CupertinoTextField(
                  controller: _contentController,
                  placeholder: '分享你的策略思路...（必填）',
                  maxLines: 8,
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: AppColors.inputFill,
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                const SizedBox(height: 16),
                Text(
                  '配图（最多6张）',
                  style: TextStyle(
                    fontSize: 13,
                    color: AppColors.textSecondary,
                  ),
                ),
                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    ..._selectedImages.map(
                      (file) => Stack(
                        children: [
                          ClipRRect(
                            borderRadius: BorderRadius.circular(8),
                            child: Image.file(
                              File(file.path),
                              width: 80,
                              height: 80,
                              fit: BoxFit.cover,
                            ),
                          ),
                          Positioned(
                            top: -4,
                            right: -4,
                            child: GestureDetector(
                              onTap: () =>
                                  setState(() => _selectedImages.remove(file)),
                              child: const Icon(
                                CupertinoIcons.xmark_circle_fill,
                                size: 20,
                                color: CupertinoColors.systemRed,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    if (_selectedImages.length < 6)
                      GestureDetector(
                        onTap: _pickImage,
                        child: Container(
                          width: 80,
                          height: 80,
                          decoration: BoxDecoration(
                            color: AppColors.inputFill,
                            borderRadius: BorderRadius.circular(8),
                          ),
                          child: Icon(
                            CupertinoIcons.camera_viewfinder,
                            color: AppColors.textTertiary,
                            size: 28,
                          ),
                        ),
                      ),
                  ],
                ),
                if (_isUploading) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      CupertinoActivityIndicator(),
                      SizedBox(width: 8),
                      Text(
                        '正在上传图片...',
                        style: TextStyle(
                          fontSize: 13,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ),
        if (_isSubmitting)
          Container(
            color: AppColors.scaffoldBg.withValues(alpha: 0.6),
            child: const Center(child: CupertinoActivityIndicator(radius: 16)),
          ),
      ],
    );
  }
}
