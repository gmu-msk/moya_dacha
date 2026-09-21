// Каркас экрана: заголовок, поля, состояние сервиса значком в заголовке.
//
// Был одинаково повторён в экране входа и на главном (ADR-0012).
import 'package:flutter/material.dart';

import '../theme.dart';
import 'app_logo.dart';
import 'server_status.dart';

class AppScreen extends StatelessWidget {
  const AppScreen({
    super.key,
    required this.child,
    this.title,
    this.actions,
    this.floatingActionButton,
    this.padded = true,
    this.showServerStatus = true,
  });

  /// Содержимое экрана. Занимает всё место под заголовком.
  final Widget child;

  /// Куда экран привёл: «Профиль», «Новый пост». Пусто — главный экран
  /// приложения, и в заголовке стоит логотип, а не слово
  /// (specs/000-ui.md, правило 14).
  final String? title;

  /// Кнопки справа в заголовке: например, аватар, открывающий профиль.
  final List<Widget>? actions;

  /// Главное действие экрана, если оно должно быть под рукой при
  /// прокрутке: «Новый пост» в ленте.
  final Widget? floatingActionButton;

  /// Поля по краям содержимого. Лента отступы задаёт себе сама:
  /// фотографии на маленьком экране должны идти во всю ширину.
  final bool padded;

  /// Точка состояния сервиса в заголовке (demo/stories/000-status).
  final bool showServerStatus;

  @override
  Widget build(BuildContext context) {
    final title = this.title;

    return Scaffold(
      appBar: AppBar(
        title: title == null ? const AppLogo() : Text(title),
        // Логотип стоит с краю, слово — по центру, как Material 3 и просит.
        centerTitle: title == null ? false : null,
        actions: [
          if (showServerStatus) const ServerStatus(),
          ...?actions,
        ],
      ),
      floatingActionButton: floatingActionButton,
      body: SafeArea(
        child: Padding(
          padding: padded
              ? const EdgeInsets.all(AppGap.large)
              : const EdgeInsets.symmetric(vertical: AppGap.small),
          child: child,
        ),
      ),
    );
  }
}
