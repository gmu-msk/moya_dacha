// Доступ к перезапуску приложения изнутри экранов.
//
// Нужен ровно одному экрану — «Сервер»: после смены адреса приложение
// должно начать жизнь заново (сессия стёрта, состояние сервиса
// перепроверено), а решает это корневой виджет.
import 'package:flutter/widgets.dart';

class AppScope extends InheritedWidget {
  const AppScope({super.key, required this.restart, required super.child});

  /// Перечитать адрес сервера и сессию и открыть приложение заново.
  final Future<void> Function() restart;

  /// Может вернуть null: экраны показываются и вне приложения — в тестах
  /// и на витрине общих виджетов.
  static AppScope? of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppScope>();

  @override
  bool updateShouldNotify(AppScope oldWidget) => false;
}
