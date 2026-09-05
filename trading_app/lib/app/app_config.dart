/// 应用依赖配置 — 集中管理所有依赖创建
library;

import 'package:flutter/widgets.dart';
import 'package:provider/provider.dart';
import 'package:dio/dio.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/core/storage/local_cache.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/data/auth_storage.dart';
import 'package:trading_app/features/auth/data/session_controller.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/strategy/presentation/strategy_view_model.dart';
import 'package:trading_app/features/news/data/news_remote_datasource.dart';
import 'package:trading_app/features/news/data/news_repository.dart';
import 'package:trading_app/features/news/presentation/market_news/news_view_model.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';
import 'package:trading_app/features/permissions/data/module_permission_repository.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'package:trading_app/features/short_term_emotion/data/short_term_emotion_repository.dart';
import 'package:trading_app/features/short_term_emotion/presentation/short_term_emotion_view_model.dart';

/// 创建所有 Provider 并返回 MultiProvider
class AppDependencies extends StatelessWidget {
  const AppDependencies({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final authStorage = SharedPreferencesAuthStorage();

    return MultiProvider(
      providers: [
        ChangeNotifierProvider(create: (_) => ThemeManager()..restore()),
        ChangeNotifierProvider<SessionController>(
          create: (_) => AuthSessionController(authStorage),
        ),
        Provider<Dio>(
          create: (context) => createApiClient(
            sessionController: context.read<SessionController>(),
          ),
        ),
        Provider<AuthRepository>(
          create: (context) => ApiAuthRepository(
            dio: context.read<Dio>(),
            storage: authStorage,
            sessionController: context.read<SessionController>(),
          ),
        ),
        ChangeNotifierProvider(
          create: (context) => AuthViewModel(
            context.read<AuthRepository>(),
            sessionController: context.read<SessionController>(),
          )..restore(),
        ),
      ],
      child: child,
    );
  }
}

/// Dependencies that may load protected, per-user business data.
class AuthenticatedDependencies extends StatelessWidget {
  const AuthenticatedDependencies({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final dio = context.read<Dio>();

    return MultiProvider(
      providers: [
        Provider<ModulePermissionSource>(
          create: (_) => ModulePermissionRepository(dio),
        ),
        ChangeNotifierProvider<ModulePermissionController>(
          create: (context) =>
              ModulePermissionController(context.read<ModulePermissionSource>())
                ..load(),
        ),
        Provider<RadarRepository>(create: (_) => RadarRepositoryImpl(dio: dio)),
        ChangeNotifierProvider(
          create: (context) =>
              RadarViewModel(context.read<RadarRepository>())
                ..loadMonitoredStocks(),
        ),
        ChangeNotifierProvider(create: (_) => VoiceAnnouncementViewModel()),
        ChangeNotifierProvider(
          create: (context) => T0StrategyViewModel(
            fetchRealtimeQuotes: context
                .read<RadarRepository>()
                .fetchRealtimeQuotes,
            onModuleForbidden: context
                .read<ModulePermissionController>()
                .revoke,
          ),
        ),
        ChangeNotifierProvider(
          create: (_) =>
              ShortTermEmotionViewModel(ShortTermEmotionRepository(dio: dio))
                ..load(),
        ),
        ChangeNotifierProvider(
          create: (_) {
            final remote = NewsRemoteDataSource(dio: dio);
            final repo = NewsRepositoryImpl(
              cache: MemoryLocalCache(),
              remoteDataSource: remote,
            );
            return NewsViewModel(repo);
          },
        ),
        ChangeNotifierProvider(create: (_) => StrategyViewModel(dio: dio)),
      ],
      child: child,
    );
  }
}
