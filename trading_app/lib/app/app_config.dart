/// 应用依赖配置 — 集中管理所有依赖创建
library;

import 'package:flutter/widgets.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/core/storage/local_cache.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/data/auth_storage.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/strategy/presentation/strategy_view_model.dart';
import 'package:trading_app/features/news/data/news_remote_datasource.dart';
import 'package:trading_app/features/news/data/news_repository.dart';
import 'package:trading_app/features/news/presentation/market_news/news_view_model.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';
import 'package:trading_app/features/short_term_emotion/data/short_term_emotion_repository.dart';
import 'package:trading_app/features/short_term_emotion/presentation/short_term_emotion_view_model.dart';

/// 创建所有 Provider 并返回 MultiProvider
class AppDependencies extends StatelessWidget {
  const AppDependencies({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final cache = MemoryLocalCache();
    final authStorage = SharedPreferencesAuthStorage();
    final radarRepo = RadarRepositoryImpl();

    return MultiProvider(
      providers: [
        ChangeNotifierProvider(create: (_) => ThemeManager()..restore()),
        ChangeNotifierProvider(
          create: (_) => AuthViewModel(
            ApiAuthRepository(dio: createApiClient(), storage: authStorage),
          )..restore(),
        ),
        ChangeNotifierProvider(
          create: (_) => RadarViewModel(radarRepo)..loadMonitoredStocks(),
        ),
        ChangeNotifierProvider(create: (_) => VoiceAnnouncementViewModel()),
        ChangeNotifierProvider(
          create: (_) => T0StrategyViewModel(
            fetchRealtimeQuotes: radarRepo.fetchRealtimeQuotes,
          ),
        ),
        Provider<RadarRepository>(create: (_) => radarRepo),
        ChangeNotifierProvider(
          create: (_) =>
              ShortTermEmotionViewModel(ShortTermEmotionRepository())..load(),
        ),
        ChangeNotifierProvider(
          create: (_) {
            final remote = NewsRemoteDataSource();
            final repo = NewsRepositoryImpl(
              cache: cache,
              remoteDataSource: remote,
            );
            return NewsViewModel(repo)..load();
          },
        ),
        ChangeNotifierProvider(create: (_) => StrategyViewModel()),
      ],
      child: child,
    );
  }
}
