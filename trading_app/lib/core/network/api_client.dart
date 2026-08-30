/// Network API client with Dio and request logging interceptor.
library;

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

import '../../features/auth/data/session_controller.dart';

const _requestSessionSnapshotKey = 'authSessionSnapshot';

/// Base API URL, override via `--dart-define=API_BASE_URL=...`.
/// Dev server example:
/// `flutter run -d chrome --dart-define=API_BASE_URL=http://localhost:8080`
const _configuredApiBaseUrl = String.fromEnvironment('API_BASE_URL');

/// 非 web 端（Android/桌面）无页面 origin 可用，需要显式后端地址。
const _defaultApiBaseUrl = 'http://118.178.19.165:8080';

String get kApiBaseUrl {
  if (_configuredApiBaseUrl.isNotEmpty) {
    return _configuredApiBaseUrl;
  }
  // 网页由后端同端口托管，用当前页面 origin 即可，
  // 同一份产物在本机 localhost 与公网 IP 下都能正确访问 API。
  if (kIsWeb) {
    return Uri.base.origin;
  }
  return _defaultApiBaseUrl;
}

Dio? _apiClient;
SessionController? _apiSessionController;

/// Return the app-wide Dio instance.
Dio createApiClient({String? baseUrl, SessionController? sessionController}) {
  final resolvedBaseUrl = baseUrl ?? kApiBaseUrl;
  final existing = _apiClient;
  if (existing != null) {
    final keepsExistingController =
        sessionController == null ||
        identical(sessionController, _apiSessionController);
    if (existing.options.baseUrl == resolvedBaseUrl &&
        keepsExistingController) {
      return existing;
    }
    debugPrint(
      '[API BASE] rebuild ${existing.options.baseUrl} -> $resolvedBaseUrl',
    );
    existing.close(force: true);
  }

  _apiSessionController = sessionController;
  _apiClient = _buildApiClient(resolvedBaseUrl, sessionController);
  return _apiClient!;
}

/// Reset the singleton in tests or when rebuilding app dependencies.
@visibleForTesting
void resetApiClientForTesting() {
  _apiClient?.close(force: true);
  _apiClient = null;
  _apiSessionController = null;
}

Dio _buildApiClient(String baseUrl, SessionController? sessionController) {
  debugPrint('[API BASE] $baseUrl');
  final dio = Dio(
    BaseOptions(
      baseUrl: baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 30),
    ),
  );

  if (sessionController != null) {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          if (options.extra['skipAuth'] != true) {
            final snapshot = await sessionController.captureToken();
            options.extra[_requestSessionSnapshotKey] = snapshot;
            if (snapshot.token != null && snapshot.token!.isNotEmpty) {
              options.headers['Authorization'] = 'Bearer ${snapshot.token}';
            }
          }
          handler.next(options);
        },
        onError: (error, handler) async {
          final isAuthRequest = error.requestOptions.extra['skipAuth'] == true;
          if (!isAuthRequest && error.response?.statusCode == 401) {
            final reason = _invalidationReason(error.response?.data);
            final snapshot =
                error.requestOptions.extra[_requestSessionSnapshotKey];
            try {
              if (snapshot is SessionTokenSnapshot) {
                await sessionController.clearIfCurrent(
                  snapshot,
                  reason: reason,
                );
              }
            } finally {
              handler.next(error);
            }
            return;
          }
          handler.next(error);
        },
      ),
    );
  }

  dio.interceptors.add(
    LogInterceptor(
      request: false,
      requestHeader: false,
      responseHeader: false,
      error: true,
      logPrint: (obj) {
        // Only print errors by default; use ApiPathLogger for path logging
      },
    ),
  );

  dio.interceptors.add(ApiPathLogger());

  return dio;
}

SessionInvalidationReason _invalidationReason(dynamic data) {
  final code = data is Map ? data['code'] : null;
  return switch (code) {
    'SESSION_REPLACED' => SessionInvalidationReason.replaced,
    'SESSION_EXPIRED' => SessionInvalidationReason.expired,
    _ => SessionInvalidationReason.revoked,
  };
}

/// Interceptor that logs every API request path.
class ApiPathLogger extends Interceptor {
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    // debugPrint('[API] ${options.method} ${options.uri}');
    handler.next(options);
  }

  @override
  void onResponse(Response response, ResponseInterceptorHandler handler) {
    // debugPrint(
    //   '[API] ${response.requestOptions.method} ${response.requestOptions.uri} -> ${response.statusCode}',
    // );
    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    debugPrint(
      '[API ERROR] ${err.requestOptions.method} ${err.requestOptions.uri} -> ${err.message}',
    );
    handler.next(err);
  }
}
