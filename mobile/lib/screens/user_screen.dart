// Профиль пользователя: specs/009-user-profile.md, подписки —
// specs/012-follows.md.
//
// Кто этот человек, сколько у него постов, подписчиков и подписок,
// кнопка подписки и все его посты сеткой по три, как в Инстаграме.
// Посты закрытого профиля видят только подписчики: остальным вместо
// сетки — замок.
// Касание клетки открывает посты этого человека подряд
// (user_posts_screen.dart). Свой профиль — тот же экран с одной лишней
// кнопкой: человек видит себя так, как его видят соседи.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../app_scope.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/author_line.dart';
import '../widgets/bottom_bar.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/follow_button.dart';
import '../widgets/loading_view.dart';
import '../widgets/place_field.dart';
import '../widgets/user_avatar.dart';
import 'follow_list_screen.dart';
import 'groups_screen.dart';
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
  VoidCallback? onFollowChanged,
}) => Navigator.of(context).push<void>(
  MaterialPageRoute(
    builder: (_) => UserScreen(
      token: token,
      viewerId: viewerId,
      userId: userId,
      onPostChanged: onPostChanged,
      onPostDeleted: onPostDeleted,
      onProfileEdited: onProfileEdited,
      onFollowChanged: onFollowChanged,
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
    this.onFollowChanged,
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

  /// Смотрящий на кого-то подписался или отписался — здесь или в списке:
  /// вкладка «Подписки» ленты теперь другая.
  final VoidCallback? onFollowChanged;

  @override
  State<UserScreen> createState() => UserScreenState();
}

class UserScreenState extends State<UserScreen> {
  final ScrollController _scroll = ScrollController();
  late final UserPostsPager _posts = UserPostsPager(
    token: widget.token,
    userId: widget.userId,
  );

  UserProfile? _profile;
  String? _error;
  bool _editing = false;
  bool _blocking = false;

  bool get _mine => widget.userId == widget.viewerId;

  /// Видны ли смотрящему посты и списки: профиль открыт, свой или
  /// смотрящий подписан (specs/012-follows.md, требование 7).
  /// Заблокированного не видно, пока не разблокируешь
  /// (specs/022-edit-block-delete.md, требование 15).
  static bool canSeeInside(UserProfile profile, {required bool mine}) =>
      mine ||
      !profile.blocked &&
          (!profile.closed ||
              profile.relation?.following == RelationFollowingEnum.yes);

  @override
  void initState() {
    super.initState();
    // Свой профиль — раздел панели, его считает главный экран.
    if (!_mine) {
      usage.screen('user');
    }
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

  /// Перечитать профиль: свой пост выложен или удалён в ленте, а профиль
  /// открыт разделом нижней панели (specs/011-bottom-bar.md, требование 8).
  Future<void> refresh() => _load();

  /// К самому верху профиля: повторное касание «Профиля» в нижней панели
  /// (specs/011-bottom-bar.md, требование 4).
  Future<void> scrollToTop() => scrollBackToTop(_scroll);

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
  /// самое заново (specs/009-user-profile.md, требование 14). Посты
  /// закрытого профиля без подписки не запрашиваются: сервис ответит
  /// отказом, а показать вместо них нужно замок.
  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final profile = await UsersApi(apiClient(token: widget.token))
          .getUser(widget.userId);
      if (profile == null) {
        throw ApiException(200, 'Сервис не вернул профиль');
      }
      final inside = canSeeInside(profile, mine: _mine);
      if (inside) {
        await _posts.refresh();
      } else {
        _posts.clear();
      }
      debugPrint(
        '$logMarker screen=user id=${widget.userId} '
        'posts=${profile.posts} followers=${profile.followers} '
        'following=${profile.following} closed=${profile.closed} '
        'relation=${profile.relation?.following} mine=$_mine',
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

  /// Подписались, отписались или подали заявку: у человека другое число
  /// подписчиков, а посты закрытого профиля могли открыться или закрыться.
  void _relationChanged(Relation relation) {
    widget.onFollowChanged?.call();
    _load();
  }

  /// Подписчики или подписки человека (specs/012-follows.md,
  /// требование 14). Вернувшись, профиль перечитывается: в списке могли
  /// подписаться на него самого.
  Future<void> _openList(UserProfile profile, FollowListTab tab) async {
    await Navigator.of(context).push<void>(
      MaterialPageRoute(
        builder: (_) => FollowListScreen(
          token: widget.token,
          viewerId: widget.viewerId,
          user: profile,
          tab: tab,
          onFollowChanged: widget.onFollowChanged,
        ),
      ),
    );
    if (mounted) {
      await _load();
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

  /// Свои группы и поиск групп (specs/029-groups.md, требование 29).
  Future<void> _openGroups(UserProfile profile) =>
      Navigator.of(context).push<void>(
        MaterialPageRoute(
          builder: (_) => GroupsScreen(
            token: widget.token,
            viewerId: widget.viewerId,
            place: profile.place,
          ),
        ),
      );

  /// Посты подряд, сразу на том, которого коснулись
  /// (specs/009-user-profile.md, требование 11).
  Future<void> _openPosts(int index, UserProfile profile) =>
      Navigator.of(context).push<void>(
        MaterialPageRoute(
          builder: (_) => UserPostsScreen(
            posts: _posts,
            startAt: index,
            title: profile.nickname,
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
      actions: [
        if (profile != null && !_mine)
          PopupMenuButton<bool>(
            tooltip: 'Ещё',
            enabled: !_blocking,
            onSelected: (block) => block ? _block(profile) : _unblock(),
            itemBuilder: (_) => [
              profile.blocked
                  ? const PopupMenuItem(
                      value: false,
                      child: Text('Разблокировать'),
                    )
                  : const PopupMenuItem(
                      value: true,
                      child: Text('Заблокировать'),
                    ),
            ],
          ),
      ],
      child: body,
    );
  }

  /// Заблокировать после вопроса: подписки пропадут в обе стороны
  /// (specs/022-edit-block-delete.md, требования 11 и 27).
  Future<void> _block(UserProfile profile) async {
    final agreed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('Заблокировать @${profile.nickname}?'),
        content: const Text(
          'Он перестанет видеть ваши посты и комментарии, а вы — его. '
          'Подписки между вами пропадут.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Отмена'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('Заблокировать'),
          ),
        ],
      ),
    );
    if (agreed != true || !mounted) {
      return;
    }
    await _setBlocked(block: true);
  }

  Future<void> _unblock() => _setBlocked(block: false);

  Future<void> _setBlocked({required bool block}) async {
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _blocking = true);
    try {
      final api = FollowsApi(apiClient(token: widget.token));
      if (block) {
        await api.blockUser(widget.userId);
      } else {
        await api.unblockUser(widget.userId);
      }
      debugPrint(
        '$logMarker user=${block ? 'blocked' : 'unblocked'} id=${widget.userId}',
      );
      // Подписки могли пропасть, посты — спрятаться или открыться.
      widget.onFollowChanged?.call();
      await _load();
    } on Exception catch (error) {
      debugPrint('$logMarker user=block_failed error=$error');
      messenger.showSnackBar(SnackBar(content: Text(errorMessage(error))));
    } finally {
      if (mounted) {
        setState(() => _blocking = false);
      }
    }
  }

  Widget _content(UserProfile profile) {
    final posts = _posts.posts;
    final nextPageError = _posts.nextPageError;
    final inside = canSeeInside(profile, mine: _mine);
    final relation = profile.relation;

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
        SliverToBoxAdapter(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              AppGap.medium,
              AppGap.medium,
              AppGap.medium,
              0,
            ),
            child: FollowCounts(
              profile: profile,
              onOpen: inside ? (tab) => _openList(profile, tab) : null,
            ),
          ),
        ),
        if (!_mine && relation != null && !profile.blocked)
          SliverPadding(
            padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
            sliver: SliverToBoxAdapter(
              child: FollowButton(
                token: widget.token,
                userId: profile.id,
                relation: relation,
                onChanged: _relationChanged,
              ),
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
            // «Группы» — рядом с правкой профиля (specs/029-groups.md,
            // требование 29).
            sliver: SliverToBoxAdapter(
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: _editing ? null : _edit,
                      icon: const Icon(Icons.edit_outlined),
                      label: const Text('Изменить профиль'),
                    ),
                  ),
                  const SizedBox(width: AppGap.small),
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () => _openGroups(profile),
                      icon: const Icon(Icons.groups_outlined),
                      label: const Text('Группы'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        if (profile.blocked && !_mine)
          SliverFillRemaining(
            hasScrollBody: false,
            child: Padding(
              padding: const EdgeInsets.all(AppGap.large),
              child: EmptyView(
                icon: Icons.block_outlined,
                title: 'Вы заблокировали этого человека',
                action: OutlinedButton(
                  onPressed: _blocking ? null : _unblock,
                  child: const Text('Разблокировать'),
                ),
              ),
            ),
          )
        else if (!inside)
          SliverFillRemaining(
            hasScrollBody: false,
            child: Padding(
              padding: const EdgeInsets.all(AppGap.large),
              child: ClosedProfileView(nickname: profile.nickname),
            ),
          )
        else if (posts.isEmpty)
          SliverFillRemaining(
            hasScrollBody: false,
            child: Padding(
              padding: const EdgeInsets.all(AppGap.large),
              child: EmptyView(
                icon: Icons.eco_outlined,
                title: 'Постов пока нет',
                hint: _mine
                    ? 'Выложите первый — он появится здесь и в ленте'
                    : 'Когда ${profile.nickname} что-нибудь выложит, '
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

/// Кто это: аватар, никнейм и полное имя, с какого времени здесь и «о себе»
/// (specs/009-user-profile.md, требование 1). Сколько постов — в строке
/// чисел под шапкой (specs/012-follows.md, требование 23).
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
              name: profile.nickname,
              link: profile.avatarUrl,
              radius: AvatarRadius.onScreen,
              tone: mine ? AvatarTone.own : AvatarTone.author,
            ),
            const SizedBox(width: AppGap.medium),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Никнейм — вторая краска темы, как у автора в ленте,
                  // полное имя под ним, если человек его указал
                  // (specs/010-nicknames.md, требование 9).
                  // Замок справа от ника — профиль закрыт
                  // (specs/012-follows.md, требование 22).
                  // Никнейм без пробелов, и длинный при крупном шрифте рвался
                  // посреди слова: он уменьшается, оставаясь в одну строку.
                  FittedBox(
                    fit: BoxFit.scaleDown,
                    alignment: AlignmentDirectional.centerStart,
                    child: Text.rich(
                      maxLines: 1,
                      TextSpan(
                        text: profile.nickname,
                        children: [
                          if (profile.closed)
                            WidgetSpan(
                              alignment: PlaceholderAlignment.middle,
                              child: Padding(
                                padding: const EdgeInsets.only(
                                  left: AppGap.tiny,
                                ),
                                child: Icon(
                                  Icons.lock_outline,
                                  semanticLabel: 'Закрытый профиль',
                                  color: theme.colorScheme.onSurface,
                                ),
                              ),
                            ),
                        ],
                      ),
                      style: theme.textTheme.titleLarge?.copyWith(
                        color: theme.colorScheme.secondary,
                      ),
                    ),
                  ),
                  if (profile.name.isNotEmpty)
                    Text(profile.name, style: theme.textTheme.titleMedium),
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
        if (profile.place case final place?) ...[
          const SizedBox(height: AppGap.small),
          PlaceLine(place: place),
        ],
      ],
    );
  }
}

/// Три числа между двумя кантами: посты, подписчики, подписки
/// (specs/012-follows.md, требование 23). Подписчики и подписки касаемы,
/// когда список можно открыть; [onOpen] — `null`, когда нельзя.
class FollowCounts extends StatelessWidget {
  const FollowCounts({super.key, required this.profile, this.onOpen});

  final UserProfile profile;
  final void Function(FollowListTab tab)? onOpen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final line = BorderSide(
      color: theme.colorScheme.outlineVariant,
      width: AppShape.hairline,
    );
    final open = onOpen;

    return DecoratedBox(
      decoration: BoxDecoration(
        border: Border(top: line, bottom: line),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: AppGap.tiny),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _count(context, profile.posts, postsWord(profile.posts), null),
            _count(
              context,
              profile.followers,
              followersWord(profile.followers),
              open == null ? null : () => open(FollowListTab.followers),
            ),
            _count(
              context,
              profile.following,
              followingWord(profile.following),
              open == null ? null : () => open(FollowListTab.following),
            ),
          ],
        ),
      ),
    );
  }

  Widget _count(
    BuildContext context,
    int number,
    String word,
    VoidCallback? onTap,
  ) {
    final theme = Theme.of(context);

    return Expanded(
      child: Semantics(
        button: onTap != null,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(AppGap.small),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              vertical: AppGap.small,
              horizontal: AppGap.tiny,
            ),
            child: Column(
              children: [
                Text('$number', style: theme.textTheme.titleLarge),
                // При крупном системном шрифте «подписчиков» не влезает в
                // треть ширины и рвалось посреди слова; слово уменьшается
                // целиком (specs/000-ui.md, требование 2).
                FittedBox(
                  fit: BoxFit.scaleDown,
                  child: Text(
                    word,
                    maxLines: 1,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Слово под числом: «пост», «поста», «постов».
String postsWord(int count) => _word(count, 'пост', 'поста', 'постов');

/// «подписчик», «подписчика», «подписчиков».
String followersWord(int count) =>
    _word(count, 'подписчик', 'подписчика', 'подписчиков');

/// «подписка», «подписки», «подписок».
String followingWord(int count) =>
    _word(count, 'подписка', 'подписки', 'подписок');

/// Слово к числу — без самого числа: число над ним крупно.
String _word(int count, String one, String few, String many) =>
    countWord(count, one, few, many).substring('$count '.length);

/// Вместо сетки постов у закрытого профиля без подписки
/// (specs/012-follows.md, требование 22).
class ClosedProfileView extends StatelessWidget {
  const ClosedProfileView({super.key, required this.nickname});

  final String nickname;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return EmptyView(
      icon: Icons.lock_outline,
      art: DecoratedBox(
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          border: Border.all(
            color: theme.colorScheme.outlineVariant,
            width: AppShape.hairline,
          ),
        ),
        child: Padding(
          padding: const EdgeInsets.all(AppGap.medium),
          child: Icon(
            Icons.lock_outline,
            size: AppGap.large + AppGap.small,
            color: theme.colorScheme.onSurface,
          ),
        ),
      ),
      title: 'Закрытый профиль',
      hint: 'Посты $nickname видят только его подписчики',
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
        borderRadius: BorderRadius.circular(AppShape.small),
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

  /// Постов не видно: профиль закрыт, а смотрящий не подписан или
  /// отписался (specs/012-follows.md, требование 7).
  void clear() {
    posts.clear();
    _cursor = null;
    nextPageError = null;
    notifyListeners();
  }

  /// Пост удалён (specs/009-user-profile.md, требование 13).
  void remove(String postId) {
    posts.removeWhere((item) => item.id == postId);
    notifyListeners();
  }
}
