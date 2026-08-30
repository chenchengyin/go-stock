import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/strategy/data/strategy_repository.dart';
import 'package:trading_app/features/strategy/presentation/strategy_view_model.dart';

void main() {
  group('StrategyRepository server-owned identity', () {
    test('mutation bodies never contain client user identity', () async {
      final adapter = _StrategyTestAdapter((_) => _okResponse);
      final repository = StrategyRepository(
        dio: Dio()..httpClientAdapter = adapter,
      );

      await repository.viewPost(11);
      await repository.checkIn();
      await repository.createPost(
        title: 'Title',
        content: 'Content',
        images: const <String>['one.png'],
      );
      await repository.toggleLike(11);
      await repository.addComment(
        postId: 11,
        parentId: 7,
        content: 'Reply',
        images: const <String>['reply.png'],
        replyToUid: 'target-user',
        replyToName: 'Target',
      );
      await repository.deleteComment(7);
      await repository.deletePost(11);

      expect(
        adapter.requests.map((request) => request.data),
        <Map<String, dynamic>>[
          <String, dynamic>{'action': 'view_post', 'postId': 11},
          <String, dynamic>{'action': 'checkin'},
          <String, dynamic>{
            'action': 'create_post',
            'title': 'Title',
            'content': 'Content',
            'images': <String>['one.png'],
          },
          <String, dynamic>{'action': 'toggle_like', 'postId': 11},
          <String, dynamic>{
            'action': 'add_comment',
            'postId': 11,
            'parentId': 7,
            'content': 'Reply',
            'images': <String>['reply.png'],
            'replyToUid': 'target-user',
            'replyToName': 'Target',
          },
          <String, dynamic>{'action': 'delete_comment', 'commentId': 7},
          <String, dynamic>{'action': 'delete_post', 'postId': 11},
        ],
      );
      for (final request in adapter.requests) {
        expect(request.data, isNot(contains('userId')));
        expect(request.data, isNot(contains('nickname')));
      }
    });

    test('identity queries never contain a client user id', () async {
      final adapter = _StrategyTestAdapter((_) => _okResponse);
      final repository = StrategyRepository(
        dio: Dio()..httpClientAdapter = adapter,
      );

      await repository.getUserPoints();
      await repository.hasCheckedIn();
      await repository.getLikeStatus(11);
      await repository.getTodayReplyPoints();

      expect(
        adapter.requests.map((request) => request.queryParameters),
        <Map<String, dynamic>>[
          <String, dynamic>{'action': 'points'},
          <String, dynamic>{'action': 'checkin_status'},
          <String, dynamic>{'action': 'liked', 'postId': '11'},
          <String, dynamic>{'action': 'today_reply_points'},
        ],
      );
      for (final request in adapter.requests) {
        expect(request.queryParameters, isNot(contains('userId')));
      }
    });
  });

  test(
    'StrategyViewModel operations do not swallow SESSION_REPLACED',
    () async {
      final operations = <String, Future<void> Function(StrategyViewModel)>{
        'load': (viewModel) => viewModel.load(),
        'loadPoints': (viewModel) => viewModel.loadPoints(),
        'checkIn': (viewModel) async {
          await viewModel.checkIn();
        },
        'createPost': (viewModel) async {
          await viewModel.createPost(title: 'Title');
        },
        'toggleLike': (viewModel) async {
          await viewModel.toggleLike(11);
        },
        'addComment': (viewModel) async {
          await viewModel.addComment(postId: 11, content: 'Reply');
        },
        'deletePost': (viewModel) => viewModel.deletePost(11),
      };

      for (final entry in operations.entries) {
        final adapter = _StrategyTestAdapter(
          (_) => const _TestResponse.json(401, <String, dynamic>{
            'code': 'SESSION_REPLACED',
            'message': '账号已在其他设备登录，请重新登录',
          }),
        );
        final viewModel = StrategyViewModel(
          dio: Dio()..httpClientAdapter = adapter,
        );

        await expectLater(
          entry.value(viewModel),
          throwsA(
            isA<DioException>().having(
              (error) => (error.response?.data as Map<String, dynamic>)['code'],
              '${entry.key} code',
              'SESSION_REPLACED',
            ),
          ),
        );
        expect(adapter.requests, hasLength(1), reason: entry.key);
        viewModel.dispose();
      }
    },
  );
}

const _okResponse = _TestResponse.json(200, <String, dynamic>{});

class _TestResponse {
  const _TestResponse.json(this.statusCode, this.data);

  final int statusCode;
  final Object? data;
}

class _StrategyTestAdapter implements HttpClientAdapter {
  _StrategyTestAdapter(this._handler);

  final FutureOr<_TestResponse> Function(RequestOptions request) _handler;
  final List<RequestOptions> requests = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    final response = await _handler(options);
    return ResponseBody.fromString(
      jsonEncode(response.data),
      response.statusCode,
      headers: <String, List<String>>{
        Headers.contentTypeHeader: <String>[Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
