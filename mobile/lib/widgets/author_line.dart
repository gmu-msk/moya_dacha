// Строка автора над постом: аватар, имя и когда это было.
//
// Одна и та же в ленте и на экране поста — они должны выглядеть
// одинаково, иначе переход с ленты на пост читается как переход
// в другое приложение (ADR-0012).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import 'user_avatar.dart';

class AuthorLine extends StatelessWidget {
  const AuthorLine({super.key, required this.author, required this.when});

  final Author author;

  /// Когда пост выложен.
  final DateTime when;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: AuthorAvatar(author: author, radius: AvatarRadius.inPost),
      title: Text(author.name),
      subtitle: Text(whenPosted(when)),
    );
  }
}

/// Когда это было, словами: «5 минут назад», «вчера», «в прошлом месяце»
/// (specs/000-ui.md, правило 12). Точная дата дачнику не нужна: ему важно,
/// свежее это или прошлогоднее.
///
/// Считается от текущего момента в местном времени читателя. Часы и минуты
/// переходят в дни по календарю, а не делением на 24: пост в 23:30 назавтра
/// в 00:30 — это «вчера», а не «час назад».
///
/// [from] — «сейчас», от которого считаем; по умолчанию настоящее сейчас.
/// Задаётся он только в проверках: иначе они зависели бы от того, в какой
/// день их запустили.
String whenPosted(DateTime moment, {DateTime? from}) {
  final local = moment.toLocal();
  final now = from ?? DateTime.now();

  // Время поста впереди наших часов: часы разошлись, а не пост из будущего.
  if (local.isAfter(now)) {
    return 'только что';
  }

  final passed = now.difference(local);
  if (passed.inMinutes < 1) {
    return 'только что';
  }
  if (passed.inMinutes < 60) {
    return '${_count(passed.inMinutes, 'минуту', 'минуты', 'минут')} назад';
  }

  final days = _calendarDays(local, now);
  if (days == 0) {
    return '${_count(passed.inHours, 'час', 'часа', 'часов')} назад';
  }
  if (days == 1) {
    return 'вчера';
  }
  if (days < 7) {
    return '${_count(days, 'день', 'дня', 'дней')} назад';
  }

  final months = _calendarMonths(local, now);
  if (months == 0) {
    final weeks = days ~/ 7;
    return weeks == 1
        ? 'на прошлой неделе'
        : '${_count(weeks, 'неделю', 'недели', 'недель')} назад';
  }
  if (months == 1) {
    return 'в прошлом месяце';
  }
  if (months < 12) {
    return '${_count(months, 'месяц', 'месяца', 'месяцев')} назад';
  }

  final years = months ~/ 12;
  return years == 1
      ? 'в прошлом году'
      : '${_count(years, 'год', 'года', 'лет')} назад';
}

/// Сколько календарных дней между двумя моментами: полночь считается
/// границей суток, а не 24 часа от начала отсчёта.
int _calendarDays(DateTime from, DateTime to) {
  final fromDay = DateTime(from.year, from.month, from.day);
  final toDay = DateTime(to.year, to.month, to.day);
  return toDay.difference(fromDay).inDays;
}

/// Сколько целых месяцев прошло: 31 января и 1 марта — это месяц с лишним,
/// а 31 января и 28 февраля — ещё нет.
int _calendarMonths(DateTime from, DateTime to) {
  var months = (to.year - from.year) * 12 + to.month - from.month;
  if (to.day < from.day) {
    months -= 1;
  }
  return months < 0 ? 0 : months;
}

/// Число со словом в нужном падеже: 1 минуту, 2 минуты, 5 минут.
String _count(int number, String one, String few, String many) {
  final lastTwo = number % 100;
  final last = number % 10;

  if (lastTwo >= 11 && lastTwo <= 14) {
    return '$number $many';
  }
  if (last == 1) {
    return '$number $one';
  }
  if (last >= 2 && last <= 4) {
    return '$number $few';
  }
  return '$number $many';
}
