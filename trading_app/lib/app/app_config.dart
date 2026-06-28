/// 应用依赖配置 — 集中管理所有依赖创建
library;

import 'package:provider/provider.dart';
import 'package:flutter/widgets.dart';
import 'package:trading_app/core/storage/local_cache.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/strategy/presentation/strategy_view_model.dart';
import 'package:trading_app/features/news/data/news_remote_datasource.dart';
import 'package:trading_app/features/news/data/news_repository.dart';
import 'package:trading_app/features/news/presentation/market_news/news_view_model.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';

/// 创建所有 Provider 并返回 MultiProvider
class AppDependencies extends StatelessWidget {
  const AppDependencies({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final cache = MemoryLocalCache();
    final radarRepo = RadarRepositoryImpl();

    return MultiProvider(
      providers: [
        ChangeNotifierProvider(
          create: (_) => AuthViewModel(MockAuthRepository(cache))..restore(),
        ),
        ChangeNotifierProvider(
          create: (_) => RadarViewModel(radarRepo)..loadMonitoredStocks(),
        ),
        Provider<RadarRepository>(create: (_) => radarRepo),
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
