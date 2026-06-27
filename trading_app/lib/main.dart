import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'app/app_shell.dart';
import 'core/network/api_client.dart';
import 'core/storage/local_cache.dart';
import 'core/theme/app_theme.dart';
import 'features/auth/data/auth_repository.dart';
import 'features/auth/presentation/view_models/auth_view_model.dart';
import 'features/hotlist/data/hotlist_repository.dart';
import 'features/hotlist/presentation/view_models/hotlist_view_model.dart';
import 'features/news/data/news_remote_datasource.dart';
import 'features/news/data/news_repository.dart';
import 'features/news/presentation/view_models/news_view_model.dart';
import 'features/radar/data/radar_repository.dart';
import 'features/radar/presentation/view_models/radar_view_model.dart';

void main() {
  final cache = MemoryLocalCache();
  final authRepository = MockAuthRepository(cache);
  final radarRepository = MockRadarRepository(cache);
  final hotlistRepository = MockHotlistRepository(cache);

  // Create shared Dio client
  final dio = createApiClient();

  // Create news data source and repository
  final newsRemoteDataSource = NewsRemoteDataSource(dio: dio);
  final newsRepository = NewsRepositoryImpl(
    cache: cache,
    remoteDataSource: newsRemoteDataSource,
  );

  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider(
          create: (_) => AuthViewModel(authRepository)..restore(),
        ),
        ChangeNotifierProvider(
          create: (_) => RadarViewModel(radarRepository)..load(),
        ),
        ChangeNotifierProvider(
          create: (_) => NewsViewModel(newsRepository)..load(),
        ),
        ChangeNotifierProvider(
          create: (_) => HotlistViewModel(hotlistRepository)..load(),
        ),
      ],
      child: const TradingRadarApp(),
    ),
  );
}

class TradingRadarApp extends StatelessWidget {
  const TradingRadarApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '交易雷达',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.light(),
      home: const AppShell(),
    );
  }
}

