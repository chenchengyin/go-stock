import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter_tts/flutter_tts.dart';
import 'package:flutter_vibrate/flutter_vibrate.dart';
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
  static const _vibrateEnabledKey = 'voice_announcement_vibrate_enabled';
  static const _askedKey = 'voice_announcement_asked';
  static const _announcedIdsKey = 'voice_announced_ids';
  static const _announcedDateKey = 'voice_announced_date';
  static const _speechRateKey = 'voice_announcement_speech_rate';

  /// 是否启用语音播报（默认关闭，等用户授权）
  bool _enabled = false;
  bool get enabled => _enabled;

  /// 是否启用新异动震动提醒（默认开启）
  bool _vibrateEnabled = true;
  bool get vibrateEnabled => _vibrateEnabled;

  /// 是否已经询问过用户授权
  bool _askedBefore = false;
  bool get askedBefore => _askedBefore;

  /// 是否已初始化完成
  bool _initialized = false;
  bool get initialized => _initialized;

  /// 当前平台语速配置
  static final _SpeechRateConfig _speechRateConfig = _resolveSpeechRateConfig();

  /// 语速最小值
  double get minSpeechRate => _speechRateConfig.min;

  /// 语速最大值
  double get maxSpeechRate => _speechRateConfig.max;

  /// 默认语速（当前平台正常语速）
  double get defaultSpeechRate => _speechRateConfig.defaultValue;

  /// 播报语速
  double _speechRate = 0.55;
  double get speechRate => _speechRate;

  /// 初始化错误信息
  String? _initError;
  String? get initError => _initError;

  /// 播报队列
  final List<VoiceQueueItem> _queue = [];
  List<VoiceQueueItem> get queue => List.unmodifiable(_queue);

  /// 已播报队列（最新在前）
  final List<VoiceQueueItem> _spokenQueue = [];
  List<VoiceQueueItem> get spokenQueue => List.unmodifiable(_spokenQueue);

  /// 当前正在播报的文本
  String? _currentSpeakingText;
  String? get currentSpeakingText => _currentSpeakingText;

  /// 当前正在播报的队列项
  VoiceQueueItem? _currentItem;

  /// 是否正在播报中
  bool get isSpeaking => _currentSpeakingText != null;

  /// 队列去重集合（避免同一条异动反复入队）
  final Set<int> _announcedChangeIds = {};

  /// 加载持久化开关、语速、已播报记录并初始化 TTS
  Future<void> loadSettings() async {
    await _loadSettings();
    await _loadVibrateEnabled();
    await _loadSpeechRate();
    await _loadAnnouncedIds();
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

  /// 加载持久化震动开关，未设置过则默认开启
  Future<void> _loadVibrateEnabled() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      _vibrateEnabled = prefs.getBool(_vibrateEnabledKey) ?? true;
    } catch (e) {
      debugPrint('[VoiceTTS] load vibrate enabled failed: $e');
      _vibrateEnabled = true;
    }
  }

  /// 设置震动开关并持久化
  Future<void> setVibrateEnabled(bool value) async {
    if (_vibrateEnabled == value) return;
    _vibrateEnabled = value;
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool(_vibrateEnabledKey, _vibrateEnabled);
    } catch (e) {
      debugPrint('[VoiceTTS] save vibrate enabled failed: $e');
    }
    notifyListeners();
  }

  /// 新异动震动提醒（0.8 秒）
  Future<void> _triggerNewChangeVibrate() async {
    if (!_vibrateEnabled) return;
    try {
      final canVibrate = await Vibrate.canVibrate;
      if (!canVibrate) return;
      Vibrate.vibrateWithPauses(const [Duration(milliseconds: 800)]);
    } catch (e) {
      debugPrint('[VoiceTTS] vibrate failed: $e');
    }
  }

  /// 加载持久化语速，未设置过则使用当前平台默认值
  Future<void> _loadSpeechRate() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      _speechRate = prefs.getDouble(_speechRateKey) ?? _speechRateConfig.defaultValue;
    } catch (e) {
      debugPrint('[VoiceTTS] load speech rate failed: $e');
      _speechRate = _speechRateConfig.defaultValue;
    }
  }

  /// 设置语速并持久化
  Future<void> setSpeechRate(double value) async {
    final clamped = value.clamp(_speechRateConfig.min, _speechRateConfig.max);
    if (_speechRate == clamped) return;
    _speechRate = clamped;
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setDouble(_speechRateKey, _speechRate);
      await _flutterTts.setSpeechRate(_speechRate);
    } catch (e) {
      debugPrint('[VoiceTTS] set speech rate failed: $e');
    }
    notifyListeners();
  }

  /// 加载已播报记录，跨天自动清空
  Future<void> _loadAnnouncedIds() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final savedDate = prefs.getString(_announcedDateKey) ?? '';
      final today = _todayString();
      if (savedDate != today) {
        await prefs.remove(_announcedIdsKey);
        await prefs.setString(_announcedDateKey, today);
        _announcedChangeIds.clear();
      } else {
        final raw = prefs.getStringList(_announcedIdsKey);
        _announcedChangeIds.addAll(raw?.map(int.parse) ?? []);
      }
    } catch (e) {
      debugPrint('[VoiceTTS] load announced ids failed: $e');
      _announcedChangeIds.clear();
    }
  }

  /// 保存已播报记录
  Future<void> _saveAnnouncedIds() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setStringList(
        _announcedIdsKey,
        _announcedChangeIds.map((id) => id.toString()).toList(),
      );
      await prefs.setString(_announcedDateKey, _todayString());
    } catch (e) {
      debugPrint('[VoiceTTS] save announced ids failed: $e');
    }
  }

  String _todayString() {
    final now = DateTime.now();
    return '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
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
      await _flutterTts.setSpeechRate(_speechRate);
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
    // 去重：同 ID 当天只处理一次
    if (!_announcedChangeIds.add(change.id)) return;
    unawaited(_saveAnnouncedIds());

    // 新异动震动提醒（与语音开关独立，默认开启）
    unawaited(_triggerNewChangeVibrate());

    // 语音未启用或未初始化，不入队播报
    if (!_enabled || !_initialized) return;

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

  /// 测试：播放几条模拟异动（程序启动后调一次即可）
  void playTestChanges() {
    if (!_initialized) return;

    // 清掉已播报记录和已播放队列，确保测试每次都重新播放
    _announcedChangeIds.clear();
    _spokenQueue.clear();
    unawaited(_saveAnnouncedIds());
    notifyListeners();

    _enqueueTestChanges();
  }

  /// 重新启动测试：停止当前播报、清空队列，再重新播放测试数据
  Future<void> restartTest() async {
    if (!_initialized) return;
    await stopSpeaking();
    playTestChanges();
  }

  void _enqueueTestChanges() {

    final now = DateTime.now();
    final dateStr = '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
    final timeStr = '${now.hour.toString().padLeft(2, '0')}:${now.minute.toString().padLeft(2, '0')}:${now.second.toString().padLeft(2, '0')}';

    final testChanges = [
      StockChange(
        id: -999901,
        changeTime: timeStr,
        changeDate: dateStr,
        stockCode: 'sh688082',
        stockName: '盛美上海',
        changeType: 9001,
        typeName: '30秒急速波动',
        price: 120.50,
        changeRate: 2.35,
        volume: 12500,
        amount: 15060000,
        description: '30秒急速波动 +2.35%，成交额1506.00万',
      ),
      StockChange(
        id: -999902,
        changeTime: timeStr,
        changeDate: dateStr,
        stockCode: 'sz300820',
        stockName: '英杰电气',
        changeType: 9003,
        typeName: '30秒爆量',
        price: 45.20,
        changeRate: 0.88,
        volume: 86000,
        amount: 38870000,
        description: '30秒爆量 4.2倍，成交额3887.00万',
      ),
      StockChange(
        id: -999903,
        changeTime: timeStr,
        changeDate: dateStr,
        stockCode: 'sh603933',
        stockName: '睿能科技',
        changeType: 9005,
        typeName: '60秒急速波动',
        price: 18.60,
        changeRate: -1.20,
        volume: 32000,
        amount: 5952000,
        description: '60秒急速下跌 1.20%，成交额595.20万',
      ),
    ];

    debugPrint('[VoiceTTS] playTestChanges: ${testChanges.length} items');
    for (final change in testChanges) {
      // 测试数据也走正常入队逻辑，但不污染已播报去重（使用特殊负 ID）
      enqueueChange(change, urgent: false);
    }
  }

  /// 构建播报文本：股票名 + 简化后的描述 + 当前最新涨幅
  String _buildSpeechText(StockChange change) {
    final name = change.stockName.isNotEmpty ? change.stockName : change.stockCode;
    final desc = _simplifyDescription(change.description ?? _buildDefaultDesc(change));
    final rate = change.currentChangeRate != 0.0 ? change.currentChangeRate : change.changeRate;
    final rateStr = rate >= 0 ? '涨${rate.toStringAsFixed(2)}%' : '跌${rate.abs().toStringAsFixed(2)}%';
    return '$name，$desc，当前$rateStr';
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
        .replaceAll(RegExp(r'\[[^\]]*\]'), ' ')
        .replaceAll('%', '')
        .replaceAll('+', '涨')
        .replaceAll('急涨', '急速上涨')
        .replaceAll('急跌', '急速下跌')
        .replaceAll('爆量', '成交爆量')
        .replaceAll('缩量', '成交缩量')
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
        .replaceAllMapped(
          RegExp(r'\d+秒急速波动\s+急速(上涨|下跌)'),
          (m) => '急速${m.group(1)}',
        )
        .trim()
        .replaceAll(RegExp(r'\s+'), ' ');
  }

  /// 消费队列
  Future<void> _processQueue() async {
    if (_currentSpeakingText != null || _queue.isEmpty || !_enabled) return;

    final item = _queue.removeAt(0);
    _currentItem = item;
    _currentSpeakingText = item.text;
    notifyListeners();

    try {
      // 每次播报前重新设置语速，避免引擎状态被重置导致语速失效
      await _flutterTts.setSpeechRate(_speechRate);
      if (kIsWeb) {
        // Web 端：flutter_tts.speak 不保证等待完成，用延时兜底
        await _flutterTts.speak(item.text);
        await Future.delayed(const Duration(seconds: 4));
      } else {
        await _flutterTts.speak(item.text);
      }
    } catch (e) {
      debugPrint('[VoiceTTS] speak error: $e');
      _currentItem = null;
      _currentSpeakingText = null;
      notifyListeners();
      _processQueue();
    }
  }

  /// 一条播报完成
  void _onSpeakComplete() {
    if (_currentSpeakingText == null) return;
    final completedItem = _currentItem;
    _currentItem = null;
    _currentSpeakingText = null;
    if (completedItem != null) {
      _spokenQueue.insert(0, completedItem);
    }
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
    _currentItem = null;
    _currentSpeakingText = null;
    notifyListeners();
  }

  /// 仅清空队列（不停止当前正在播的）
  void clearQueue() {
    _queue.clear();
    notifyListeners();
  }

  /// 清空已播放队列
  void clearSpokenQueue() {
    _spokenQueue.clear();
    notifyListeners();
  }

  /// 清空已播报记录（用于重置去重）
  Future<void> clearAnnouncedHistory() async {
    _announcedChangeIds.clear();
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(_announcedIdsKey);
      await prefs.setString(_announcedDateKey, _todayString());
    } catch (e) {
      debugPrint('[VoiceTTS] clear announced history failed: $e');
    }
    notifyListeners();
  }

  @override
  void dispose() {
    unawaited(_flutterTts.stop());
    super.dispose();
  }

  /// 根据当前平台解析合适的语速范围与默认值
  static _SpeechRateConfig _resolveSpeechRateConfig() {
    if (kIsWeb) {
      return const _SpeechRateConfig(min: 0.3, max: 1.5, defaultValue: 0.55);
    }
    if (Platform.isIOS || Platform.isMacOS) {
      // iOS/macOS AVSpeechSynthesizer：0.5 为正常语速，有效范围 0.0~1.0
      return const _SpeechRateConfig(min: 0.1, max: 1.0, defaultValue: 0.5);
    }
    if (Platform.isAndroid) {
      // Android TextToSpeech：1.0 为正常语速，支持更广范围
      return const _SpeechRateConfig(min: 0.25, max: 2.0, defaultValue: 1.0);
    }
    return const _SpeechRateConfig(min: 0.3, max: 1.5, defaultValue: 0.55);
  }
}

/// 语速配置
class _SpeechRateConfig {
  const _SpeechRateConfig({
    required this.min,
    required this.max,
    required this.defaultValue,
  });

  final double min;
  final double max;
  final double defaultValue;
}
