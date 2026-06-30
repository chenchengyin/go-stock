import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'notification_util_web.dart'
    if (dart.library.io) 'notification_util_stub.dart';

FlutterLocalNotificationsPlugin? _notificationsPlugin;
final Map<String, int> _summaryNotificationIds = {};

const _stockChangeChannel = AndroidNotificationChannel(
  'stock_change_channel',
  '异动监控通知',
  description: '股票异动提醒通知',
  importance: Importance.high,
);

typedef NotificationTapCallback = void Function(Map<String, dynamic>);

NotificationTapCallback? _onNotificationTap;

Future<void> initNotifications({
  NotificationTapCallback? onTap,
}) async {
  _onNotificationTap = onTap;
  if (defaultTargetPlatform == TargetPlatform.android) {
    _notificationsPlugin = FlutterLocalNotificationsPlugin();
    const androidSettings = AndroidInitializationSettings(
      '@mipmap/ic_launcher',
    );
    const initSettings = InitializationSettings(android: androidSettings);
    await _notificationsPlugin!.initialize(
      initSettings,
      onDidReceiveNotificationResponse: _onNotificationResponse,
      onDidReceiveBackgroundNotificationResponse: _onNotificationResponse,
    );
    await _ensureAndroidNotificationPermission();
    await _notificationsPlugin!
        .resolvePlatformSpecificImplementation<
          AndroidFlutterLocalNotificationsPlugin
        >()
        ?.createNotificationChannel(_stockChangeChannel);
  }
}

void _onNotificationResponse(NotificationResponse response) {
  if (response.payload != null && _onNotificationTap != null) {
    try {
      final data = jsonDecode(response.payload!) as Map<String, dynamic>;
      _onNotificationTap!(data);
    } catch (_) {}
  }
}

Future<void> showDesktopNotification(String title, String body) async {
  await showStockChangeNotification(title, body);
}

Future<void> showStockChangeNotification(String title, String body) async {
  if (defaultTargetPlatform == TargetPlatform.android) {
    await _showAndroidNotification(title, body);
  } else if (kIsWeb) {
    showWebNotification(title, body);
  }
}

Future<void> showStockChangeNotificationWithGroup({
  required StockChange change,
}) async {
  if (defaultTargetPlatform != TargetPlatform.android) {
    return;
  }
  await _ensureAndroidNotificationPermission();
  if (_notificationsPlugin == null) {
    await initNotifications();
  }

  final amountStr = change.amount > 0
      ? change.amount >= 100000000
          ? '${(change.amount / 100000000).toStringAsFixed(2)}亿'
          : change.amount >= 10000
              ? '${(change.amount / 10000).toStringAsFixed(2)}万'
              : change.amount.toStringAsFixed(0)
      : '';

  final title = '${change.stockName} ${change.typeName}';
  final body = [
    '${change.changeDate} ${change.changeTime}',
    '¥${change.price} ${change.changeRate > 0 ? '+' : ''}${change.changeRate}%',
    if (amountStr.isNotEmpty) '成交额: $amountStr',
  ].join('\n');

  final groupId = 'stock_${change.stockCode}';
  final payload = jsonEncode({
    'type': 'stock_change_detail',
    'stockCode': change.stockCode,
    'stockName': change.stockName,
  });

  final androidDetails = AndroidNotificationDetails(
    _stockChangeChannel.id,
    _stockChangeChannel.name,
    channelDescription: _stockChangeChannel.description,
    importance: Importance.high,
    priority: Priority.high,
    ticker: '股票异动提醒',
    groupKey: groupId,
    setAsGroupSummary: false,
    styleInformation: BigTextStyleInformation(body),
    autoCancel: true,
    largeIcon: const DrawableResourceAndroidBitmap('@mipmap/ic_launcher'),
    color: Colors.blue,
  );

  final details = NotificationDetails(android: androidDetails);
  await _notificationsPlugin!.show(
    change.id,
    title,
    body,
    details,
    payload: payload,
  );

  await _updateGroupSummary(change.stockCode, change.stockName);
}

Future<void> _updateGroupSummary(String stockCode, String stockName) async {
  if (_notificationsPlugin == null) return;

  final groupId = 'stock_$stockCode';
  final androidPlugin = _notificationsPlugin!
      .resolvePlatformSpecificImplementation<
          AndroidFlutterLocalNotificationsPlugin>();
  if (androidPlugin == null) return;

  final activeNotifications = await androidPlugin.getActiveNotifications();

  int count = 0;
  for (final n in activeNotifications) {
    if (n.groupKey == groupId) {
      count++;
    }
  }

  if (count == 0) {
    if (_summaryNotificationIds.containsKey(stockCode)) {
      await _notificationsPlugin!.cancel(_summaryNotificationIds[stockCode]!);
      _summaryNotificationIds.remove(stockCode);
    }
    return;
  }

  final summaryId = stockCode.hashCode.abs();
  _summaryNotificationIds[stockCode] = summaryId;

  final androidDetails = AndroidNotificationDetails(
    _stockChangeChannel.id,
    _stockChangeChannel.name,
    channelDescription: _stockChangeChannel.description,
    importance: Importance.high,
    priority: Priority.high,
    groupKey: groupId,
    setAsGroupSummary: true,
    autoCancel: true,
    largeIcon: const DrawableResourceAndroidBitmap('@mipmap/ic_launcher'),
    color: Colors.blue,
  );

  final details = NotificationDetails(android: androidDetails);
  await _notificationsPlugin!.show(
    summaryId,
    '$stockName（${count}条异动）',
    '',
    details,
    payload: jsonEncode({
      'type': 'stock_change_detail',
      'stockCode': stockCode,
      'stockName': stockName,
    }),
  );
}

Future<void> cancelStockNotifications(String stockCode) async {
  if (defaultTargetPlatform != TargetPlatform.android ||
      _notificationsPlugin == null) {
    return;
  }
  final groupId = 'stock_$stockCode';
  final androidPlugin = _notificationsPlugin!
      .resolvePlatformSpecificImplementation<
          AndroidFlutterLocalNotificationsPlugin>();
  if (androidPlugin == null) return;

  final activeNotifications = await androidPlugin.getActiveNotifications();

  for (final notification in activeNotifications) {
    if (notification.groupKey == groupId) {
      final id = notification.id;
      if (id != null) {
        await _notificationsPlugin!.cancel(id);
      }
    }
  }

  _summaryNotificationIds.remove(stockCode);
}

Future<void> _showAndroidNotification(String title, String body) async {
  if (_notificationsPlugin == null) {
    await initNotifications();
  }
  await _ensureAndroidNotificationPermission();

  final androidDetails = AndroidNotificationDetails(
    _stockChangeChannel.id,
    _stockChangeChannel.name,
    channelDescription: _stockChangeChannel.description,
    importance: Importance.high,
    priority: Priority.high,
    ticker: '股票异动提醒',
    styleInformation: BigTextStyleInformation(
      body,
      contentTitle: title,
      summaryText: '交易雷达',
    ),
  );
  final details = NotificationDetails(android: androidDetails);
  await _notificationsPlugin!.show(
    DateTime.now().millisecondsSinceEpoch ~/ 1000,
    title,
    body,
    details,
  );
}

Future<void> _ensureAndroidNotificationPermission() async {
  await _notificationsPlugin
      ?.resolvePlatformSpecificImplementation<
        AndroidFlutterLocalNotificationsPlugin
      >()
      ?.requestNotificationsPermission();
}
