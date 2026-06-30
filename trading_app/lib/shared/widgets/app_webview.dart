import 'package:flutter/material.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart';

/// 通用 WebView 组件
class AppWebView extends StatefulWidget {
  final String url;
  final String? title;
  final void Function(String url, String title)? onPageFinished;
  final void Function(String? title)? onTitleChanged;

  const AppWebView({
    super.key,
    required this.url,
    this.title,
    this.onPageFinished,
    this.onTitleChanged,
  });

  @override
  State<AppWebView> createState() => _AppWebViewState();
}

class _AppWebViewState extends State<AppWebView> {
  late InAppWebViewController _controller;
  bool _isLoading = true;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.title ?? '加载中...'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.pop(context),
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: () => _controller.reload(),
          ),
        ],
      ),
      body: Stack(
        children: [
          InAppWebView(
            initialUrlRequest: URLRequest(url: WebUri(widget.url)),
            initialOptions: InAppWebViewGroupOptions(
              crossPlatform: InAppWebViewOptions(
                javaScriptEnabled: true,
                cacheEnabled: true,
                supportZoom: true,
              ),
            ),
            onWebViewCreated: (controller) {
              _controller = controller;
            },
            onLoadStart: (controller, url) {
              setState(() => _isLoading = true);
            },
            onLoadStop: (controller, url) async {
              setState(() => _isLoading = false);

              // 获取页面标题
              String? title = await controller.getTitle();
              if (title != null && title.isNotEmpty) {
                widget.onTitleChanged?.call(title);
              }

              widget.onPageFinished?.call(url?.toString() ?? '', title ?? '');
            },
            onConsoleMessage: (controller, consoleMessage) {
              debugPrint('WebView Console: ${consoleMessage.message}');
            },
          ),
          if (_isLoading)
            const Center(
              child: CircularProgressIndicator(),
            ),
        ],
      ),
    );
  }
}

/// 打开 WebView 页面的工具函数
Future<void> openWebView(
  BuildContext context, {
  required String url,
  String? title,
  void Function(String url, String title)? onPageFinished,
}) {
  return Navigator.push(
    context,
    MaterialPageRoute(
      builder: (context) => AppWebView(
        url: url,
        title: title,
        onPageFinished: onPageFinished,
      ),
    ),
  );
}
