import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'notification_util_web.dart'
    if (dart.library.io) 'notification_util_stub.dart';

FlutterLocalNotificationsPlugin? _notificationsPlugin;
const _stockChangeChannel = AndroidNotificationChannel(
  'stock_change_channel',
  '异动监控通知',
  description: '股票异动提醒通知',
  importance: Importance.high,
);

Future<void> initNotifications() async {
  if (defaultTargetPlatform == TargetPlatform.android) {
    _notificationsPlugin = FlutterLocalNotificationsPlugin();
    const androidSettings = AndroidInitializationSettings(
      '@mipmap/ic_launcher',
    );
    const initSettings = InitializationSettings(android: androidSettings);
    await _notificationsPlugin!.initialize(initSettings);
    await _ensureAndroidNotificationPermission();
    await _notificationsPlugin!
        .resolvePlatformSpecificImplementation<
          AndroidFlutterLocalNotificationsPlugin
        >()
        ?.createNotificationChannel(_stockChangeChannel);
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
