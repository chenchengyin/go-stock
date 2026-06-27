/// 应用依赖配置 — 集中管理所有依赖创建
library;

import 'package:provider/provider.dart';
import 'package:flutter/widgets.dart';

import '../core/storage/local_cache.dart';
import '../features/auth/data/auth_repository.dart';
import '../features/auth/presentation/auth_view_model.dart';
import '../features/strategy/presentation/strategy_view_model.dart';
import '../features/news/data/news_remote_datasource.dart';
import '../features/news/data/news_repository.dart';
import '../features/news/presentation/market_news/news_view_model.dart';
import '../features/radar/data/radar_repository.dart';
import '../features/radar/presentation/radar_list/radar_view_model.dart';

/// 创建所有 Provider 并返回 MultiProvider
class AppDependencies extends StatelessWidget {
  const AppDependencies({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final cache = MemoryLocalCache();

    return MultiProvider(
      providers: [
        ChangeNotifierProvider(
          create: (_) => AuthViewModel(MockAuthRepository(cache))..restore(),
        ),
        ChangeNotifierProvider(
          create: (_) => RadarViewModel(MockRadarRepository(cache))..load(),
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
