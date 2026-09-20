// Экран поста (specs/003-posts.md).
//
// Пост целиком: все фотографии и подпись без сокращений. Сюда попадают
// с ленты (specs/004-feed.md) и сразу после публикации.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/author_line.dart';

class PostScreen extends StatelessWidget {
  const PostScreen({super.key, required this.post});

  final Post post;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AppScreen(
      title: 'Пост',
      // Пост смотрят, а не проверяют связь: место лучше отдать
      // фотографиям.
      showServerStatus: false,
      child: ListView(
        children: [
          AuthorLine(author: post.author, when: post.createdAt),
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
}
