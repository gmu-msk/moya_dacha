// Посты с тэгом: specs/028-post-tags.md, требование 24.
//
// Открывается касанием тэга в карточке или на экране поста. Это та же
// лента «Все», только суженная до одного тэга: новые сверху, подгрузка
// по прокрутке.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/feed_view.dart';
import 'post_screen.dart';
import 'user_screen.dart';

/// Открыть экран «#тэг» поверх текущего.
Future<void> openTagPosts(
  BuildContext context, {
  required String token,
  required String viewerId,
  required String tag,
  void Function(Post post)? onPostChanged,
}) => Navigator.of(context).push(
  MaterialPageRoute<void>(
    builder: (_) => TagPostsScreen(
      token: token,
      viewerId: viewerId,
      tag: tag,
      onPostChanged: onPostChanged,
    ),
  ),
);

class TagPostsScreen extends StatefulWidget {
  const TagPostsScreen({
    super.key,
    required this.token,
    required this.viewerId,
    required this.tag,
    this.onPostChanged,
  });

  final String token;
  final String viewerId;
  final String tag;
  final void Function(Post post)? onPostChanged;

  @override
  State<TagPostsScreen> createState() => _TagPostsScreenState();
}

class _TagPostsScreenState extends State<TagPostsScreen> {
  final _feed = GlobalKey<FeedViewState>();

  @override
  void initState() {
    super.initState();
    usage.screen('tag_posts');
  }

  void _changed(Post post) {
    _feed.currentState?.replace(post);
    widget.onPostChanged?.call(post);
  }

  Future<void> _open(Post post) async {
    final deleted = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => PostScreen(
          post: post,
          token: widget.token,
          viewerId: widget.viewerId,
          heroTag: postHeroTag(post),
          onChanged: _changed,
        ),
      ),
    );
    if (deleted == true) {
      await _feed.currentState?.refresh();
    }
  }

  void _openTag(String tag) {
    if (tag == widget.tag) {
      _feed.currentState?.scrollToTop();
      return;
    }
    openTagPosts(
      context,
      token: widget.token,
      viewerId: widget.viewerId,
      tag: tag,
      onPostChanged: _changed,
    );
  }

  @override
  Widget build(BuildContext context) {
    return AppScreen(
      title: '#${widget.tag}',
      showServerStatus: false,
      padded: false,
      child: FeedView(
        key: _feed,
        token: widget.token,
        tag: widget.tag,
        onOpenPost: _open,
        onNewPost: () {},
        onOpenTag: _openTag,
        onOpenAuthor: (author) => openUserProfile(
          context,
          token: widget.token,
          viewerId: widget.viewerId,
          userId: author.id,
          onPostChanged: _changed,
        ),
      ),
    );
  }
}
