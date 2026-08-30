import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../../app/app_config.dart';
import '../../../app/app_shell.dart';
import '../../../shared/view_state.dart';
import 'auth_view_model.dart';
import 'login_page.dart';

class AuthGate extends StatelessWidget {
  const AuthGate({
    super.key,
    this.authenticatedBuilder,
    this.unauthenticatedBuilder,
  });

  final WidgetBuilder? authenticatedBuilder;
  final WidgetBuilder? unauthenticatedBuilder;

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();
    if (auth.state.isLoading || auth.state.status == ViewStatus.idle) {
      return const Center(child: CircularProgressIndicator());
    }
    if (!auth.isLoggedIn) {
      return unauthenticatedBuilder?.call(context) ?? const LoginPage();
    }
    return authenticatedBuilder?.call(context) ??
        const AuthenticatedDependencies(child: AppShell());
  }
}
