// Каркас экрана: заголовок, поля, состояние сервиса внизу.
//
// Был одинаково повторён в экране входа и на главном (ADR-0012).
import 'package:flutter/material.dart';

import '../theme.dart';
import 'server_status.dart';

class AppScreen extends StatelessWidget {
  const AppScreen({
    super.key,
    required this.child,
    this.title = 'МояДача',
    this.actions,
    this.floatingActionButton,
    this.padded = true,
    this.showServerStatus = true,
  });

  /// Содержимое экрана. Занимает всё место над строкой состояния сервиса.
  final Widget child;

  final String title;

  /// Кнопки справа в заголовке: например, аватар, открывающий профиль.
  final List<Widget>? actions;

  /// Главное действие экрана, если оно должно быть под рукой при
  /// прокрутке: «Новый пост» в ленте.
  final Widget? floatingActionButton;

  /// Поля по краям содержимого. Лента отступы задаёт себе сама:
  /// фотографии на маленьком экране должны идти во всю ширину.
  final bool padded;

  /// Строка «Сервер отвечает, база жива» внизу (demo/stories/000-status).
  final bool showServerStatus;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(title), actions: actions),
      floatingActionButton: floatingActionButton,
      body: SafeArea(
        child: Padding(
          padding: padded
              ? const EdgeInsets.all(AppGap.large)
              : const EdgeInsets.symmetric(vertical: AppGap.small),
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
