// Профиль пользователя: specs/009-user-profile.md.
//
// Кто этот человек и все его посты сеткой по три, как в Инстаграме.
// Касание клетки открывает посты этого человека подряд
// (user_posts_screen.dart). Свой профиль — тот же экран с одной лишней
// кнопкой: человек видит себя так, как его видят соседи.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../app_scope.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/author_line.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/user_avatar.dart';
import 'profile_screen.dart';
import 'user_posts_screen.dart';

/// Сколько постов запрашивается за раз. Кратно трём: страница сетки
/// кончается полным рядом.
const profilePageSize = 30;

/// За сколько пикселей до конца списка запрашивается следующая страница.
const profileLoadAheadPixels = 600.0;

/// Открыть профиль человека по идентификатору — `author.id` поста или
/// комментария (specs/009-user-profile.md, требование 6).
Future<void> openUserProfile(
  BuildContext context, {
  required String token,
  required String viewerId,
  required String userId,
  void Function(Post post)? onPostChanged,
  VoidCallback? onPostDeleted,
  void Function(CurrentUser user)? onProfileEdited,
}) => Navigator.of(context).push<void>(
  MaterialPageRoute(
    builder: (_) => UserScreen(
      token: token,
      viewerId: viewerId,
      userId: userId,
      onPostChanged: onPostChanged,
      onPostDeleted: onPostDeleted,
      onProfileEdited: onProfileEdited,
    ),
  ),
);

class UserScreen extends StatefulWidget {
  const UserScreen({
    super.key,
    required this.token,
    required this.viewerId,
    required this.userId,
    this.onPostChanged,
    this.onPostDeleted,
    this.onProfileEdited,
  });

  final String token;

  /// Кто смотрит: у своего профиля есть «Изменить профиль».
  final String viewerId;

  /// Чей профиль.
  final String userId;

  /// Пост отметили здесь: тому, кто открыл профиль (ленте), нужно
  /// показать то же число (specs/009-user-profile.md, требование 12).
  final void Function(Post post)? onPostChanged;

  /// Свой пост удалён отсюда: в ленте его тоже больше нет.
  final VoidCallback? onPostDeleted;

  /// Свой профиль поправили: имя и аватар в шапке ленты теперь другие.
  final void Function(CurrentUser user)? onProfileEdited;

  @override
  State<UserScreen> createState() => _UserScreenState();
}

class _UserScreenState extends State<UserScreen> {
  final ScrollController _scroll = ScrollController();
  late final UserPostsPager _posts = UserPostsPager(
    token: widget.token,
    userId: widget.userId,
  );

  UserProfile? _profile;
  String? _error;
  bool _editing = false;

  bool get _mine => widget.userId == widget.viewerId;

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
    _posts.addListener(_onPosts);
    _load();
  }

  @override
  void dispose() {
    _posts.removeListener(_onPosts);
    _posts.dispose();
    _scroll.dispose();
    super.dispose();
  }

  void _onPosts() => setState(() {});

  void _onScroll() {
    if (!_scroll.hasClients) {
      return;
    }
    final left = _scroll.position.maxScrollExtent - _scroll.position.pixels;
    if (left < profileLoadAheadPixels) {
      _posts.loadMore();
    }
  }

  /// Кто это и первая страница постов. Потянуть профиль вниз — то же
  /// самое заново (specs/009-user-profile.md, требование 14).
  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final results = await Future.wait<Object?>([
        UsersApi(apiClient(token: widget.token)).getUser(widget.userId),
        _posts.refresh(),
      ]);
      final profile = results.first as UserProfile?;
      debugPrint(
        '$logMarker screen=user id=${widget.userId} '
        'posts=${profile?.posts} mine=$_mine',
      );
      if (!mounted) {
        return;
      }
      setState(() => _profile = profile);
    } on Exception catch (error) {
      debugPrint('$logMarker screen=user error=$error');
      if (!mounted) {
        return;
      }
      setState(() => _error = errorMessage(error));
    }
  }

  /// Только сведения о человеке: после удаления поста у него другое число.
  Future<void> _reloadProfile() async {
    try {
      final profile = await UsersApi(apiClient(token: widget.token))
          .getUser(widget.userId);
      if (mounted && profile != null) {
        setState(() => _profile = profile);
      }
    } on Exception catch (error) {
      // Пост уже удалён и из сетки ушёл: число поправится при обновлении.
      debugPrint('$logMarker screen=user reload_failed error=$error');
    }
  }

  /// Правка своего профиля — экран из specs/002-profile.md. Ему нужен
  /// профиль, каким его видит сам человек, с номером телефона.
  Future<void> _edit() async {
    setState(() => _editing = true);
    try {
      final me = await ProfileApi(apiClient(token: widget.token)).getMe();
      if (!mounted || me == null) {
        return;
      }
      final signOut = AppScope.of(context)?.signOut;
      final edited = await Navigator.of(context).push<CurrentUser>(
        MaterialPageRoute(
          builder: (_) => ProfileScreen(
            token: widget.token,
            user: me,
            onSignedOut: signOut ?? () async {},
          ),
        ),
      );
      if (edited != null) {
        widget.onProfileEdited?.call(edited);
        await _load();
      }
    } on Exception catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(errorMessage(error))));
      }
    } finally {
      if (mounted) {
        setState(() => _editing = false);
      }
    }
  }

  /// Посты подряд, сразу на том, которого коснулись
  /// (specs/009-user-profile.md, требование 11).
  Future<void> _openPosts(int index, UserProfile profile) =>
      Navigator.of(context).push<void>(
        MaterialPageRoute(
          builder: (_) => UserPostsScreen(
            posts: _posts,
            startAt: index,
            title: profile.name,
            token: widget.token,
            viewerId: widget.viewerId,
            onPostChanged: widget.onPostChanged,
            onPostDeleted: () {
              _reloadProfile();
              widget.onPostDeleted?.call();
            },
          ),
        ),
      );

  @override
  Widget build(BuildContext context) {
    final profile = _profile;
    final error = _error;

    final Widget body;
    if (profile != null) {
      body = RefreshIndicator(onRefresh: _load, child: _content(profile));
    } else if (error != null) {
      body = Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: _load),
      );
    } else {
      body = const LoadingView(label: 'Открываю профиль…');
    }

    return AppScreen(
      title: 'Профиль',
      // Профиль смотрят, а не проверяют связь.
      showServerStatus: false,
      padded: false,
      child: body,
    );
  }

  Widget _content(UserProfile profile) {
    final posts = _posts.posts;
    final nextPageError = _posts.nextPageError;

    return CustomScrollView(
      controller: _scroll,
      // Даже короткий профиль тянется вниз: иначе его не обновить.
      physics: const AlwaysScrollableScrollPhysics(),
      slivers: [
        SliverPadding(
          padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
          sliver: SliverToBoxAdapter(
            child: _Header(profile: profile, mine: _mine),
          ),
        ),
        if (_mine)
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(
              AppGap.medium,
              AppGap.medium,
              AppGap.medium,
              0,
            ),
            sliver: SliverToBoxAdapter(
              child: OutlinedButton.icon(
                onPressed: _editing ? null : _edit,
                icon: const Icon(Icons.edit_outlined),
                label: const Text('Изменить профиль'),
              ),
            ),
          ),
        if (posts.isEmpty)
          SliverFillRemaining(
            hasScrollBody: false,
            child: Padding(
              padding: const EdgeInsets.all(AppGap.large),
              child: EmptyView(
                icon: Icons.eco_outlined,
                title: 'Постов пока нет',
                hint: _mine
                    ? 'Выложите первый — он появится здесь и в ленте'
                    : 'Когда ${profile.name} что-нибудь выложит, '
                          'это появится здесь и в ленте',
              ),
            ),
          )
        else
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(
              AppGap.medium,
              AppGap.large,
              AppGap.medium,
              0,
            ),
            sliver: SliverGrid.builder(
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 3,
                mainAxisSpacing: AppGap.tiny,
                crossAxisSpacing: AppGap.tiny,
              ),
              itemCount: posts.length,
              itemBuilder: (context, index) => _PostTile(
                // Клетка едет за постом, а не за местом в сетке.
                key: ValueKey(posts[index].id),
                post: posts[index],
                onTap: () => _openPosts(index, profile),
              ),
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
    );
  }
}

/// Кто это: аватар, имя, сколько постов, с какого времени здесь
/// и «о себе» (specs/009-user-profile.md, требование 1).
class _Header extends StatelessWidget {
  const _Header({required this.profile, required this.mine});

  final UserProfile profile;
  final bool mine;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final quiet = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Avatar(
              name: profile.name,
              link: profile.avatarUrl,
              radius: AvatarRadius.onScreen,
              tone: mine ? AvatarTone.own : AvatarTone.author,
            ),
            const SizedBox(width: AppGap.medium),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Имя — вторая краска темы, как у автора в ленте.
                  Text(
                    profile.name,
                    style: theme.textTheme.titleLarge?.copyWith(
                      color: theme.colorScheme.secondary,
                    ),
                  ),
                  Text(postsCount(profile.posts), style: quiet),
                  Text(hereSince(profile.createdAt), style: quiet),
                ],
              ),
            ),
          ],
        ),
        if (profile.about.isNotEmpty) ...[
          const SizedBox(height: AppGap.medium),
          Text(profile.about, style: theme.textTheme.bodyLarge),
        ],
      ],
    );
  }
}

/// «1 пост», «3 поста», «14 постов»; «постов пока нет», когда их нет.
String postsCount(int count) => count == 0
    ? 'постов пока нет'
    : countWord(count, 'пост', 'поста', 'постов');

/// «в МоейДаче с мая»; год — только если не нынешний.
///
/// [now] задаётся только в проверках.
String hereSince(DateTime moment, {DateTime? now}) {
  const months = [
    'января', 'февраля', 'марта', 'апреля', 'мая', 'июня', //
    'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
  ];
  final local = moment.toLocal();
  final year = (now ?? DateTime.now()).year;
  final month = months[local.month - 1];
  return local.year == year
      ? 'в МоейДаче с $month'
      : 'в МоейДаче с $month ${local.year}';
}

/// Клетка сетки: первая фотография поста, обрезанная по центру.
class _PostTile extends StatelessWidget {
  const _PostTile({super.key, required this.post, required this.onTap});

  final Post post;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final photo = post.media.isEmpty ? null : post.media.first;
    final count = post.media.length;

    return Semantics(
      button: true,
      label: count > 1 ? 'Пост, фотографий: $count' : 'Пост',
      child: ClipRRect(
        borderRadius: BorderRadius.circular(AppGap.small),
        child: Stack(
          fit: StackFit.expand,
          children: [
            ColoredBox(color: theme.colorScheme.surfaceContainerHighest),
            if (photo != null)
              Image.network(mediaUrl(photo.url), fit: BoxFit.cover),
            if (count > 1)
              const Positioned(
                top: AppGap.tiny,
                right: AppGap.tiny,
                // Значок лежит на фотографии, а какая она — неизвестно:
                // белое с тенью видно на любом снимке, как точки в ленте.
                child: Icon(
                  Icons.collections_outlined,
                  color: Colors.white,
                  shadows: [Shadow(blurRadius: AppGap.tiny)],
                ),
              ),
            Material(
              type: MaterialType.transparency,
              child: InkWell(onTap: onTap),
            ),
          ],
        ),
      ),
    );
  }
}

/// Посты одного человека страницами — общие для сетки и прокрутки:
/// лайк, поставленный в прокрутке, виден и в сетке, а подгруженное
/// в прокрутке не надо загружать в сетке заново.
class UserPostsPager extends ChangeNotifier {
  UserPostsPager({required this.token, required this.userId});

  final String token;
  final String userId;

  final List<Post> posts = [];
  String? _cursor;
  bool loadingMore = false;
  String? nextPageError;

  UsersApi get _api => UsersApi(apiClient(token: token));

  /// Первая страница заново — вместо накопленного. Ошибка уходит тому,
  /// кто спросил: без первой страницы показывать нечего.
  Future<void> refresh() async {
    final page = await _api.getUserPosts(userId, limit: profilePageSize);
    posts
      ..clear()
      ..addAll(page?.items ?? const []);
    _cursor = page?.nextCursor;
    nextPageError = null;
    notifyListeners();
  }

  /// Следующая страница. Ошибка не стирает показанное: под ним —
  /// сообщение и «Повторить» (specs/009-user-profile.md, требование 15).
  Future<void> loadMore() async {
    final cursor = _cursor;
    if (cursor == null || loadingMore) {
      return;
    }
    loadingMore = true;
    nextPageError = null;
    notifyListeners();
    try {
      final page = await _api.getUserPosts(
        userId,
        limit: profilePageSize,
        cursor: cursor,
      );
      posts.addAll(page?.items ?? const []);
      _cursor = page?.nextCursor;
    } on Exception catch (error) {
      debugPrint('$logMarker user_posts=failed error=$error');
      nextPageError = errorMessage(error);
    } finally {
      loadingMore = false;
      notifyListeners();
    }
  }

  /// Пост изменился: его отметили здесь или на экране поста.
  void replace(Post post) {
    final at = posts.indexWhere((item) => item.id == post.id);
    if (at < 0) {
      return;
    }
    posts[at] = post;
    notifyListeners();
  }

  /// Пост удалён (specs/009-user-profile.md, требование 13).
  void remove(String postId) {
    posts.removeWhere((item) => item.id == postId);
    notifyListeners();
  }
}
