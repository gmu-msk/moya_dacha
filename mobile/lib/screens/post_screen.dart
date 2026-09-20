// Экран поста (specs/003-posts.md).
//
// Ленты ещё нет, и это единственное место, где пост видно: автор
// публикует и сразу попадает сюда.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/user_avatar.dart';

class PostScreen extends StatelessWidget {
  const PostScreen({super.key, required this.post});

  final Post post;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final avatar = post.author.avatarUrl;

    return AppScreen(
      title: 'Пост',
      // Пост смотрят, а не проверяют связь: место лучше отдать
      // фотографиям.
      showServerStatus: false,
      child: ListView(
        children: [
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: CircleAvatar(
              radius: AvatarRadius.inBar,
              backgroundColor: theme.colorScheme.primaryContainer,
              foregroundImage: avatar == null || avatar.isEmpty
                  ? null
                  : NetworkImage(mediaUrl(avatar)),
              child: Text(_initial(post.author.name)),
            ),
            title: Text(post.author.name),
            subtitle: Text(_when(post.createdAt)),
          ),
          for (final media in post.media)
            Padding(
              padding: const EdgeInsets.only(bottom: AppGap.small),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(AppGap.small),
                child: AspectRatio(
                  // Размеры приходят вместе с постом, поэтому место под
                  // фотографию занимается до того, как она загрузится,
                  // и экран не дёргается (specs/003-posts.md, требование 8).
                  aspectRatio: media.height == 0
                      ? 1
                      : media.width / media.height,
                  child: Image.network(mediaUrl(media.url), fit: BoxFit.cover),
                ),
              ),
            ),
          if (post.caption.isNotEmpty) ...[
            const SizedBox(height: AppGap.small),
            Text(post.caption, style: theme.textTheme.bodyLarge),
          ],
        ],
      ),
    );
  }

  static String _initial(String name) {
    final trimmed = name.trim();
    return trimmed.isEmpty ? '?' : trimmed.characters.first.toUpperCase();
  }

  static String _when(DateTime moment) {
    final local = moment.toLocal();
    return '${local.day.toString().padLeft(2, '0')}.'
        '${local.month.toString().padLeft(2, '0')}.${local.year} '
        '${local.hour.toString().padLeft(2, '0')}:'
        '${local.minute.toString().padLeft(2, '0')}';
  }
}
