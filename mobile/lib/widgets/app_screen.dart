// Каркас экрана: заголовок, поля, состояние сервиса внизу.
//
// Был одинаково повторён в экране входа и на главном (ADR-0011).
import 'package:flutter/material.dart';

import '../theme.dart';
import 'server_status.dart';

class AppScreen extends StatelessWidget {
  const AppScreen({
    super.key,
    required this.child,
    this.title = 'МояДача',
    this.showServerStatus = true,
  });

  /// Содержимое экрана. Занимает всё место над строкой состояния сервиса.
  final Widget child;

  final String title;

  /// Строка «Сервер отвечает, база жива» внизу (demo/stories/000-status).
  final bool showServerStatus;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(AppGap.large),
          child: Column(
            children: [
              Expanded(child: child),
              if (showServerStatus) const ServerStatus(),
            ],
          ),
        ),
      ),
    );
  }
}
