// Лента: specs/004-feed.md.
//
// Страницы подгружаются по мере прокрутки, жест вниз обновляет ленту
// целиком. Ошибка следующей страницы не стирает то, что уже показано:
// на дачной связи это обычное дело, и терять из-за неё пролистанное
// нельзя (specs/004-feed.md, требование 13).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import 'author_line.dart';
import 'bottom_bar.dart';
import 'empty_view.dart';
import 'error_view.dart';
import 'like_button.dart';
import 'post_action.dart';
import 'loading_view.dart';

/// Сколько постов запрашивается за раз. Столько же сервис отдаёт
/// по умолчанию (specs/004-feed.md, требование 4).
const feedPageSize = 20;

/// За сколько пикселей до конца списка запрашивается следующая страница:
/// примерно экран, чтобы к моменту, когда человек долистает, она уже была.
const _loadAheadPixels = 600.0;

class FeedView extends StatefulWidget {
  const FeedView({
    super.key,
    required this.token,
    required this.onOpenPost,
    required this.onNewPost,
    this.onOpenAuthor,
  });

  final String token;

  /// Открыть пост целиком: все фотографии и подпись.
  final void Function(Post post) onOpenPost;

  /// Открыть профиль автора поста (specs/009-user-profile.md).
  final void Function(Author author)? onOpenAuthor;

  /// Выложить первый пост — из пустой ленты.
  final VoidCallback onNewPost;

  @override
  State<FeedView> createState() => FeedViewState();
}

class FeedViewState extends State<FeedView> {
  final ScrollController _scroll = ScrollController();
  final List<Post> _posts = [];

  String? _cursor;
  String? _error;
  String? _nextPageError;
  bool _loading = true;
  bool _loadingMore = false;

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
    _refresh();
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (!_scroll.hasClients || _cursor == null || _loadingMore) {
      return;
    }
    final left = _scroll.position.maxScrollExtent - _scroll.position.pixels;
    if (left < _loadAheadPixels) {
      _loadMore();
    }
  }

  /// Обновление ленты: первая страница запрашивается заново и показывается
  /// вместо накопленного (specs/004-feed.md, требование 12).
  Future<void> refresh() => _refresh();

  /// К самому верху ленты: повторное касание «Ленты» в нижней панели
  /// (specs/011-bottom-bar.md, требование 4).
  Future<void> scrollToTop() => scrollBackToTop(_scroll);

  /// Показать пост заново: его лайкнули здесь или на экране поста,
  /// и в ленте должно быть то же число (specs/005-likes.md).
  void replace(Post post) {
    final at = _posts.indexWhere((item) => item.id == post.id);
    if (at < 0) {
      return;
    }
    setState(() => _posts[at] = post);
  }

  Future<void> _refresh() async {
    setState(() {
      _error = null;
      _nextPageError = null;
      _loading = _posts.isEmpty;
    });

    try {
      final page = await _api.getFeed(limit: feedPageSize);
      debugPrint('$logMarker feed=loaded posts=${page?.items.length}');
      if (!mounted) {
        return;
      }
      setState(() {
        _posts
          ..clear()
          ..addAll(page?.items ?? const []);
        _cursor = page?.nextCursor;
        _loading = false;
      });
    } on Exception catch (error) {
      debugPrint('$logMarker feed=failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _loading = false;
        _error = errorMessage(error);
      });
    }
  }

  Future<void> _loadMore() async {
    final cursor = _cursor;
    if (cursor == null || _loadingMore) {
      return;
    }
    setState(() {
      _loadingMore = true;
      _nextPageError = null;
    });

    try {
      final page = await _api.getFeed(limit: feedPageSize, cursor: cursor);
      debugPrint('$logMarker feed=page posts=${page?.items.length}');
      if (!mounted) {
        return;
      }
      setState(() {
        _posts.addAll(page?.items ?? const []);
        _cursor = page?.nextCursor;
        _loadingMore = false;
      });
    } on Exception catch (error) {
      debugPrint('$logMarker feed=page_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _loadingMore = false;
        _nextPageError = errorMessage(error);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final error = _error;

    if (_loading) {
      return const LoadingView(label: 'Открываю ленту…');
    }
    if (error != null && _posts.isEmpty) {
      return ErrorView(message: error, onRetry: _refresh);
    }
    if (_posts.isEmpty) {
      return RefreshIndicator(
        onRefresh: _refresh,
        // Пустое состояние тоже должно тянуться вниз, иначе обновить
        // ленту, пока в ней пусто, нечем.
        child: ListView(
          children: [
            SizedBox(
              height: MediaQuery.sizeOf(context).height * 0.6,
              child: EmptyView(
                icon: Icons.eco_outlined,
                title: 'Постов пока нет',
                hint: 'Будьте первым: покажите, что у вас выросло.',
                action: FilledButton.icon(
                  onPressed: widget.onNewPost,
                  icon: const Icon(Icons.add_a_photo_outlined),
                  label: const Text('Новый пост'),
                ),
              ),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _refresh,
      child: ListView.separated(
        controller: _scroll,
        padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
        itemCount: _posts.length + 1,
        separatorBuilder: (_, _) => const SizedBox(height: AppGap.large),
        itemBuilder: (context, index) {
          if (index == _posts.length) {
            return _footer();
          }
          final post = _posts[index];
          return FeedPostCard(
            // Состояние карточки должно ехать за постом, а не за местом
            // в списке: иначе после обновления ленты сердечко остаётся
            // от того, кто был здесь раньше.
            key: ValueKey(post.id),
            post: post,
            token: widget.token,
            onTap: () => widget.onOpenPost(post),
            onChanged: replace,
            onOpenAuthor: widget.onOpenAuthor,
          );
        },
      ),
    );
  }

  Widget _footer() {
    final error = _nextPageError;

    if (error != null) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: AppGap.large),
        child: ErrorView(message: error, onRetry: _loadMore),
      );
    }
    if (_loadingMore) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: AppGap.large),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    return const SizedBox(height: AppGap.large);
  }
}

/// Пост в ленте: автор, первая фотография и начало подписи. Остальное —
/// на экране поста (specs/004-feed.md, требования 9 и 11).
class FeedPostCard extends StatelessWidget {
  const FeedPostCard({
    super.key,
    required this.post,
    required this.token,
    required this.onTap,
    required this.onChanged,
    this.onOpenAuthor,
    this.showAuthor = true,
  });

  final Post post;
  final String token;
  final VoidCallback onTap;

  /// Пост изменился: его лайкнули прямо здесь.
  final void Function(Post post) onChanged;

  /// Открыть профиль автора: касанием имени или аватара.
  final void Function(Author author)? onOpenAuthor;

  /// Показывать ли строку автора. В постах одного человека её нет: автор
  /// один и назван в заголовке экрана (specs/009-user-profile.md,
  /// требование 11) — остаётся только время.
  final bool showAuthor;

  /// Сколько строк подписи видно в ленте.
  static const captionLines = 3;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final photo = post.media.isEmpty ? null : post.media.first;

    // Пост — карточка на полотне: белая подложка, тонкий кант и
    // скругление приходят из темы, экран их не повторяет (ADR-0012).
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(AppGap.medium),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (showAuthor)
                AuthorLine(
                  author: post.author,
                  when: post.createdAt,
                  onTap: onOpenAuthor == null
                      ? null
                      : () => onOpenAuthor!(post.author),
                )
              else
                Padding(
                  padding: const EdgeInsets.only(bottom: AppGap.small),
                  child: Text(
                    whenPosted(post.createdAt),
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              if (photo != null)
                ClipRRect(
                  borderRadius: BorderRadius.circular(AppShape.photo),
                  child: Stack(
                    children: [
                      AspectRatio(
                        // Размеры приходят вместе с постом, поэтому место
                        // под фотографию занято до её загрузки и лента
                        // не дёргается (specs/004-feed.md, требование 10).
                        aspectRatio: photo.height == 0
                            ? 1
                            : photo.width / photo.height,
                        child: Image.network(
                          mediaUrl(photo.url),
                          fit: BoxFit.cover,
                        ),
                      ),
                      if (post.media.length > 1) ...[
                        Positioned(
                          top: AppGap.small,
                          right: AppGap.small,
                          child: _PhotoCount(count: post.media.length),
                        ),
                        // Точки по нижнему краю: по ним видно, что
                        // фотография не одна, ещё до того, как прочитано
                        // «1/4».
                        Positioned(
                          left: 0,
                          right: 0,
                          bottom: AppGap.small,
                          child: _PhotoDots(count: post.media.length),
                        ),
                      ],
                    ],
                  ),
                ),
              if (post.caption.isNotEmpty) ...[
                const SizedBox(height: AppGap.small),
                Text(
                  post.caption,
                  style: theme.textTheme.bodyLarge,
                  maxLines: captionLines,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
              Row(
                children: [
                  LikeButton(post: post, token: token, onChanged: onChanged),
                  // Число комментариев: по нему видно, где разговор идёт,
                  // а где ещё нет (specs/006-comments.md, требование 7).
                  // Сам разговор — на экране поста, поэтому кнопка ведёт
                  // туда.
                  PostAction(
                    icon: Icons.mode_comment_outlined,
                    count: post.comments,
                    tooltip: 'Комментарии',
                    onPressed: onTap,
                    // Комментарии — вторая краска темы, отметка — основная:
                    // в ряду под постом их видно порознь.
                    color: theme.colorScheme.secondary,
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Отметка «1/4» на первой фотографии поста.
class _PhotoCount extends StatelessWidget {
  const _PhotoCount({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return DecoratedBox(
      decoration: BoxDecoration(
        color: Colors.black54,
        borderRadius: BorderRadius.circular(AppGap.small),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppGap.small,
          vertical: 4,
        ),
        child: Text(
          '1/$count',
          style: theme.textTheme.labelLarge?.copyWith(color: Colors.white),
        ),
      ),
    );
  }
}

/// Точки под фотографией: сколько их в посте. Первая — та, что видна;
/// пролистать их можно на экране поста (specs/004-feed.md, требование 9).
class _PhotoDots extends StatelessWidget {
  const _PhotoDots({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        for (var i = 0; i < count; i++)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 3),
            child: DecoratedBox(
              decoration: BoxDecoration(
                // Точки лежат на фотографии, а какая она — неизвестно,
                // поэтому цвета темы здесь не годятся: белое на тёмной
                // подложке видно на любом снимке.
                color: i == 0 ? Colors.white : Colors.white54,
                shape: BoxShape.circle,
              ),
              child: const SizedBox.square(dimension: AppGap.small),
            ),
          ),
      ],
    );
  }
}
