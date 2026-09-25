// Строка автора над постом: аватар, имя и когда это было.
//
// Одна и та же в ленте и на экране поста — они должны выглядеть
// одинаково, иначе переход с ленты на пост читается как переход
// в другое приложение (ADR-0012).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import 'user_avatar.dart';
import 'visibility_picker.dart';

class AuthorLine extends StatelessWidget {
  const AuthorLine({
    super.key,
    required this.author,
    required this.when,
    this.visibility,
    this.onTap,
  });

  final Author author;

  /// Когда пост выложен.
  final DateTime when;

  /// Кто видит пост: у «друзьям» и «только мне» рядом со временем
  /// отметка (specs/013-post-visibility.md, требование 7).
  final PostVisibility? visibility;

  /// Открыть профиль автора: имя и аватар ведут к нему
  /// (specs/009-user-profile.md, требование 9).
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return ListTile(
      contentPadding: EdgeInsets.zero,
      // Плотнее материаловой строки: аватар 48 и две строки текста
      // занимают 56, а не 72 — пост компактнее (specs/000-ui.md, «Вид»).
      // Цель касания остаётся не меньше 48.
      visualDensity: const VisualDensity(vertical: -4),
      minVerticalPadding: 0,
      onTap: onTap,
      leading: AuthorAvatar(author: author, radius: AvatarRadius.inPost),
      // Никнейм — вторая краска темы: по ней видно, где кончается один пост
      // и начинается следующий, даже когда фотографии похожи.
      title: Text(
        author.nickname,
        style: theme.textTheme.titleMedium?.copyWith(
          color: theme.colorScheme.secondary,
        ),
      ),
      subtitle: PostedLine(when: when, visibility: visibility),
    );
  }
}

/// Когда выложен пост и, если он не для всех, кому: «вчера · 👥 друзьям».
class PostedLine extends StatelessWidget {
  const PostedLine({
    super.key,
    required this.when,
    this.visibility,
    this.style,
  });

  final DateTime when;
  final PostVisibility? visibility;
  final TextStyle? style;

  @override
  Widget build(BuildContext context) {
    final visibility = this.visibility;
    final mark = visibility == null ? null : visibilityMark(visibility);
    if (visibility == null || mark == null) {
      return Text(whenPosted(when), style: style);
    }
    final base = style ?? DefaultTextStyle.of(context).style;
    return Text.rich(
      TextSpan(
        text: '${whenPosted(when)} · ',
        children: [
          WidgetSpan(
            alignment: PlaceholderAlignment.middle,
            child: Icon(
              visibilityIcon(visibility),
              size: base.fontSize,
              color: base.color,
            ),
          ),
          TextSpan(text: ' $mark'),
        ],
      ),
      style: style,
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
    return '${countWord(passed.inMinutes, 'минуту', 'минуты', 'минут')} назад';
  }

  final days = _calendarDays(local, now);
  if (days == 0) {
    return '${countWord(passed.inHours, 'час', 'часа', 'часов')} назад';
  }
  if (days == 1) {
    return 'вчера';
  }
  if (days < 7) {
    return '${countWord(days, 'день', 'дня', 'дней')} назад';
  }

  final months = _calendarMonths(local, now);
  if (months == 0) {
    final weeks = days ~/ 7;
    return weeks == 1
        ? 'на прошлой неделе'
        : '${countWord(weeks, 'неделю', 'недели', 'недель')} назад';
  }
  if (months == 1) {
    return 'в прошлом месяце';
  }
  if (months < 12) {
    return '${countWord(months, 'месяц', 'месяца', 'месяцев')} назад';
  }

  final years = months ~/ 12;
  return years == 1
      ? 'в прошлом году'
      : '${countWord(years, 'год', 'года', 'лет')} назад';
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
String countWord(int number, String one, String few, String many) {
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
