// «Сохранённые»: specs/032-bookmarks.md, требования 14–16.
//
// Открывается значком закладки в шапке своего профиля. Это та же лента,
// только свои закладки, новые сверху. Убранный пост остаётся на экране
// до обновления: промах исправляется вторым касанием (требование 15).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/feed_view.dart';
import 'group_screen.dart';
import 'post_screen.dart';
import 'tag_posts_screen.dart';
import 'user_screen.dart';

class BookmarksScreen extends StatefulWidget {
  const BookmarksScreen({
    super.key,
    required this.token,
    required this.viewerId,
    this.onPostChanged,
  });

  final String token;
  final String viewerId;
  final void Function(Post post)? onPostChanged;

  @override
  State<BookmarksScreen> createState() => _BookmarksScreenState();
}

class _BookmarksScreenState extends State<BookmarksScreen> {
  final _feed = GlobalKey<FeedViewState>();

  @override
  void initState() {
    super.initState();
    usage.screen('bookmarks');
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

  @override
  Widget build(BuildContext context) {
    return AppScreen(
      title: 'Сохранённые',
      showServerStatus: false,
      padded: false,
      child: FeedView(
        key: _feed,
        token: widget.token,
        bookmarks: true,
        onOpenPost: _open,
        onNewPost: () {},
        onOpenTag: (tag) => openTagPosts(
          context,
          token: widget.token,
          viewerId: widget.viewerId,
          tag: tag,
          onPostChanged: _changed,
        ),
        onOpenGroup: (group) => openGroup(
          context,
          token: widget.token,
          viewerId: widget.viewerId,
          groupId: group.id,
        ),
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
