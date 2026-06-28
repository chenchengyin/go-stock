/// Network API client with Dio and request logging interceptor.
library;

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

/// Base API URL, override via environment or config.
/// Web uses localhost, Android uses local network IP.
const _configuredApiBaseUrl = String.fromEnvironment('API_BASE_URL');

String get kApiBaseUrl {
  if (_configuredApiBaseUrl.isNotEmpty) {
    return _configuredApiBaseUrl;
  }
  if (kIsWeb) {
    return 'http://localhost:8080';
  }
  // Android device needs local network IP
  if (defaultTargetPlatform == TargetPlatform.android) {
    return 'http://192.168.0.104:8080';
  }
  return 'http://localhost:8080';
}

Dio? _apiClient;

/// Return the app-wide Dio instance.
Dio createApiClient({String? baseUrl}) {
  final resolvedBaseUrl = baseUrl ?? kApiBaseUrl;
  final existing = _apiClient;
  if (existing != null) {
    if (existing.options.baseUrl == resolvedBaseUrl) {
      return existing;
    }
    debugPrint(
      '[API BASE] rebuild ${existing.options.baseUrl} -> $resolvedBaseUrl',
    );
    existing.close(force: true);
  }

  _apiClient = _buildApiClient(resolvedBaseUrl);
  return _apiClient!;
}

/// Reset the singleton in tests or when rebuilding app dependencies.
@visibleForTesting
void resetApiClientForTesting() {
  _apiClient?.close(force: true);
  _apiClient = null;
}

Dio _buildApiClient(String baseUrl) {
  debugPrint('[API BASE] $baseUrl');
  final dio = Dio(
    BaseOptions(
      baseUrl: baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 30),
    ),
  );

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

/// Interceptor that logs every API request path.
class ApiPathLogger extends Interceptor {
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    debugPrint('[API] ${options.method} ${options.uri}');
    handler.next(options);
  }

  @override
  void onResponse(Response response, ResponseInterceptorHandler handler) {
    debugPrint(
      '[API] ${response.requestOptions.method} ${response.requestOptions.uri} -> ${response.statusCode}',
    );
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
