/// Network API client with Dio and request logging interceptor.
library;

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

/// Base API URL, override via environment or config.
const String kApiBaseUrl = 'http://localhost:8080';

/// Create a Dio instance with request logging interceptor.
Dio createApiClient({String? baseUrl}) {
  final dio = Dio(BaseOptions(
    baseUrl: baseUrl ?? kApiBaseUrl,
    connectTimeout: const Duration(seconds: 10),
    receiveTimeout: const Duration(seconds: 30),
  ));

  dio.interceptors.add(LogInterceptor(
    request: false,
    requestHeader: false,
    responseHeader: false,
    error: true,
    logPrint: (obj) {
      // Only print errors by default; use ApiPathLogger for path logging
    },
  ));

  dio.interceptors.add(ApiPathLogger());

  return dio;
}

/// Interceptor that logs every API request path.
class ApiPathLogger extends Interceptor {
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    debugPrint('[API] ${options.method} ${options.path}');
    handler.next(options);
  }

  @override
  void onResponse(Response response, ResponseInterceptorHandler handler) {
    debugPrint('[API] ${response.requestOptions.method} ${response.requestOptions.path} -> ${response.statusCode}');
    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    debugPrint('[API ERROR] ${err.requestOptions.method} ${err.requestOptions.path} -> ${err.message}');
    handler.next(err);
  }
}
