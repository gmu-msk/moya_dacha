// Экран поста (specs/003-posts.md).
//
// Вид «Сад» (2a): фото во всю ширину 4:5 листаются свайпом, как в ленте;
// из ленты фото «раскрывается» сюда переходом-героем. Двойное касание
// ставит отметку. Ниже — автор, подпись целиком, отметка и комментарии.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/author_line.dart';
import '../widgets/comments_view.dart';
import '../widgets/confirm.dart';
import '../widgets/edit_text_dialog.dart';
import '../widgets/feed_view.dart';
import '../widgets/like_button.dart';
import '../widgets/place_field.dart';
import '../widgets/report_dialog.dart';
import '../widgets/visibility_picker.dart';
import 'user_screen.dart';

class PostScreen extends StatefulWidget {
  const PostScreen({
    super.key,
    required this.post,
    required this.token,
    required this.viewerId,
    this.onChanged,
    this.heroTag,
  });

  final Post post;
  final String token;
  final String viewerId;
  final void Function(Post post)? onChanged;

  /// Тег фото в ленте, из которого экран раскрылся.
  final Object? heroTag;

  @override
  State<PostScreen> createState() => _PostScreenState();
}

class _PostScreenState extends State<PostScreen> {
  late Post post = widget.post;

  final _like = GlobalKey<LikeButtonState>();
  final _heart = GlobalKey<BigHeartState>();

  bool _deleting = false;

  bool get _mine => post.author.id == widget.viewerId;

  Future<void> _reload() async {
    try {
      final updated = await PostsApi(apiClient(token: widget.token))
          .getPost(post.id);
      if (!mounted || updated == null) {
        return;
      }
      setState(() => post = updated);
      widget.onChanged?.call(updated);
    } on Exception catch (error) {
      debugPrint('$logMarker post=reload_failed error=$error');
    }
  }

  Future<void> _delete() async {
    final agreed = await confirmDelete(
      context,
      title: 'Удалить пост?',
      question:
          'Пост, его фотографии, лайки и комментарии исчезнут '
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
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(errorMessage(error))));
    }
  }

  /// Поправить подпись. Фотографии, лайки и комментарии остаются
  /// (specs/022-edit-block-delete.md, требование 1).
  Future<void> _editCaption() async {
    final updated = await editText<Post>(
      context,
      title: 'Изменить подпись',
      initial: post.caption,
      label: 'Подпись',
      maxLength: maxCaptionLength,
      allowEmpty: true,
      save: (text) =>
          PostsApi(apiClient(token: widget.token))
              .editCaption(post.id, CaptionUpdate(caption: text)),
    );
    if (!mounted || updated == null) {
      return;
    }
    debugPrint('$logMarker post=caption_edited id=${post.id}');
    _changed(updated);
  }

  Future<void> _changeVisibility() async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      final me = await ProfileApi(apiClient(token: widget.token)).getMe();
      if (!mounted) {
        return;
      }
      final closed = me?.closed ?? false;
      final picked = await pickVisibility(
        context,
        current: post.visibility,
        closed: closed,
      );
      if (picked == null || picked == post.visibility) {
        return;
      }
      final updated = await PostsApi(apiClient(token: widget.token))
          .setPostVisibility(post.id, PostVisibilityUpdate(visibility: picked));
      debugPrint('$logMarker post=visibility id=${post.id} value=$picked');
      if (!mounted || updated == null) {
        return;
      }
      setState(() => post = updated);
      widget.onChanged?.call(updated);
      messenger.showSnackBar(
        SnackBar(content: Text(visibilityChanged(picked, closed: closed))),
      );
    } on Exception catch (error) {
      debugPrint('$logMarker post=visibility_failed error=$error');
      messenger.showSnackBar(SnackBar(content: Text(errorMessage(error))));
    }
  }

  Future<void> _report() async {
    await askAndReport(
      context,
      title: 'Пожаловаться на пост?',
      question:
          'Жалобу посмотрит владелец сервиса. Пост останется на '
          'месте, и автор о ней не узнает.',
      send: (reason) =>
          PostsApi(apiClient(token: widget.token))
              .reportPost(post.id, reportDraft: ReportDraft(reason: reason)),
    );
  }

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

  void _changed(Post updated) {
    setState(() => post = updated);
    widget.onChanged?.call(updated);
  }

  @override
  void initState() {
    super.initState();
    usage.screen('post');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    Widget photos = FeedPhotos(media: post.media, radius: 0);
    final tag = widget.heroTag;
    if (tag != null) {
      photos = Hero(tag: tag, child: photos);
    }

    return AppScreen(
      untitled: true,
      showServerStatus: false,
      padded: false,
      actions: [
        if (_mine)
          IconButton(
            tooltip: 'Кто увидит',
            onPressed: _deleting ? null : _changeVisibility,
            icon: Icon(visibilityIcon(post.visibility)),
          ),
        if (_mine)
          IconButton(
            tooltip: 'Изменить подпись',
            onPressed: _deleting ? null : _editCaption,
            icon: const Icon(Icons.edit_outlined),
          ),
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
        padding: const EdgeInsets.only(bottom: AppGap.large),
        children: [
          if (post.media.isNotEmpty)
            GestureDetector(
              onDoubleTap: () {
                _heart.currentState?.play();
                _like.currentState?.likeByDoubleTap();
              },
              child: Stack(
                children: [
                  photos,
                  Positioned.fill(child: BigHeart(key: _heart)),
                ],
              ),
            ),
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppGap.medium,
              AppGap.medium,
              AppGap.medium,
              0,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                AuthorLine(
                  author: post.author,
                  when: post.createdAt,
                  visibility: post.visibility,
                  edited: post.editedAt != null,
                  onTap: () => _openAuthor(post.author),
                ),
                if (post.place case final place?) ...[
                  const SizedBox(height: AppGap.small),
                  PostPlaceLine(place: place, distanceKm: post.distanceKm),
                ],
                if (post.caption.isNotEmpty) ...[
                  const SizedBox(height: AppGap.small),
                  Text(post.caption, style: theme.textTheme.bodyLarge),
                ],
                Align(
                  alignment: Alignment.centerLeft,
                  child: LikeButton(
                    key: _like,
                    post: post,
                    token: widget.token,
                    onChanged: _changed,
                  ),
                ),
                const SizedBox(height: AppGap.small),
                CommentsView(
                  postId: post.id,
                  token: widget.token,
                  viewerId: widget.viewerId,
                  onChanged: _reload,
                  onOpenAuthor: _openAuthor,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
