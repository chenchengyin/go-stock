import 'dart:async';

/// Requests the server's auction confirmation while the browser page is alive.
/// Server-side prewarming and selection remain independent of this reminder.
class RedAuctionReminder {
  RedAuctionReminder({
    required Future<Map<String, dynamic>> Function(String date) request,
    required void Function(Map<String, dynamic>) onConfirmed,
    required void Function() onTimeout,
    DateTime Function()? now,
  }) : _request = request,
       _onConfirmed = onConfirmed,
       _onTimeout = onTimeout,
       _now = now ?? DateTime.now;

  final Future<Map<String, dynamic>> Function(String date) _request;
  final void Function(Map<String, dynamic>) _onConfirmed;
  final void Function() _onTimeout;
  final DateTime Function() _now;
  Timer? _timer;
  bool _enabled = false;
  bool _disposed = false;
  bool _requesting = false;
  int _generation = 0;
  String? _attemptedDate;
  String? _completedDate;
  String? _timedOutDate;
  DateTime? _nextAttemptAt;

  DateTime _shanghaiNow() => _now().toUtc().add(const Duration(hours: 8));

  static String _date(DateTime time) =>
      '${time.year}-${time.month.toString().padLeft(2, '0')}-${time.day.toString().padLeft(2, '0')}';

  static bool _weekday(DateTime time) => time.weekday <= DateTime.friday;

  static DateTime _start(DateTime time) =>
      DateTime.utc(time.year, time.month, time.day, 9, 25, 10);

  static DateTime _end(DateTime time) =>
      DateTime.utc(time.year, time.month, time.day, 9, 26);

  void setEnabled(bool enabled) {
    if (_disposed || _enabled == enabled) return;
    _enabled = enabled;
    _generation++;
    _schedule();
  }

  /// Recheck wall-clock time after browser throttling or computer sleep.
  void resume() => _schedule();

  void _schedule() {
    _timer?.cancel();
    _timer = null;
    if (_disposed || !_enabled || _requesting) return;
    final now = _shanghaiNow();
    final date = _date(now);
    if (_weekday(now) && _completedDate != date && _timedOutDate != date) {
      final start = _start(now);
      final end = _end(now);
      if (now.isBefore(start)) {
        _timer = Timer(start.difference(now), _attempt);
        return;
      }
      if (now.isBefore(end)) {
        final next = _nextAttemptAt;
        final target = next != null && next.isAfter(now)
            ? (next.isBefore(end) ? next : end)
            : now;
        _timer = Timer(target.difference(now), _attempt);
        return;
      }
      if (_attemptedDate == date) {
        _timedOutDate = date;
        _onTimeout();
        if (_disposed || !_enabled) return;
      }
    }
    var next = _start(now).add(const Duration(days: 1));
    while (!_weekday(next)) {
      next = next.add(const Duration(days: 1));
    }
    _timer = Timer(next.difference(now), _attempt);
  }

  Future<void> _attempt() async {
    if (_disposed || !_enabled || _requesting) return;
    final now = _shanghaiNow();
    final date = _date(now);
    if (!_weekday(now) ||
        now.isBefore(_start(now)) ||
        !now.isBefore(_end(now)) ||
        (_nextAttemptAt != null && now.isBefore(_nextAttemptAt!))) {
      _schedule();
      return;
    }
    final generation = _generation;
    _attemptedDate = date;
    _requesting = true;
    Map<String, dynamic>? response;
    try {
      response = await _request(date);
    } catch (_) {
      // A transient network error gets the same bounded retry as unready data.
    } finally {
      _requesting = false;
      _nextAttemptAt = _shanghaiNow().add(const Duration(seconds: 5));
    }
    if (_disposed || !_enabled) return;
    if (generation != _generation || _date(_shanghaiNow()) != date) {
      _schedule();
      return;
    }
    if (response?['date'] == date) {
      if (response?['no_data'] == true) {
        _completedDate = date;
      } else if (response?['confirmed'] == true &&
          response?['data_incomplete'] != true &&
          response?['results'] is List) {
        _completedDate = date;
        _onConfirmed(response!);
      }
    }
    _schedule();
  }

  void dispose() {
    _disposed = true;
    _generation++;
    _timer?.cancel();
  }
}
