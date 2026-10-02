// Лента: specs/004-feed.md, вкладки «Все» и «Подписки» — specs/012-follows.md.
//
// Вид «Сад» (2a): пост без карточки — автор, фото 4:5, подпись, действия.
// Посты въезжают снизу по очереди, фото листаются свайпом, двойное касание
// ставит отметку с большим сердцем, касание раскрывает фото в пост.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import 'author_line.dart';
import 'bottom_bar.dart';
import 'comment_icon.dart';
import 'empty_view.dart';
import 'error_view.dart';
import 'like_button.dart';
import 'post_action.dart';
import 'post_groups_line.dart';
import 'visibility_picker.dart' show audienceOf;
import 'loading_view.dart';
import 'place_field.dart';
import 'segment_tabs.dart';
import 'tag_field.dart';

const feedPageSize = 20;
const _loadAheadPixels = 600.0;

/// Тег перехода «фото из ленты → пост».
String postHeroTag(Post post) => 'post-photo-${post.id}';

/// Сколько постов после обновления въезжают по очереди; дальше — без
/// задержки, чтобы низ ленты не ждал.
const _staggered = 6;

/// Сколько после обновления ленты новые карточки ещё въезжают. Потом
/// прокрутка показывает посты сразу, без анимации.
const _entranceWindow = Duration(milliseconds: 1200);

enum FeedScope { all, following }

class FeedTabs extends StatefulWidget {
  const FeedTabs({
    super.key,
    required this.token,
    required this.onOpenPost,
    required this.onNewPost,
    this.onOpenAuthor,
    this.onOpenTag,
    this.onOpenGroup,
    this.onOpenGroups,
    this.onRefreshed,
  });

  final String token;
  final void Function(Post post) onOpenPost;
  final void Function(Author author)? onOpenAuthor;
  final void Function(String tag)? onOpenTag;
  final void Function(GroupBrief group)? onOpenGroup;

  /// Значок «Группы» справа от вкладок (specs/029-groups.md, требование 32).
  final VoidCallback? onOpenGroups;
  final VoidCallback onNewPost;
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

  Future<void> refresh() async {
    await Future.wait(_opened.map((view) => view.refresh()));
  }

  Future<void> scrollToTop() async =>
      _views[_scope]!.currentState?.scrollToTop();

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
    _views[scope]!.currentState?.replayEntrance();
  }

  // Один и тот же пост может быть в обеих вкладках: переход «фото → пост»
  // включён только у открытой, иначе у героя два двойника.
  Widget _view(FeedScope scope) => HeroMode(
    enabled: scope == _scope,
    child: FeedView(
      key: _views[scope],
      token: widget.token,
      scope: scope,
      onOpenPost: widget.onOpenPost,
      onNewPost: widget.onNewPost,
      onOpenAuthor: widget.onOpenAuthor,
      onOpenTag: widget.onOpenTag,
      onOpenGroup: widget.onOpenGroup,
      onShowAll: () => _select(FeedScope.all),
      onRefreshed: widget.onRefreshed,
    ),
  );

  @override
  Widget build(BuildContext context) {
    final onOpenGroups = widget.onOpenGroups;
    final tabs = SegmentTabs(
      labels: const ['Все', 'Подписки'],
      selected: _scope.index,
      onSelect: (index) => _select(FeedScope.values[index]),
    );
    return Column(
      children: [
        if (onOpenGroups == null)
          tabs
        else
          Padding(
            padding: const EdgeInsets.only(right: AppGap.tiny),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: SegmentTabs(
                    labels: tabs.labels,
                    selected: tabs.selected,
                    onSelect: tabs.onSelect,
                    margin: const EdgeInsets.fromLTRB(
                      AppGap.medium,
                      0,
                      AppGap.tiny,
                      AppGap.small,
                    ),
                  ),
                ),
                IconButton(
                  tooltip: 'Группы',
                  onPressed: onOpenGroups,
                  icon: const Icon(Icons.groups_outlined),
                ),
              ],
            ),
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
    this.onOpenTag,
    this.onOpenGroup,
    this.scope = FeedScope.all,
    this.tag,
    this.groupId,
    this.header,
    this.onShowAll,
    this.onRefreshed,
  });

  final String token;
  final FeedScope scope;

  /// Только посты с этим тэгом (specs/028-post-tags.md, требование 24).
  final String? tag;

  /// Лента группы (specs/030-group-posts.md, требование 18).
  final String? groupId;

  /// То, что прокручивается над постами, — шапка экрана группы.
  final Widget? header;
  final void Function(String tag)? onOpenTag;
  final void Function(GroupBrief group)? onOpenGroup;
  final VoidCallback? onShowAll;
  final VoidCallback? onRefreshed;
  final void Function(Post post) onOpenPost;
  final void Function(Author author)? onOpenAuthor;
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

  /// Когда лента показана заново: от этого момента карточки въезжают.
  DateTime _shownAt = DateTime.now();

  /// Поколение показа: новое — карточки строятся заново и въезжают.
  int _generation = 0;

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  /// Страница ленты: лента группы — своя ручка, остальное — общая лента.
  Future<Feed?> _page({String? cursor}) {
    final groupId = widget.groupId;
    if (groupId != null) {
      return GroupsApi(apiClient(token: widget.token))
          .getGroupPosts(groupId, limit: feedPageSize, cursor: cursor);
    }
    return _api.getFeed(
      scope: widget.scope.name,
      limit: feedPageSize,
      cursor: cursor,
      tag: widget.tag,
    );
  }

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

  Future<void> refresh() => _refresh();

  Future<void> scrollToTop() => scrollBackToTop(_scroll);

  /// Вкладку открыли снова — посты въезжают, как на макете.
  void replayEntrance() {
    if (!mounted || _posts.isEmpty) {
      return;
    }
    setState(() {
      _shownAt = DateTime.now();
      _generation++;
    });
  }

  void replace(Post post) {
    final at = _posts.indexWhere((item) => item.id == post.id);
    if (at < 0) {
      return;
    }
    setState(() => _posts[at] = post);
  }

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
      final page = await _page();
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
        _shownAt = DateTime.now();
        _generation++;
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
      final page = await _page(cursor: cursor);
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

    final header = widget.header;

    if (_loading) {
      return _underHeader(const LoadingView(label: 'Открываю ленту…'));
    }
    if (error != null && _posts.isEmpty) {
      return _underHeader(ErrorView(message: error, onRetry: _refresh));
    }
    if (_posts.isEmpty) {
      return RefreshIndicator(
        onRefresh: _pulled,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          children: [
            ?header,
            SizedBox(
              height:
                  MediaQuery.sizeOf(context).height *
                  (header == null ? 0.6 : 0.4),
              child: widget.groupId != null
                  ? const EmptyView(
                      icon: Icons.groups_outlined,
                      title: 'В группе пока нет постов',
                    )
                  : widget.tag != null
                  ? EmptyView(
                      icon: Icons.tag,
                      title: 'Постов с тэгом #${widget.tag} пока нет',
                    )
                  : widget.scope == FeedScope.following
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

    // Шапка — во всю ширину, посты — с полями.
    final skip = header == null ? 0 : 1;
    return RefreshIndicator(
      onRefresh: _pulled,
      child: ListView.separated(
        controller: _scroll,
        padding: EdgeInsets.fromLTRB(
          header == null ? AppGap.medium : 0,
          header == null ? AppGap.tiny : 0,
          header == null ? AppGap.medium : 0,
          AppGap.large,
        ),
        itemCount: _posts.length + 1 + skip,
        separatorBuilder: (_, index) =>
            SizedBox(height: index < skip ? AppGap.tiny : AppGap.loose),
        itemBuilder: (context, at) {
          if (header != null && at == 0) {
            return header;
          }
          final index = at - skip;
          if (index == _posts.length) {
            return _inset(_footer());
          }
          final post = _posts[index];
          final fresh = DateTime.now().difference(_shownAt) < _entranceWindow;
          return _inset(
            Entrance(
              key: ValueKey('${post.id}-$_generation'),
              animate: fresh,
              delay: fresh && index < _staggered
                  ? AppMotion.stagger * index
                  : Duration.zero,
              child: FeedPostCard(
                key: ValueKey(post.id),
                post: post,
                token: widget.token,
                heroTag: postHeroTag(post),
                onTap: () => widget.onOpenPost(post),
                onChanged: replace,
                onOpenAuthor: widget.onOpenAuthor,
                onOpenTag: widget.onOpenTag,
                onOpenGroup: widget.onOpenGroup,
              ),
            ),
          );
        },
      ),
    );
  }

  /// Поля у постов, когда над ними шапка во всю ширину.
  Widget _inset(Widget child) => widget.header == null
      ? child
      : Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
          child: child,
        );

  /// Ожидание и ошибка — под шапкой, если она есть: шапка экрана группы
  /// видна, пока лента грузится.
  Widget _underHeader(Widget state) {
    final header = widget.header;
    if (header == null) {
      return state;
    }
    return RefreshIndicator(
      onRefresh: _pulled,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: [
          header,
          SizedBox(
            height: MediaQuery.sizeOf(context).height * 0.4,
            child: state,
          ),
        ],
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

/// Въезд снизу с проявлением: сдвиг 28, масштаб .97 → 1.
class Entrance extends StatefulWidget {
  const Entrance({
    super.key,
    required this.child,
    this.animate = true,
    this.delay = Duration.zero,
  });

  final Widget child;
  final bool animate;
  final Duration delay;

  @override
  State<Entrance> createState() => _EntranceState();
}

class _EntranceState extends State<Entrance>
    with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(
    vsync: this,
    duration: AppMotion.entrance,
    value: widget.animate ? 0 : 1,
  );
  Timer? _start;

  @override
  void initState() {
    super.initState();
    if (widget.animate) {
      _start = Timer(widget.delay, () {
        if (mounted) {
          _c.forward();
        }
      });
    }
  }

  @override
  void dispose() {
    _start?.cancel();
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final t = CurvedAnimation(parent: _c, curve: AppMotion.ease);
    return AnimatedBuilder(
      animation: t,
      child: widget.child,
      builder: (context, child) => Opacity(
        opacity: t.value.clamp(0.0, 1.0),
        child: Transform.translate(
          offset: Offset(0, 28 * (1 - t.value)),
          child: Transform.scale(scale: 0.97 + 0.03 * t.value, child: child),
        ),
      ),
    );
  }
}

/// Пост в ленте: автор, фото каруселью, начало подписи и действия.
class FeedPostCard extends StatefulWidget {
  const FeedPostCard({
    super.key,
    required this.post,
    required this.token,
    required this.onTap,
    required this.onChanged,
    this.onOpenAuthor,
    this.onOpenTag,
    this.onOpenGroup,
    this.showAuthor = true,
    this.heroTag,
  });

  final Post post;
  final String token;
  final VoidCallback onTap;
  final void Function(Post post) onChanged;
  final void Function(Author author)? onOpenAuthor;

  /// Касание тэга (specs/028-post-tags.md, требование 23).
  final void Function(String tag)? onOpenTag;

  /// Касание группы поста (specs/030-group-posts.md, требование 17).
  final void Function(GroupBrief group)? onOpenGroup;
  final bool showAuthor;

  /// Тег перехода «фото → пост»; без него фото не летит.
  final Object? heroTag;

  static const captionLines = 3;

  @override
  State<FeedPostCard> createState() => _FeedPostCardState();
}

class _FeedPostCardState extends State<FeedPostCard> {
  final _like = GlobalKey<LikeButtonState>();
  final _heart = GlobalKey<BigHeartState>();

  void _doubleTap() {
    _heart.currentState?.play();
    _like.currentState?.likeByDoubleTap();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final post = widget.post;

    Widget photos = FeedPhotos(media: post.media);
    final tag = widget.heroTag;
    if (tag != null) {
      photos = Hero(tag: tag, child: photos);
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.showAuthor)
          Padding(
            padding: const EdgeInsets.only(bottom: AppGap.snug),
            child: AuthorLine(
              author: post.author,
              when: post.createdAt,
              audience: audienceOf(post),
              edited: post.editedAt != null,
              onTap: widget.onOpenAuthor == null
                  ? null
                  : () => widget.onOpenAuthor!(post.author),
            ),
          )
        else
          Padding(
            padding: const EdgeInsets.only(bottom: AppGap.small),
            child: PostedLine(
              when: post.createdAt,
              audience: audienceOf(post),
              edited: post.editedAt != null,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        if (post.groups.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: AppGap.tiny),
            child: PostGroupsLine(
              groups: post.groups,
              onOpen: widget.onOpenGroup,
            ),
          ),
        if (post.place case final place?)
          Padding(
            padding: const EdgeInsets.only(bottom: AppGap.snug),
            child: PostPlaceLine(place: place, distanceKm: post.distanceKm),
          ),
        if (post.media.isNotEmpty)
          GestureDetector(
            onTap: widget.onTap,
            onDoubleTap: _doubleTap,
            child: Stack(
              children: [
                photos,
                Positioned.fill(child: BigHeart(key: _heart)),
              ],
            ),
          ),
        if (post.caption.isNotEmpty)
          InkWell(
            onTap: widget.onTap,
            child: Padding(
              padding: const EdgeInsets.only(top: AppGap.snug),
              child: Text(
                post.caption,
                style: theme.textTheme.bodyLarge,
                maxLines: FeedPostCard.captionLines,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ),
        if (post.tags.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: AppGap.tiny),
            child: PostTagsLine(tags: post.tags, onOpen: widget.onOpenTag),
          ),
        Padding(
          padding: const EdgeInsets.only(top: AppGap.tiny),
          child: Row(
            children: [
              LikeButton(
                key: _like,
                post: post,
                token: widget.token,
                onChanged: widget.onChanged,
              ),
              PostAction(
                iconWidget: CommentIcon(color: theme.colorScheme.secondary),
                count: post.comments,
                tooltip: 'Комментарии',
                onPressed: widget.onTap,
                color: theme.colorScheme.secondary,
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// Фото поста: карусель 4:5 со свайпом. Точки внизу показывают, какая
/// открыта (открытая — вытянутая «таблетка»), отметка «2/7» появляется
/// после свайпа и гаснет (specs/004-feed.md, требование 9).
class FeedPhotos extends StatefulWidget {
  const FeedPhotos({
    super.key,
    required this.media,
    this.radius = AppShape.photo,
    this.aspectRatio = 4 / 5,
  });

  final List<Media> media;

  /// Скругление: в ленте 6, в посте фото во всю ширину — 0.
  final double radius;

  /// Рамка единая для всех постов (макет «Сад»): лента не прыгает по
  /// высоте, место занято до загрузки (требование 10).
  final double aspectRatio;

  static const countShown = Duration(milliseconds: 1500);
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
    final scheme = Theme.of(context).colorScheme;

    return ClipRRect(
      borderRadius: BorderRadius.circular(widget.radius),
      child: AspectRatio(
        aspectRatio: widget.aspectRatio,
        child: ColoredBox(
          color: scheme.surfaceContainerHighest,
          child: media.length == 1
              ? _photo(media.first)
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
                      top: AppGap.snug,
                      right: AppGap.snug,
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
                    Positioned(
                      left: 0,
                      right: 0,
                      bottom: AppGap.snug,
                      child: IgnorePointer(
                        child: _PhotoDots(current: _page, count: media.length),
                      ),
                    ),
                  ],
                ),
        ),
      ),
    );
  }

  Widget _photo(Media photo) => Image.network(
    mediaUrl(photo.url),
    fit: BoxFit.cover,
    // Проявление при загрузке вместо резкого появления.
    frameBuilder: (context, child, frame, sync) => sync
        ? child
        : AnimatedOpacity(
            opacity: frame == null ? 0 : 1,
            duration: AppMotion.standard,
            child: child,
          ),
  );
}

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
        borderRadius: BorderRadius.circular(AppShape.pill),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppGap.small + AppGap.tiny,
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

/// Точки: открытая — «таблетка» 18×6, остальные — кружки 6×6.
class _PhotoDots extends StatelessWidget {
  const _PhotoDots({required this.current, required this.count});

  final int current;
  final int count;

  static const _dot = 6.0;
  static const _active = 18.0;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        for (var i = 0; i < count; i++)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 2.5),
            child: AnimatedContainer(
              duration: AppMotion.standard,
              curve: AppMotion.ease,
              width: i == current ? _active : _dot,
              height: _dot,
              decoration: BoxDecoration(
                // Белое с тенью видно на любом снимке.
                color: i == current ? Colors.white : Colors.white70,
                borderRadius: BorderRadius.circular(AppShape.pill),
                boxShadow: const [
                  BoxShadow(color: Colors.black26, blurRadius: 4),
                ],
              ),
            ),
          ),
      ],
    );
  }
}
