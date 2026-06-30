import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart';
import 'package:trading_app/core/storage/storage_service.dart';

/// 获取东财 qgqp_b_id 的页面
class QgqpBidPage extends StatefulWidget {
  const QgqpBidPage({super.key});

  @override
  State<QgqpBidPage> createState() => _QgqpBidPageState();
}

class _QgqpBidPageState extends State<QgqpBidPage> {
  late InAppWebViewController _controller;
  bool _isLoading = true;
  bool _isExtracting = false;
  String? _extractedBid;
  String? _errorMessage;

  // 东财选股页面的 Cookie 名称
  static const String _targetCookieName = 'qgqp_b_id';

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('获取东财标识'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.pop(context),
        ),
      ),
      body: Column(
        children: [
          // 说明区域
          Container(
            padding: const EdgeInsets.all(16),
            color: Colors.blue.shade50,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  '说明',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
                ),
                const SizedBox(height: 8),
                const Text(
                  '请在下方页面中登录您的东方财富账号，然后等待页面加载完成。'
                  '系统将自动提取 Cookie 中的 qgqp_b_id 用于选股功能。',
                ),
                if (_extractedBid != null) ...[
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Colors.green.shade50,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      children: [
                        Icon(Icons.check_circle, color: Colors.green.shade700),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '已获取 qgqp_b_id',
                                style: TextStyle(color: Colors.green.shade700),
                              ),
                              Text(
                                _extractedBid!,
                                style: TextStyle(
                                  color: Colors.green.shade700,
                                  fontFamily: 'monospace',
                                  fontSize: 12,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
                if (_errorMessage != null) ...[
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Colors.red.shade50,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      children: [
                        Icon(Icons.error, color: Colors.red.shade700),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            _errorMessage!,
                            style: TextStyle(color: Colors.red.shade700),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ],
            ),
          ),

          // WebView 区域
          Expanded(
            child: Stack(
              children: [
                InAppWebView(
                  initialUrlRequest: URLRequest(
                    url: WebUri('https://xuangu.eastmoney.com/'),
                  ),
                  initialOptions: InAppWebViewGroupOptions(
                    crossPlatform: InAppWebViewOptions(
                      javaScriptEnabled: true,
                      cacheEnabled: false,
                      javaScriptCanOpenWindowsAutomatically: true,
                    ),
                    ios: IOSInAppWebViewOptions(
                      allowsInlineMediaPlayback: true,
                    ),
                  ),
                  onWebViewCreated: (controller) {
                    _controller = controller;
                  },
                  onLoadStart: (controller, url) {
                    setState(() {
                      _isLoading = true;
                      _errorMessage = null;
                    });
                  },
                  onLoadStop: (controller, url) async {
                    setState(() => _isLoading = false);

                    // 页面加载完成后，尝试提取 Cookie
                    await _extractCookie();
                  },
                  onConsoleMessage: (controller, consoleMessage) {
                    debugPrint(
                      'WebView Console: ${consoleMessage.message}',
                    );
                  },
                ),
                if (_isLoading || _isExtracting)
                  Container(
                    color: Colors.black26,
                    child: Center(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const CircularProgressIndicator(),
                          const SizedBox(height: 16),
                          Text(
                            _isExtracting
                                ? '正在提取 Cookie...'
                                : '页面加载中...',
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),

          // 底部操作区
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: SizedBox(
                width: double.infinity,
                child: CupertinoButton(
                  color: Colors.blue,
                  onPressed: _extractedBid != null ? _saveAndClose : null,
                  child: Text(
                    _extractedBid != null ? '保存并关闭' : '请先获取 qgqp_b_id',
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  /// 从 Cookie 中提取 qgqp_b_id
  Future<void> _extractCookie() async {
    if (_isExtracting) return;

    setState(() {
      _isExtracting = true;
      _errorMessage = null;
    });

    try {
      // 东财选股页面的域名
      final webUri = WebUri('https://xuangu.eastmoney.com/');

      // 获取该域名的所有 Cookie
      final cookieManager = CookieManager.instance();
      final cookies = await cookieManager.getCookies(url: webUri);

      String? bid;
      for (final cookie in cookies) {
        if (cookie.name == _targetCookieName) {
          bid = cookie.value;
          break;
        }
      }

      setState(() {
        _extractedBid = bid;
        _isExtracting = false;
        if (bid == null) {
          _errorMessage = '未找到 qgqp_b_id，请确保已登录东财账号并刷新页面';
        }
      });
    } catch (e) {
      setState(() {
        _isExtracting = false;
        _errorMessage = '提取 Cookie 失败: $e';
      });
    }
  }

  /// 保存并关闭
  Future<void> _saveAndClose() async {
    if (_extractedBid == null) return;

    // 保存到本地
    final storage = LocalStorageService();
    await storage.saveQgqpBId(_extractedBid!);

    if (mounted) {
      Navigator.pop(context, _extractedBid);
    }
  }
}

/// 打开获取 qgqp_b_id 页面的工具函数
Future<String?> openQgqpBidPage(BuildContext context) async {
  return Navigator.push<String>(
    context,
    MaterialPageRoute(
      builder: (context) => const QgqpBidPage(),
    ),
  );
}
