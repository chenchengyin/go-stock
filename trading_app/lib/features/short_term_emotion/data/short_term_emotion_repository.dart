import 'package:dio/dio.dart';

import '../../../core/network/api_client.dart';
import '../domain/short_term_emotion_models.dart';

class ShortTermEmotionRepository {
  ShortTermEmotionRepository({Dio? dio}) : _dio = dio ?? createApiClient();

  final Dio _dio;

  Future<ShortTermEmotion> fetch() async {
    final response = await _dio.get('/api/short-term-emotion');
    if (response.statusCode == 200 && response.data is Map) {
      return ShortTermEmotion.fromJson(
        Map<String, dynamic>.from(response.data as Map),
      );
    }
    throw Exception('获取超短情绪失败: ${response.statusCode}');
  }
}
