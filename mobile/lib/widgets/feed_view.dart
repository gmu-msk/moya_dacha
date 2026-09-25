// Лента: specs/004-feed.md, вкладки «Все» и «Подписки» — specs/012-follows.md.
//
// Страницы подгружаются по мере прокрутки, жест вниз обновляет ленту
// целиком. Ошибка следующей страницы не стирает то, что уже показано:
// на дачной связи это обычное дело, и терять из-за неё пролистанное
// нельзя (specs/004-feed.md, требование 13).
import 'dart:async';

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
import 'segment_tabs.dart';

/// Сколько постов запрашивается за раз. Столько же сервис отдаёт
/// по умолчанию (specs/004-feed.md, требование 4).
const feedPageSize = 20;

/// За сколько пикселей до конца списка запрашивается следующая страница:
/// примерно экран, чтобы к моменту, когда человек долистает, она уже была.
const _loadAheadPixels = 600.0;

/// Вкладки ленты. Имя — значение параметра `scope` в запросе.
enum FeedScope { all, following }

/// Лента с вкладками «Все» (слева, открывается первой) и «Подписки»
/// (specs/012-follows.md, требования 15 и 19). Обе вкладки живут, пока
/// открыт главный экран: переключение не теряет пролистанное, а вкладка
/// «Подписки» строится при первом касании.
class FeedTabs extends StatefulWidget {
  const FeedTabs({
    super.key,
    required this.token,
    required this.onOpenPost,
    required this.onNewPost,
    this.onOpenAuthor,
    this.onRefreshed,
  });

  final String token;
  final void Function(Post post) onOpenPost;
  final void Function(Author author)? onOpenAuthor;
  final VoidCallback onNewPost;

  /// Ленту потянули вниз: заодно узнать, нет ли нового в уведомлениях
  /// (specs/014-notifications.md, требование 6).
  final VoidCallback? onRefreshed;

  @override
  State<FeedTabs> createState() => FeedTabsState();
}

class FeedTabsState extends State<FeedTabs> {
  final Map<FeedScope, GlobalKey<FeedViewState>> _views = {
    for (final scope in FeedScope.values) scope: GlobalKey<FeedViewState>(),
  };

  FeedScope _scope = FeedScope.all;
  bool _followingOpened = false;

  Iterable<FeedViewState> get _opened =>
      _views.values.map((key) => key.currentState).whereType<FeedViewState>();

  /// Обе вкладки заново: свой пост выложен или удалён, на кого-то
  /// подписались или отписались.
  Future<void> refresh() async {
    await Future.wait(_opened.map((view) => view.refresh()));
  }

  /// Открытую вкладку — наверх (требование 19).
  Future<void> scrollToTop() async =>
      _views[_scope]!.currentState?.scrollToTop();

  /// Пост изменился — в обеих вкладках, где он есть.
  void replace(Post post) {
    for (final view in _opened) {
      view.replace(post);
    }
  }

  void _select(FeedScope scope) {
    if (scope == _scope) {
      scrollToTop();
      return;
    }
    debugPrint('$logMarker feed=tab scope=${scope.name}');
    setState(() {
      _scope = scope;
      _followingOpened = _followingOpened || scope == FeedScope.following;
    });
  }

  FeedView _view(FeedScope scope) => FeedView(
    key: _views[scope],
    token: widget.token,
    scope: scope,
    onOpenPost: widget.onOpenPost,
    onNewPost: widget.onNewPost,
    onOpenAuthor: widget.onOpenAuthor,
    onShowAll: () => _select(FeedScope.all),
    onRefreshed: widget.onRefreshed,
  );

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        SegmentTabs(
          labels: const ['Все', 'Подписки'],
          selected: _scope.index,
          onSelect: (index) => _select(FeedScope.values[index]),
        ),
        Expanded(
          child: IndexedStack(
            index: _scope.index,
            children: [
              _view(FeedScope.all),
              if (_followingOpened)
                _view(FeedScope.following)
              else
                const SizedBox.shrink(),
            ],
          ),
        ),
      ],
    );
  }
}

class FeedView extends StatefulWidget {
  const FeedView({
    super.key,
    required this.token,
    required this.onOpenPost,
    required this.onNewPost,
    this.onOpenAuthor,
    this.scope = FeedScope.all,
    this.onShowAll,
    this.onRefreshed,
  });

  final String token;

  /// Какая это вкладка ленты.
  final FeedScope scope;

  /// Из пустых «Подписок» — во «Все» (specs/012-follows.md, требование 2
  /// сценария).
  final VoidCallback? onShowAll;

  /// Ленту потянули вниз.
  final VoidCallback? onRefreshed;

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

  /// Потянули вниз — лента заново, и заодно проверка уведомлений.
  Future<void> _pulled() {
    widget.onRefreshed?.call();
    return _refresh();
  }

  Future<void> _refresh() async {
    setState(() {
      _error = null;
      _nextPageError = null;
      _loading = _posts.isEmpty;
    });

    try {
      final page = await _api.getFeed(
        scope: widget.scope.name,
        limit: feedPageSize,
      );
      debugPrint(
        '$logMarker feed=loaded scope=${widget.scope.name} '
        'posts=${page?.items.length}',
      );
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
      final page = await _api.getFeed(
        scope: widget.scope.name,
        limit: feedPageSize,
        cursor: cursor,
      );
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
        onRefresh: _pulled,
        // Пустое состояние тоже должно тянуться вниз, иначе обновить
        // ленту, пока в ней пусто, нечем.
        child: ListView(
          children: [
            SizedBox(
              height: MediaQuery.sizeOf(context).height * 0.6,
              child: widget.scope == FeedScope.following
                  // Новичок ни на кого не подписан (specs/012-follows.md,
                  // «Тексты»).
                  ? EmptyView(
                      icon: Icons.people_outline,
                      title: 'Вы пока ни на кого не подписаны',
                      hint:
                          'Во вкладке «Все» посты всех дачников. Понравится '
                          'чей-то огород, подпишитесь в профиле, и его посты '
                          'будут здесь.',
                      action: FilledButton(
                        onPressed: widget.onShowAll,
                        child: const Text('Смотреть все посты'),
                      ),
                    )
                  : EmptyView(
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
      onRefresh: _pulled,
      child: ListView.separated(
        controller: _scroll,
        padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
        itemCount: _posts.length + 1,
        separatorBuilder: (_, _) => const SizedBox(height: AppGap.snug),
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

/// Пост в ленте: автор, фотографии каруселью и начало подписи. Подпись
/// целиком и комментарии — на экране поста (specs/004-feed.md,
/// требования 9 и 11).
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

    // Пост — карточка на полотне: белая подложка, тонкий кант и
    // скругление приходят из темы, экран их не повторяет (ADR-0012).
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(AppGap.snug),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (showAuthor)
                AuthorLine(
                  author: post.author,
                  when: post.createdAt,
                  visibility: post.visibility,
                  onTap: onOpenAuthor == null
                      ? null
                      : () => onOpenAuthor!(post.author),
                )
              else
                Padding(
                  padding: const EdgeInsets.only(bottom: AppGap.small),
                  child: PostedLine(
                    when: post.createdAt,
                    visibility: post.visibility,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              if (post.media.isNotEmpty) FeedPhotos(media: post.media),
              if (post.caption.isNotEmpty) ...[
                const SizedBox(height: AppGap.tiny),
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

/// Фотографии поста в ленте. Если их несколько, они листаются свайпом
/// прямо здесь; точки внизу показывают, какая открыта, а отметка «2/7»
/// появляется после свайпа и плавно гаснет (specs/004-feed.md,
/// требование 9). Следующая фотография не грузится, пока до неё не
/// долистали: `PageView` строит страницы только по мере надобности.
class FeedPhotos extends StatefulWidget {
  const FeedPhotos({super.key, required this.media});

  final List<Media> media;

  /// Сколько видна отметка «2/7» после свайпа.
  static const countShown = Duration(milliseconds: 1500);

  /// За сколько она гаснет и появляется.
  static const countFade = Duration(milliseconds: 400);

  @override
  State<FeedPhotos> createState() => _FeedPhotosState();
}

class _FeedPhotosState extends State<FeedPhotos> {
  final _pages = PageController();
  Timer? _hide;
  int _page = 0;
  bool _countVisible = true;

  @override
  void initState() {
    super.initState();
    _scheduleHide();
  }

  @override
  void dispose() {
    _hide?.cancel();
    _pages.dispose();
    super.dispose();
  }

  void _scheduleHide() {
    _hide?.cancel();
    _hide = Timer(FeedPhotos.countShown, () {
      if (mounted) {
        setState(() => _countVisible = false);
      }
    });
  }

  void _turned(int page) {
    setState(() {
      _page = page;
      _countVisible = true;
    });
    _scheduleHide();
  }

  @override
  Widget build(BuildContext context) {
    final media = widget.media;
    final first = media.first;

    return ClipRRect(
      borderRadius: BorderRadius.circular(AppShape.photo),
      child: AspectRatio(
        // Размеры приходят вместе с постом, поэтому место под фотографию
        // занято до её загрузки и лента не дёргается (specs/004-feed.md,
        // требование 10). Рамку задаёт первая фотография, остальные
        // вписываются в неё обрезкой — иначе карточка прыгала бы по
        // высоте на каждом свайпе.
        aspectRatio: first.height == 0 ? 1 : first.width / first.height,
        child: media.length == 1
            ? _photo(first)
            : Stack(
                fit: StackFit.expand,
                children: [
                  PageView.builder(
                    controller: _pages,
                    itemCount: media.length,
                    onPageChanged: _turned,
                    itemBuilder: (_, index) => _photo(media[index]),
                  ),
                  Positioned(
                    top: AppGap.small,
                    right: AppGap.small,
                    child: IgnorePointer(
                      child: AnimatedOpacity(
                        opacity: _countVisible ? 1 : 0,
                        duration: FeedPhotos.countFade,
                        child: _PhotoCount(
                          current: _page + 1,
                          count: media.length,
                        ),
                      ),
                    ),
                  ),
                  // Точки остаются всегда: по ним видно, что фотография
                  // не одна, и когда отметка погасла.
                  Positioned(
                    left: 0,
                    right: 0,
                    bottom: AppGap.small,
                    child: IgnorePointer(
                      child: _PhotoDots(current: _page, count: media.length),
                    ),
                  ),
                ],
              ),
      ),
    );
  }

  Widget _photo(Media photo) =>
      Image.network(mediaUrl(photo.url), fit: BoxFit.cover);
}

/// Отметка «2/7» на фотографии поста.
class _PhotoCount extends StatelessWidget {
  const _PhotoCount({required this.current, required this.count});

  final int current;
  final int count;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return DecoratedBox(
      decoration: BoxDecoration(
        color: Colors.black54,
        borderRadius: BorderRadius.circular(AppShape.small),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppGap.small,
          vertical: AppGap.tiny,
        ),
        child: Text(
          '$current/$count',
          style: theme.textTheme.labelLarge?.copyWith(color: Colors.white),
        ),
      ),
    );
  }
}

/// Точки под фотографией: сколько их в посте и какая открыта.
class _PhotoDots extends StatelessWidget {
  const _PhotoDots({required this.current, required this.count});

  final int current;
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
                color: i == current ? Colors.white : Colors.white54,
                shape: BoxShape.circle,
              ),
              child: const SizedBox.square(dimension: AppGap.small),
            ),
          ),
      ],
    );
  }
}
