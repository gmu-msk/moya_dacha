// Строка человека в списке: аватар, никнейм, полное имя и кнопка справа
// (specs/012-follows.md, требование 14).
import 'package:flutter/material.dart';

import '../theme.dart';
import 'user_avatar.dart';

class PersonRow extends StatelessWidget {
  const PersonRow({
    super.key,
    required this.nickname,
    required this.name,
    required this.avatarUrl,
    this.subtitle,
    this.trailing,
    this.onTap,
  });

  final String nickname;
  final String name;
  final String? avatarUrl;

  /// Что под никнеймом вместо полного имени: «Это вы».
  final String? subtitle;

  /// Кнопка справа.
  final Widget? trailing;

  /// Открыть профиль человека.
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final below = subtitle ?? name;

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
      onTap: onTap,
      leading: Avatar(
        name: nickname,
        link: avatarUrl,
        radius: AvatarRadius.inPost,
      ),
      // Никнейм — вторая краска темы, как у автора в ленте.
      title: Text(
        nickname,
        overflow: TextOverflow.ellipsis,
        style: theme.textTheme.titleMedium?.copyWith(
          color: theme.colorScheme.secondary,
        ),
      ),
      subtitle: below.isEmpty
          ? null
          : Text(below, overflow: TextOverflow.ellipsis),
      trailing: trailing,
    );
  }
}
