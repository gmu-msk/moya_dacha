// Аватар пользователя.
//
// Пока аватара нет, показывается первая буква имени: пустой кружок
// ничего не говорит о том, кто это (specs/002-profile.md).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';

/// Размеры аватара. Своих чисел экраны не придумывают (ADR-0012).
abstract final class AvatarRadius {
  /// Кнопкой в заголовке экрана.
  static const inBar = 16.0;

  /// Рядом с постом и комментарием: вровень с двумя строками справа —
  /// именем и временем (specs/000-ui.md, правило 12).
  static const inPost = 24.0;

  /// Крупно на экране, как главное изображение.
  static const onScreen = 44.0;

  /// На своей странице профиля.
  static const inProfile = 56.0;
}

class UserAvatar extends StatelessWidget {
  const UserAvatar({super.key, required this.user, this.radius = 24});

  final CurrentUser user;
  final double radius;

  @override
  Widget build(BuildContext context) =>
      Avatar(name: user.name, link: user.avatarUrl, radius: radius);
}

/// Аватар автора — то же самое для публичного представления пользователя:
/// рядом с постом и комментарием (CONTEXT.md).
class AuthorAvatar extends StatelessWidget {
  const AuthorAvatar({super.key, required this.author, this.radius = 24});

  final Author author;
  final double radius;

  @override
  Widget build(BuildContext context) =>
      Avatar(name: author.name, link: author.avatarUrl, radius: radius);
}

class Avatar extends StatelessWidget {
  const Avatar({
    super.key,
    required this.name,
    required this.link,
    this.radius = 24,
  });

  final String name;
  final String? link;
  final double radius;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final link = this.link;

    return CircleAvatar(
      radius: radius,
      backgroundColor: theme.colorScheme.primaryContainer,
      foregroundImage: link == null || link.isEmpty
          ? null
          : NetworkImage(mediaUrl(link)),
      child: Text(
        _initial(name),
        style: TextStyle(
          fontSize: radius * 0.8,
          color: theme.colorScheme.onPrimaryContainer,
        ),
      ),
    );
  }

  static String _initial(String name) {
    final trimmed = name.trim();
    return trimmed.isEmpty ? '?' : trimmed.characters.first.toUpperCase();
  }
}
