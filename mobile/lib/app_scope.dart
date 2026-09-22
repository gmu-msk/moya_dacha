// Доступ к перезапуску приложения и выходу изнутри экранов.
//
// Перезапуск нужен экрану «Сервер»: после смены адреса приложение
// должно начать жизнь заново (сессия стёрта, состояние сервиса
// перепроверено), а решает это корневой виджет. Выход нужен правке
// профиля, которая открывается из своего профиля, куда бы тот ни был
// открыт (specs/009-user-profile.md).
import 'package:flutter/widgets.dart';

class AppScope extends InheritedWidget {
  const AppScope({
    super.key,
    required this.restart,
    this.signOut,
    required super.child,
  });

  /// Перечитать адрес сервера и сессию и открыть приложение заново.
  final Future<void> Function() restart;

  /// Забыть сессию и вернуться ко входу.
  final Future<void> Function()? signOut;

  /// Может вернуть null: экраны показываются и вне приложения — в тестах
  /// и на витрине общих виджетов.
  static AppScope? of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppScope>();

  @override
  bool updateShouldNotify(AppScope oldWidget) => false;
}
