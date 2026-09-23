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

  /// Значком «Профиль» в нижней панели (specs/011-bottom-bar.md).
  static const inBottomBar = 11.0;

  /// Рядом с постом и комментарием: вровень с двумя строками справа —
  /// именем и временем (specs/000-ui.md, правило 12).
  static const inPost = 24.0;

  /// Крупно на экране, как главное изображение.
  static const onScreen = 44.0;

  /// На своей странице профиля.
  static const inProfile = 56.0;
}

/// Чей аватар: свой или соседа. От этого зависит краска заливки —
/// имена и аватары авторов в ленте идут второй краской темы, свой аватар
/// в заголовке экрана — основной.
enum AvatarTone { own, author }

class UserAvatar extends StatelessWidget {
  const UserAvatar({super.key, required this.user, this.radius = 24});

  final CurrentUser user;
  final double radius;

  @override
  Widget build(BuildContext context) => Avatar(
    name: user.nickname,
    link: user.avatarUrl,
    radius: radius,
    tone: AvatarTone.own,
  );
}

/// Аватар автора — то же самое для публичного представления пользователя:
/// рядом с постом и комментарием (CONTEXT.md).
class AuthorAvatar extends StatelessWidget {
  const AuthorAvatar({super.key, required this.author, this.radius = 24});

  final Author author;
  final double radius;

  @override
  Widget build(BuildContext context) =>
      Avatar(name: author.nickname, link: author.avatarUrl, radius: radius);
}

class Avatar extends StatelessWidget {
  const Avatar({
    super.key,
    required this.name,
    required this.link,
    this.radius = 24,
    this.tone = AvatarTone.author,
  });

  final String name;
  final String? link;
  final double radius;
  final AvatarTone tone;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final link = this.link;
    final own = tone == AvatarTone.own;
    final background = own
        ? theme.colorScheme.primaryContainer
        : theme.colorScheme.secondaryContainer;
    final foreground = own
        ? theme.colorScheme.onPrimaryContainer
        : theme.colorScheme.onSecondaryContainer;

    return CircleAvatar(
      radius: radius,
      backgroundColor: background,
      foregroundImage: link == null || link.isEmpty
          ? null
          : NetworkImage(mediaUrl(link)),
      child: Text(
        _initial(name),
        style: TextStyle(fontSize: radius * 0.8, color: foreground),
      ),
    );
  }

  static String _initial(String name) {
    final trimmed = name.trim();
    return trimmed.isEmpty ? '?' : trimmed.characters.first.toUpperCase();
  }
}
