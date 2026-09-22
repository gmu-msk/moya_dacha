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
import '../widgets/comments_view.dart';
import '../widgets/confirm.dart';
import '../widgets/like_button.dart';
import '../widgets/report_dialog.dart';
import 'user_screen.dart';

class PostScreen extends StatefulWidget {
  const PostScreen({
    super.key,
    required this.post,
    required this.token,
    required this.viewerId,
    this.onChanged,
  });

  final Post post;
  final String token;

  /// Кто смотрит: у своего поста и своего комментария есть «Удалить»,
  /// у чужого — «Пожаловаться» (specs/007-deletion.md, требование 12,
  /// specs/008-reports.md, требование 11).
  final String viewerId;

  /// Пост изменился: его лайкнули здесь, и лента должна показать то же
  /// число (specs/005-likes.md).
  final void Function(Post post)? onChanged;

  @override
  State<PostScreen> createState() => _PostScreenState();
}

class _PostScreenState extends State<PostScreen> {
  late Post post = widget.post;

  bool _deleting = false;

  bool get _mine => post.author.id == widget.viewerId;

  /// Перечитать пост: после своего комментария у него другое число, и
  /// показать его должны и этот экран, и лента (specs/006-comments.md,
  /// требование 7).
  Future<void> _reload() async {
    try {
      final updated = await PostsApi(
        apiClient(token: widget.token),
      ).getPost(post.id);
      if (!mounted || updated == null) {
        return;
      }
      setState(() => post = updated);
      widget.onChanged?.call(updated);
    } on Exception catch (error) {
      // Комментарий уже оставлен и виден: молчаливо разойтись здесь
      // лучше, чем ругаться на то, что человеку удалось.
      debugPrint('$logMarker post=reload_failed error=$error');
    }
  }

  /// Удалить свой пост. Возвращаемся в ленту: показывать экран того,
  /// чего больше нет, нечестно (specs/007-deletion.md).
  Future<void> _delete() async {
    final agreed = await confirmDelete(
      context,
      title: 'Удалить пост?',
      question: 'Пост, его фотографии, лайки и комментарии исчезнут '
          'безвозвратно.',
    );
    if (!agreed || !mounted) {
      return;
    }

    setState(() => _deleting = true);
    try {
      await PostsApi(apiClient(token: widget.token)).deletePost(post.id);
      debugPrint('$logMarker post=deleted id=${post.id}');
      if (!mounted) {
        return;
      }
      Navigator.of(context).pop(true);
    } on Exception catch (error) {
      debugPrint('$logMarker post=delete_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() => _deleting = false);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(errorMessage(error))));
    }
  }

  /// Пожаловаться на чужой пост. На экране от этого не меняется ничего:
  /// жалоба — сигнал владельцу сервиса, а не действие над постом
  /// (specs/008-reports.md, требование 12).
  Future<void> _report() async {
    await askAndReport(
      context,
      title: 'Пожаловаться на пост?',
      question: 'Жалобу посмотрит владелец сервиса. Пост останется на '
          'месте, и автор о ней не узнает.',
      send: (reason) => PostsApi(
        apiClient(token: widget.token),
      ).reportPost(post.id, reportDraft: ReportDraft(reason: reason)),
    );
  }

  /// Профиль автора поста или комментария (specs/009-user-profile.md,
  /// требование 9).
  void _openAuthor(Author author) => openUserProfile(
    context,
    token: widget.token,
    viewerId: widget.viewerId,
    userId: author.id,
    onPostChanged: (updated) {
      if (updated.id == post.id) {
        setState(() => post = updated);
        widget.onChanged?.call(updated);
      }
    },
  );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AppScreen(
      title: 'Пост',
      // Пост смотрят, а не проверяют связь: место лучше отдать
      // фотографиям.
      showServerStatus: false,
      actions: [
        if (_mine)
          IconButton(
            tooltip: 'Удалить пост',
            onPressed: _deleting ? null : _delete,
            icon: const Icon(Icons.delete_outline),
          )
        else
          IconButton(
            tooltip: 'Пожаловаться на пост',
            onPressed: _report,
            icon: const Icon(Icons.flag_outlined),
          ),
      ],
      child: ListView(
        children: [
          AuthorLine(
            author: post.author,
            when: post.createdAt,
            onTap: () => _openAuthor(post.author),
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
          const SizedBox(height: AppGap.small),
          Align(
            alignment: Alignment.centerLeft,
            child: LikeButton(
              post: post,
              token: widget.token,
              onChanged: (updated) {
                setState(() => post = updated);
                widget.onChanged?.call(updated);
              },
            ),
          ),
          const SizedBox(height: AppGap.medium),
          CommentsView(
            postId: post.id,
            token: widget.token,
            viewerId: widget.viewerId,
            onChanged: _reload,
            onOpenAuthor: _openAuthor,
          ),
        ],
      ),
    );
  }
}
