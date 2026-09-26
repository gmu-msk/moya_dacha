// Посты одного человека подряд: specs/009-user-profile.md, требование 11.
//
// Открывается касанием клетки в профиле сразу на выбранном посте. Выше
// него — более новые, ниже — более старые; как в Инстаграме, листать
// можно в обе стороны, не возвращаясь к сетке.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';
import '../widgets/feed_view.dart';
import 'post_screen.dart';
import 'user_screen.dart';

class UserPostsScreen extends StatefulWidget {
  const UserPostsScreen({
    super.key,
    required this.posts,
    required this.startAt,
    required this.title,
    required this.token,
    required this.viewerId,
    this.onPostChanged,
    this.onPostDeleted,
  });

  /// Посты, общие с сеткой профиля.
  final UserPostsPager posts;

  /// Какой пост показать первым: его коснулись в сетке.
  final int startAt;

  /// Чьи это посты — имя в заголовке. Строки автора в карточках нет.
  final String title;

  final String token;
  final String viewerId;
  final void Function(Post post)? onPostChanged;
  final VoidCallback? onPostDeleted;

  @override
  State<UserPostsScreen> createState() => _UserPostsScreenState();
}

class _UserPostsScreenState extends State<UserPostsScreen> {
  final ScrollController _scroll = ScrollController();

  /// Граница между «выше» и «ниже». Список строится от неё в обе стороны,
  /// поэтому выбранный пост сразу наверху экрана, а не где-то в середине
  /// прокрученного списка.
  final Key _center = UniqueKey();

  late int _start = widget.startAt;
  late String? _startId = _idAt(widget.startAt);

  UserPostsPager get _posts => widget.posts;

  @override
  void initState() {
    super.initState();
    usage.screen('user_posts');
    _scroll.addListener(_onScroll);
    _posts.addListener(_onPosts);
  }

  @override
  void dispose() {
    _posts.removeListener(_onPosts);
    _scroll.dispose();
    super.dispose();
  }

  String? _idAt(int index) =>
      index >= 0 && index < _posts.posts.length ? _posts.posts[index].id : null;

  /// Граница едет за постом, а не за номером: удалённый выше пост
  /// не должен сдвигать то, что человек видит.
  void _onPosts() {
    final at = _posts.posts.indexWhere((post) => post.id == _startId);
    setState(() {
      if (at >= 0) {
        _start = at;
      } else {
        _start = _start.clamp(0, _posts.posts.length);
        _startId = _idAt(_start);
      }
    });
  }

  void _onScroll() {
    if (!_scroll.hasClients) {
      return;
    }
    final left = _scroll.position.maxScrollExtent - _scroll.position.pixels;
    if (left < profileLoadAheadPixels) {
      _posts.loadMore();
    }
  }

  Future<void> _open(Post post) async {
    final deleted = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => PostScreen(
          post: post,
          token: widget.token,
          viewerId: widget.viewerId,
          onChanged: _changed,
        ),
      ),
    );
    if (deleted == true) {
      _posts.remove(post.id);
      widget.onPostDeleted?.call();
    }
  }

  void _changed(Post post) {
    _posts.replace(post);
    widget.onPostChanged?.call(post);
  }

  Widget _card(Post post) => Padding(
    padding: const EdgeInsets.only(bottom: AppGap.large),
    child: FeedPostCard(
      key: ValueKey(post.id),
      post: post,
      token: widget.token,
      showAuthor: false,
      onTap: () => _open(post),
      onChanged: _changed,
    ),
  );

  @override
  Widget build(BuildContext context) {
    final posts = _posts.posts;
    final start = _start;
    final nextPageError = _posts.nextPageError;

    return AppScreen(
      title: widget.title,
      showServerStatus: false,
      padded: false,
      child: CustomScrollView(
        controller: _scroll,
        center: _center,
        slivers: [
          // Более новые посты — над выбранным. Список до границы растёт
          // вверх: первый его элемент — ближайший к выбранному посту.
          SliverPadding(
            padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
            sliver: SliverList.builder(
              itemCount: start,
              itemBuilder: (context, index) => _card(posts[start - 1 - index]),
            ),
          ),
          SliverPadding(
            key: _center,
            padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
            sliver: SliverList.builder(
              itemCount: posts.length - start,
              itemBuilder: (context, index) => _card(posts[start + index]),
            ),
          ),
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.all(AppGap.large),
              child: nextPageError != null
                  ? ErrorView(message: nextPageError, onRetry: _posts.loadMore)
                  : _posts.loadingMore
                  ? const Center(child: CircularProgressIndicator())
                  : const SizedBox.shrink(),
            ),
          ),
        ],
      ),
    );
  }
}
