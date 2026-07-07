import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter_tts/flutter_tts.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';

/// 语音播报队列项
class VoiceQueueItem {
  const VoiceQueueItem({
    required this.id,
    required this.text,
    required this.enqueuedAt,
    this.change,
  });

  final String id;
  final String text;
  final DateTime enqueuedAt;
  final StockChange? change;
}

/// 语音播报 ViewModel：管理播报开关、队列、当前播报
class VoiceAnnouncementViewModel extends ChangeNotifier {
  VoiceAnnouncementViewModel() {
    loadSettings();
  }

  final FlutterTts _flutterTts = FlutterTts();

  static const _enabledKey = 'voice_announcement_enabled';
  static const _askedKey = 'voice_announcement_asked';

  /// 是否启用语音播报（默认关闭，等用户授权）
  bool _enabled = false;
  bool get enabled => _enabled;

  /// 是否已经询问过用户授权
  bool _askedBefore = false;
  bool get askedBefore => _askedBefore;

  /// 是否已初始化完成
  bool _initialized = false;
  bool get initialized => _initialized;

  /// 初始化错误信息
  String? _initError;
  String? get initError => _initError;

  /// 播报队列
  final List<VoiceQueueItem> _queue = [];
  List<VoiceQueueItem> get queue => List.unmodifiable(_queue);

  /// 当前正在播报的文本
  String? _currentSpeakingText;
  String? get currentSpeakingText => _currentSpeakingText;

  /// 是否正在播报中
  bool get isSpeaking => _currentSpeakingText != null;

  /// 队列去重集合（避免同一条异动反复入队）
  final Set<int> _announcedChangeIds = {};

  /// 加载持久化开关并初始化 TTS
  Future<void> loadSettings() async {
    await _loadSettings();
    await _initTts();
  }

  Future<void> _loadSettings() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      _enabled = prefs.getBool(_enabledKey) ?? false;
      _askedBefore = prefs.getBool(_askedKey) ?? false;
    } catch (e) {
      debugPrint('[VoiceTTS] load settings failed: $e');
      _enabled = false;
      _askedBefore = false;
    }
  }

  Future<void> _saveEnabled() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool(_enabledKey, _enabled);
    } catch (e) {
      debugPrint('[VoiceTTS] save enabled failed: $e');
    }
  }

  Future<void> _saveAsked() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool(_askedKey, true);
    } catch (e) {
      debugPrint('[VoiceTTS] save asked failed: $e');
    }
  }

  /// 初始化 TTS
  Future<void> _initTts() async {
    try {
      await _flutterTts.setLanguage('zh-CN');
      await _flutterTts.setSpeechRate(0.55);
      await _flutterTts.setVolume(1.0);
      await _flutterTts.setPitch(1.0);

      _flutterTts.setCompletionHandler(() {
        _onSpeakComplete();
      });
      _flutterTts.setCancelHandler(() {
        _onSpeakComplete();
      });
      _flutterTts.setErrorHandler((msg) {
        debugPrint('[VoiceTTS] error: $msg');
        _onSpeakComplete();
      });

      _initialized = true;
    } catch (e) {
      _initError = e.toString();
      debugPrint('[VoiceTTS] init failed: $e');
    } finally {
      notifyListeners();
    }
  }

  /// 尝试重新初始化（授权后调用）
  Future<void> retryInit() async {
    if (_initialized) return;
    _initError = null;
    notifyListeners();
    await _initTts();
  }

  /// 用户同意授权并开启播报
  Future<void> grantPermission() async {
    _askedBefore = true;
    _enabled = true;
    await _saveAsked();
    await _saveEnabled();
    if (!_initialized) {
      await retryInit();
    }
    notifyListeners();
  }

  /// 用户拒绝授权，仅标记已询问
  Future<void> denyPermission() async {
    _askedBefore = true;
    _enabled = false;
    await _saveAsked();
    await _saveEnabled();
    notifyListeners();
  }

  /// 设置播报开关
  Future<void> setEnabled(bool value) async {
    if (_enabled == value) return;
    _enabled = value;
    await _saveEnabled();
    if (!_enabled) {
      await stopSpeaking();
    }
    notifyListeners();
  }

  /// 将异动加入播报队列
  void enqueueChange(StockChange change, {bool urgent = false}) {
    if (!_enabled || !_initialized) return;

    // 去重：同 ID 只播报一次
    if (!_announcedChangeIds.add(change.id)) return;

    final text = _buildSpeechText(change);
    final item = VoiceQueueItem(
      id: '${change.id}_${change.stockCode}',
      text: text,
      enqueuedAt: DateTime.now(),
      change: change,
    );

    if (urgent && _queue.isNotEmpty) {
      // 紧急插队：插到当前正在播的之后
      _queue.insert(1, item);
    } else {
      _queue.add(item);
    }

    notifyListeners();
    _processQueue();
  }

  /// 批量加入播报队列
  void enqueueChanges(List<StockChange> changes, {bool urgent = false}) {
    for (final change in changes) {
      enqueueChange(change, urgent: urgent);
    }
  }

  /// 构建播报文本：股票名 + 简化后的描述
  String _buildSpeechText(StockChange change) {
    final name = change.stockName.isNotEmpty ? change.stockName : change.stockCode;
    final desc = _simplifyDescription(change.description ?? _buildDefaultDesc(change));
    return '$name，$desc';
  }

  /// 服务端异动没有 description 时的默认描述
  String _buildDefaultDesc(StockChange change) {
    final rate = change.changeRate;
    final rateStr = rate >= 0 ? '涨${rate.toStringAsFixed(2)}%' : '跌${rate.abs().toStringAsFixed(2)}%';
    return '${change.typeName} $rateStr';
  }

  /// 简化描述，让 TTS 读得更自然
  String _simplifyDescription(String raw) {
    return raw
        .replaceAll('%', '百分之')
        .replaceAll('+', '涨')
        .replaceAll('急涨', '急速上涨')
        .replaceAll('急跌', '急速下跌')
        .replaceAll('爆量', '成交量放大')
        .replaceAll('缩量', '成交量缩小')
        .replaceAllMapped(
          RegExp(r'(\d+\.?\d*)倍'),
          (m) => '${m.group(1)}倍',
        )
        .replaceAllMapped(
          RegExp(r'(\d+\.?\d*)亿'),
          (m) => '${m.group(1)}亿',
        )
        .replaceAllMapped(
          RegExp(r'(\d+\.?\d*)万'),
          (m) => '${m.group(1)}万',
        )
        .replaceAll('[', '')
        .replaceAll(']', '');
  }

  /// 消费队列
  Future<void> _processQueue() async {
    if (_currentSpeakingText != null || _queue.isEmpty || !_enabled) return;

    final item = _queue.removeAt(0);
    _currentSpeakingText = item.text;
    notifyListeners();

    try {
      if (kIsWeb) {
        // Web 端：flutter_tts.speak 不保证等待完成，用延时兜底
        await _flutterTts.speak(item.text);
        await Future.delayed(const Duration(seconds: 4));
      } else {
        await _flutterTts.speak(item.text);
      }
    } catch (e) {
      debugPrint('[VoiceTTS] speak error: $e');
      _currentSpeakingText = null;
      notifyListeners();
      _processQueue();
    }
  }

  /// 一条播报完成
  void _onSpeakComplete() {
    if (_currentSpeakingText == null) return;
    _currentSpeakingText = null;
    notifyListeners();
    _processQueue();
  }

  /// 停止当前播报，并清空队列
  Future<void> stopSpeaking() async {
    try {
      if (Platform.isIOS || Platform.isAndroid || Platform.isMacOS) {
        await _flutterTts.stop();
      }
    } catch (e) {
      debugPrint('[VoiceTTS] stop error: $e');
    }
    _queue.clear();
    _currentSpeakingText = null;
    notifyListeners();
  }

  /// 仅清空队列（不停止当前正在播的）
  void clearQueue() {
    _queue.clear();
    notifyListeners();
  }

  /// 清空已播报记录（用于重置去重）
  void clearAnnouncedHistory() {
    _announcedChangeIds.clear();
    notifyListeners();
  }

  @override
  void dispose() {
    unawaited(_flutterTts.stop());
    super.dispose();
  }
}
