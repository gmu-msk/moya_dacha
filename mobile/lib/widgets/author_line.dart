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
      leading: AuthorAvatar(author: author, radius: AvatarRadius.inBar),
      title: Text(author.name),
      subtitle: Text(whenPosted(when)),
    );
  }
}

/// Когда пост выложен, в местном времени читателя.
String whenPosted(DateTime moment) {
  final local = moment.toLocal();
  return '${local.day.toString().padLeft(2, '0')}.'
      '${local.month.toString().padLeft(2, '0')}.${local.year} '
      '${local.hour.toString().padLeft(2, '0')}:'
      '${local.minute.toString().padLeft(2, '0')}';
}
