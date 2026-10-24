import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/domain/red_auction_reminder.dart';

Map<String, dynamic> confirmed(
  String date, {
  List<dynamic> results = const [],
}) => {
  'date': date,
  'confirmed': true,
  'count': results.length,
  'results': results,
};

void main() {
  testWidgets(
    'resuming the browser does not bypass the five-second retry interval',
    (tester) async {
      final fakeStart = tester.binding.clock.now();
      final start = DateTime.parse('2026-10-23T01:25:10Z');
      var requests = 0;
      final reminder = RedAuctionReminder(
        now: () => start.add(tester.binding.clock.now().difference(fakeStart)),
        request: (date) async {
          requests++;
          return {'date': date, 'status': 'warming'};
        },
        onConfirmed: (_) {},
        onTimeout: () {},
      );
      reminder.setEnabled(true);
      await tester.pump(Duration.zero);
      expect(requests, 1);
      await tester.pump(const Duration(seconds: 1));
      reminder.resume();
      await tester.pump(Duration.zero);
      expect(requests, 1);
      await tester.pump(const Duration(seconds: 4));
      expect(requests, 2);
      reminder.dispose();
    },
  );

  testWidgets('requests at 09:25:10 then retries five seconds after response', (
    tester,
  ) async {
    var now = DateTime.parse('2026-10-23T01:25:09Z');
    final requested = <String>[];
    final received = <Map<String, dynamic>>[];
    final reminder = RedAuctionReminder(
      now: () => now,
      request: (date) async {
        requested.add(date);
        return requested.length == 1
            ? {'date': date, 'status': 'warming', 'confirmed': false}
            : confirmed(date);
      },
      onConfirmed: received.add,
      onTimeout: () => fail('should not time out'),
    );
    reminder.setEnabled(true);
    await tester.pump(Duration.zero);
    expect(requested, isEmpty);
    now = now.add(const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
    expect(requested, ['2026-10-23']);
    now = now.add(const Duration(seconds: 4));
    await tester.pump(const Duration(seconds: 4));
    expect(requested, hasLength(1));
    now = now.add(const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
    expect(received, [confirmed('2026-10-23')]);
    now = now.add(const Duration(seconds: 30));
    await tester.pump(const Duration(seconds: 30));
    expect(requested, hasLength(2));
    reminder.dispose();
  });

  testWidgets(
    'does not overlap requests and ignores a revoked in-flight response',
    (tester) async {
      var now = DateTime.parse('2026-10-23T01:25:10Z');
      final pending = Completer<Map<String, dynamic>>();
      var requests = 0;
      final received = <Map<String, dynamic>>[];
      final reminder = RedAuctionReminder(
        now: () => now,
        request: (_) {
          requests++;
          return pending.future;
        },
        onConfirmed: received.add,
        onTimeout: () {},
      );
      reminder.setEnabled(true);
      await tester.pump(Duration.zero);
      now = now.add(const Duration(seconds: 15));
      reminder.resume();
      await tester.pump(const Duration(seconds: 15));
      expect(requests, 1);
      reminder.setEnabled(false);
      pending.complete(confirmed('2026-10-23'));
      await tester.pump(Duration.zero);
      expect(received, isEmpty);
      now = now.add(const Duration(seconds: 10));
      await tester.pump(const Duration(seconds: 10));
      expect(requests, 1);
      reminder.dispose();
    },
  );

  testWidgets(
    'retries errors and invalid confirmations but stops new requests at 09:26',
    (tester) async {
      var now = DateTime.parse('2026-10-23T01:25:50Z');
      var requests = 0;
      var timeouts = 0;
      final reminder = RedAuctionReminder(
        now: () => now,
        request: (_) async {
          requests++;
          if (requests == 1) throw Exception('network');
          return confirmed('2026-10-22');
        },
        onConfirmed: (_) => fail('stale results must not notify'),
        onTimeout: () => timeouts++,
      );
      reminder.setEnabled(true);
      await tester.pump(Duration.zero);
      now = now.add(const Duration(seconds: 5));
      await tester.pump(const Duration(seconds: 5));
      expect(requests, 2);
      now = now.add(const Duration(seconds: 5));
      await tester.pump(const Duration(seconds: 5));
      expect(requests, 2);
      expect(timeouts, 1);
      reminder.resume();
      await tester.pump(Duration.zero);
      expect(timeouts, 1);
      reminder.dispose();
    },
  );

  testWidgets('skips weekends and resumes on the next weekday', (tester) async {
    var now = DateTime.parse('2026-10-23T01:25:10Z');
    final dates = <String>[];
    final received = <Map<String, dynamic>>[];
    final reminder = RedAuctionReminder(
      now: () => now,
      request: (date) async {
        dates.add(date);
        return confirmed(date);
      },
      onConfirmed: received.add,
      onTimeout: () {},
    );
    reminder.setEnabled(true);
    await tester.pump(Duration.zero);
    expect(received, hasLength(1));
    now = DateTime.parse('2026-10-24T01:25:10Z');
    reminder.resume();
    await tester.pump(const Duration(days: 1));
    expect(dates, ['2026-10-23']);
    now = DateTime.parse('2026-10-26T01:25:10Z');
    reminder.resume();
    await tester.pump(Duration.zero);
    expect(dates, ['2026-10-23', '2026-10-26']);
    reminder.dispose();
  });

  testWidgets(
    'incomplete data cannot notify and no_data ends the day silently',
    (tester) async {
      var now = DateTime.parse('2026-10-23T01:25:10Z');
      var requests = 0;
      final reminder = RedAuctionReminder(
        now: () => now,
        request: (date) async {
          requests++;
          return requests == 1
              ? {...confirmed(date), 'data_incomplete': true}
              : {'date': date, 'no_data': true, 'results': []};
        },
        onConfirmed: (_) =>
            fail('incomplete or non-trading data cannot notify'),
        onTimeout: () => fail('no_data must stop silently'),
      );
      reminder.setEnabled(true);
      await tester.pump(Duration.zero);
      now = now.add(const Duration(seconds: 5));
      await tester.pump(const Duration(seconds: 5));
      now = now.add(const Duration(seconds: 40));
      await tester.pump(const Duration(seconds: 40));
      expect(requests, 2);
      reminder.dispose();
    },
  );
}
