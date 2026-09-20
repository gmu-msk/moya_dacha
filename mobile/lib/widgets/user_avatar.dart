// Аватар пользователя.
//
// Пока аватара нет, показывается первая буква имени: пустой кружок
// ничего не говорит о том, кто это (specs/002-profile.md).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';

class UserAvatar extends StatelessWidget {
  const UserAvatar({super.key, required this.user, this.radius = 24});

  final CurrentUser user;
  final double radius;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final link = user.avatarUrl;

    return CircleAvatar(
      radius: radius,
      backgroundColor: theme.colorScheme.primaryContainer,
      foregroundImage: link == null || link.isEmpty
          ? null
          : NetworkImage(mediaUrl(link)),
      child: Text(
        _initial(user.name),
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
