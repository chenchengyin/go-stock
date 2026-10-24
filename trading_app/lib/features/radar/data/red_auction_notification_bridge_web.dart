import 'dart:js_interop';

@JS('redAuctionNotifications.permission')
external JSString _permission();

@JS('redAuctionNotifications.requestPermission')
external JSPromise<JSString> _requestPermission();

@JS('redAuctionNotifications.pairingAvailable')
external JSBoolean _pairingAvailable();

@JS('redAuctionNotifications.localReminderEnabled')
external JSBoolean _localReminderEnabled();

bool localReminderEnabled() {
  try {
    return _localReminderEnabled().toDart;
  } catch (_) {
    return false;
  }
}

@JS('redAuctionNotifications.pairLocalReminder')
external JSPromise<JSString> _pairLocalReminder(JSString accessToken);

bool pairingAvailable() {
  try {
    return _pairingAvailable().toDart;
  } catch (_) {
    return false;
  }
}

Future<String> pairLocalReminder(String accessToken) async {
  try {
    return (await _pairLocalReminder(accessToken.toJS).toDart).toDart;
  } catch (_) {
    return 'error:本机提醒连接失败';
  }
}

@JS('redAuctionNotifications.show')
external JSBoolean _show(
  JSString date,
  JSString userId,
  JSNumber count,
  JSString body,
  JSFunction onTap,
);

String permission() {
  try {
    return _permission().toDart;
  } catch (_) {
    return 'unsupported';
  }
}

Future<String> requestPermission() async {
  try {
    return (await _requestPermission().toDart).toDart;
  } catch (_) {
    return 'unsupported';
  }
}

bool show({
  required String date,
  required String userId,
  required int count,
  required String body,
  required void Function() onTap,
}) {
  try {
    return _show(
      date.toJS,
      userId.toJS,
      count.toJS,
      body.toJS,
      onTap.toJS,
    ).toDart;
  } catch (_) {
    return false;
  }
}
