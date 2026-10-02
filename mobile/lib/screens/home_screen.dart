// Главный экран — лента и свой профиль под нижней панелью
// (specs/004-feed.md, specs/011-bottom-bar.md).
//
// Всё, что до ленты, экран делает ради неё: узнаёт, кто вошёл, и, если
// человек ещё не знакомился, показывает знакомство (specs/002-profile.md).
// Дальше внизу встаёт панель: «Лента», «Новый пост», «Уведомления»
// и «Профиль». Лента, уведомления и профиль — разделы
// (specs/014-notifications.md, требование 9). У каждого своя стопка экранов (Navigator): пост,
// чужой профиль, правка профиля открываются внутри раздела, и панель
// остаётся под ними. Поверх всего, без панели, — только новый пост
// (specs/011-bottom-bar.md, требование 1).
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../push.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/bottom_bar.dart';
import '../widgets/error_view.dart';
import '../widgets/feed_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/slide_up_route.dart';
import 'intro_screen.dart';
import 'notifications_screen.dart';
import 'new_post_screen.dart';
import 'post_screen.dart';
import 'tag_posts_screen.dart';
import 'user_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key, required this.token, required this.onSignedOut});

  final String token;

  /// Выход: токен забывает и приложение, и сервис.
  final Future<void> Function() onSignedOut;

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  final GlobalKey<FeedTabsState> _feed = GlobalKey<FeedTabsState>();
  final GlobalKey<UserScreenState> _profile = GlobalKey<UserScreenState>();
  final GlobalKey<NotificationsScreenState> _notifications =
      GlobalKey<NotificationsScreenState>();

  /// Стопки экранов разделов.
  final Map<HomeTab, GlobalKey<NavigatorState>> _stacks = {
    for (final tab in HomeTab.values) tab: GlobalKey<NavigatorState>(),
  };

  CurrentUser? _user;
  String? _error;
  HomeTab _tab = HomeTab.feed;

  /// Профиль строится при первом касании «Профиля», а не при входе:
  /// кто туда не заходил, не ждёт лишнего запроса.
  bool _profileOpened = false;

  /// Уведомления тоже: их открытие отмечает всё прочитанным, и строить
  /// раздел заранее значило бы погасить точку, которую ещё не видели.
  bool _notificationsOpened = false;

  /// Есть ли непрочитанное — точка на колокольчике.
  bool _unread = false;

  /// Пуши (specs/024-push.md). Запускаются, когда человек уже на главном
  /// экране, а не на знакомстве: там вопрос о разрешении не к месту.
  final PushClient _push = PushClient();
  bool _pushStarted = false;

  @override
  void initState() {
    super.initState();
    usage.begin(widget.token, tab: _tab.name);
    _load();
  }

  @override
  void dispose() {
    usage.finish();
    _push.stop();
    super.dispose();
  }

  /// Пуши — один раз за вход, когда человек знаком (требование 1).
  void _startPush(CurrentUser user) {
    if (_pushStarted || !user.nicknameChosen) {
      return;
    }
    _pushStarted = true;
    _push.start(
      token: widget.token,
      onOpen: _openPush,
      onForeground: _checkUnread,
    );
  }

  /// Касание пуша открывает, о чём он, в разделе «Уведомления», как
  /// касание его строки (specs/024-push.md, требование 4).
  Future<void> _openPush(PushTarget target) async {
    debugPrint('$logMarker push=opened kind=${target.kind}');
    if (!mounted || _user == null) {
      return;
    }
    if (_tab != HomeTab.notifications) {
      _select(HomeTab.notifications);
    } else {
      _notificationsStack.popUntil((route) => route.isFirst);
    }
    // Стопка раздела появляется только со следующим кадром.
    await WidgetsBinding.instance.endOfFrame;
    if (!mounted) {
      return;
    }
    final postId = target.postId;
    final userId = target.userId;
    switch (target.kind) {
      case 'like' || 'comment' when postId != null:
        await _openNotificationPost(postId);
      case 'follow' || 'follow_accepted' when userId != null:
        await _openNotificationProfile(userId);
      default:
      // Заявка на подписку, заявка в группу и приглашение — в самом
      // разделе, сверху.
    }
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final user = await ProfileApi(apiClient(token: widget.token)).getMe();
      debugPrint(
        '$logMarker screen=home user=${user?.id} nickname=${user?.nickname}',
      );
      if (!mounted) {
        return;
      }
      setState(() => _user = user);
      _checkUnread();
      if (user != null) {
        _startPush(user);
      }
    } on Exception catch (error) {
      debugPrint('$logMarker screen=home error=$error');
      // Сессии больше нет — значит, человек не вошёл, что бы ни лежало
      // в памяти устройства.
      if (serviceErrorCode(error) == 'unauthorized') {
        await widget.onSignedOut();
        return;
      }
      if (!mounted) {
        return;
      }
      setState(() => _error = errorMessage(error));
    }
  }

  /// Есть ли новое: при запуске, при каждом переключении раздела и при
  /// обновлении ленты — без фонового опроса (specs/014-notifications.md,
  /// требование 6). Не узнали — точка остаётся какой была.
  Future<void> _checkUnread() async {
    try {
      final counts = await NotificationsApi(apiClient(token: widget.token))
          .getUnreadNotifications();
      if (counts == null || !mounted) {
        return;
      }
      // Заявки и приглашения в группы — тоже повод для точки
      // (specs/029-groups.md, требование 27).
      final unread =
          counts.unread > 0 || counts.requests > 0 || (counts.groups ?? 0) > 0;
      debugPrint(
        '$logMarker notifications unread=${counts.unread} '
        'requests=${counts.requests} groups=${counts.groups}',
      );
      setState(() => _unread = unread);
    } on Exception catch (error) {
      debugPrint('$logMarker notifications unread_failed error=$error');
    }
  }

  /// Касание раздела в нижней панели. Касание уже открытого закрывает
  /// открытое в нём поверх, а если закрывать нечего — возвращает раздел
  /// к самому верху (specs/011-bottom-bar.md, требование 4).
  void _select(HomeTab tab) {
    if (tab != _tab) {
      usage.tab(tab.name);
      final reopened = tab == HomeTab.notifications && _notificationsOpened;
      setState(() {
        _tab = tab;
        _profileOpened = _profileOpened || tab == HomeTab.profile;
        _notificationsOpened =
            _notificationsOpened || tab == HomeTab.notifications;
      });
      // Раздел уведомлений при каждом открытии читается заново и гасит
      // точку (требование 5); в первый раз он сделает это сам.
      if (reopened) {
        _notifications.currentState?.refresh();
      } else {
        _checkUnread();
      }
      return;
    }
    final stack = _stacks[tab]!.currentState;
    if (stack != null && stack.canPop()) {
      stack.popUntil((route) => route.isFirst);
      return;
    }
    switch (tab) {
      case HomeTab.feed:
        _feed.currentState?.scrollToTop();
      case HomeTab.notifications:
        _notifications.currentState?.scrollToTop();
      case HomeTab.profile:
        _profile.currentState?.scrollToTop();
    }
  }

  /// «Назад» телефона: закрывает открытое в разделе поверх, из профиля
  /// возвращает в ленту, из ленты — закрывает приложение
  /// (specs/011-bottom-bar.md, требование 9).
  Future<void> _back() async {
    final stack = _stacks[_tab]!.currentState;
    // maybePop, а не pop: экран правки профиля сам решает, что вернуть.
    if (stack != null && await stack.maybePop()) {
      return;
    }
    if (_tab != HomeTab.feed) {
      usage.tab(HomeTab.feed.name);
      setState(() => _tab = HomeTab.feed);
      return;
    }
    await SystemNavigator.pop();
  }

  /// Свой пост выложен или удалён: он должен появиться или исчезнуть
  /// и в ленте, и в сетке своего профиля (specs/011-bottom-bar.md,
  /// требование 8).
  void _ownPostsChanged() {
    _feed.currentState?.refresh();
    _profile.currentState?.refresh();
  }

  /// Свой никнейм или аватар поправили: они в каждом своём посте ленты,
  /// поэтому лента перечитывается.
  void _profileEdited(CurrentUser updated) {
    if (!mounted) {
      return;
    }
    setState(() => _user = updated);
    _feed.currentState?.refresh();
  }

  /// Стопка экранов ленты: пост и чужой профиль открываются в ней, под
  /// панелью.
  NavigatorState get _feedStack => _stacks[HomeTab.feed]!.currentState!;

  /// Стопка раздела уведомлений: пост и профиль из строки открываются
  /// в ней, под панелью.
  NavigatorState get _notificationsStack =>
      _stacks[HomeTab.notifications]!.currentState!;

  /// Пост из строки уведомления. Строка знает только его номер, поэтому
  /// сначала пост читается; удалённого поста строки уже нет, но раздел
  /// мог открыться раньше удаления.
  Future<void> _openNotificationPost(String postId) async {
    final user = _user;
    if (user == null) {
      return;
    }
    final Post? post;
    try {
      post = await PostsApi(apiClient(token: widget.token)).getPost(postId);
    } on Exception catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(errorMessage(error))));
      }
      _notifications.currentState?.refresh();
      return;
    }
    if (post == null || !mounted) {
      return;
    }
    final deleted = await _notificationsStack.push<bool>(
      MaterialPageRoute(
        builder: (_) => PostScreen(
          post: post!,
          token: widget.token,
          viewerId: user.id,
          onChanged: (updated) => _feed.currentState?.replace(updated),
        ),
      ),
    );
    if (deleted == true) {
      _ownPostsChanged();
      _notifications.currentState?.refresh();
    }
  }

  /// Профиль из строки уведомления: свой — раздел «Профиль».
  Future<void> _openNotificationProfile(String userId) async {
    final user = _user;
    if (user == null) {
      return;
    }
    if (userId == user.id) {
      _select(HomeTab.profile);
      return;
    }
    await openUserProfile(
      _notificationsStack.context,
      token: widget.token,
      viewerId: user.id,
      userId: userId,
      onPostChanged: (post) => _feed.currentState?.replace(post),
      onPostDeleted: _ownPostsChanged,
      onProfileEdited: _profileEdited,
      onFollowChanged: _followChanged,
    );
  }

  /// Профиль человека по автору поста. Свой — раздел «Профиль», а не
  /// второй экран поверх ленты (specs/011-bottom-bar.md, требование 7).
  Future<void> _openProfile(String userId) async {
    final user = _user;
    if (user == null) {
      return;
    }
    if (userId == user.id) {
      _select(HomeTab.profile);
      return;
    }
    await openUserProfile(
      _feedStack.context,
      token: widget.token,
      viewerId: user.id,
      userId: userId,
      onPostChanged: (post) => _feed.currentState?.replace(post),
      onPostDeleted: _ownPostsChanged,
      onProfileEdited: _profileEdited,
      onFollowChanged: _followChanged,
    );
  }

  /// На кого-то подписались или отписались: вкладка «Подписки» теперь
  /// другая (specs/012-follows.md, сценарий, шаги 4 и 5).
  void _followChanged() => _feed.currentState?.refresh();

  /// «Новый пост» в панели или в пустой ленте. Экран создания — поверх
  /// всего, без панели. Опубликовав, человек видит свой пост в ленте,
  /// а закрыв его — ленту с самого верха, где его пост первый
  /// (specs/011-bottom-bar.md, требование 5; specs/004-feed.md,
  /// требование 8).
  Future<void> _newPost() async {
    final user = _user;
    // Выезжает снизу (макет «Сад», одобрено в раунде 2).
    final post = await Navigator.of(context, rootNavigator: true).push<Post>(
      SlideUpRoute(
        builder: (_) => NewPostScreen(
          token: widget.token,
          closed: user?.closed ?? false,
          place: user?.place,
        ),
      ),
    );
    if (post == null || user == null || !mounted) {
      return;
    }
    setState(() => _tab = HomeTab.feed);
    _feedStack.popUntil((route) => route.isFirst);
    _feed.currentState?.scrollToTop();
    _ownPostsChanged();
    final deleted = await _feedStack.push<bool>(
      MaterialPageRoute(
        builder: (_) =>
            PostScreen(post: post, token: widget.token, viewerId: user.id),
      ),
    );
    if (deleted == true) {
      _ownPostsChanged();
    }
  }

  /// Посты с тэгом — поверх ленты (specs/028-post-tags.md, требование 24).
  Future<void> _openTag(String tag) async {
    final user = _user;
    if (user == null) {
      return;
    }
    await openTagPosts(
      _feedStack.context,
      token: widget.token,
      viewerId: user.id,
      tag: tag,
      onPostChanged: (post) => _feed.currentState?.replace(post),
    );
  }

  Future<void> _openPost(Post post) async {
    final user = _user;
    if (user == null) {
      return;
    }

    final deleted = await _feedStack.push<bool>(
      MaterialPageRoute(
        builder: (_) => PostScreen(
          post: post,
          token: widget.token,
          viewerId: user.id,
          // Фото из ленты раскрывается в пост (макет, раунд 1).
          heroTag: postHeroTag(post),
          onChanged: (updated) => _feed.currentState?.replace(updated),
        ),
      ),
    );
    // Пост удалён: в ленте его больше нет, и показывать его там нельзя
    // (specs/007-deletion.md, требование 7).
    if (deleted == true) {
      _ownPostsChanged();
    }
  }

  /// Раздел — своя стопка экранов с первым экраном [root].
  Widget _stack(HomeTab tab, Widget Function() root) => Navigator(
    key: _stacks[tab],
    // Первый экран строится заново при каждой перестройке главного:
    // так в него доходят свежие сведения о том, кто вошёл.
    onGenerateInitialRoutes: (_, _) => [
      MaterialPageRoute<void>(builder: (_) => root()),
    ],
  );

  @override
  Widget build(BuildContext context) {
    final user = _user;
    final error = _error;

    // Никнейм не выбран — пользователь ещё не знакомился или завёлся
    // до никнеймов и носит временный. Это единственное состояние,
    // в котором приложение не пускает дальше (specs/010-nicknames.md).
    if (user != null && !user.nicknameChosen) {
      return IntroScreen(
        token: widget.token,
        user: user,
        onDone: (introduced) {
          setState(() => _user = introduced);
          _startPush(introduced);
        },
      );
    }

    if (user == null) {
      // Пока неизвестно, кто вошёл, панели нет: без человека нечем
      // показать профиль (specs/011-bottom-bar.md, «Ограничения»).
      return AppScreen(
        padded: false,
        child: error != null
            ? ErrorView(message: error, onRetry: _load)
            : const LoadingView(label: 'Открываю ленту…'),
      );
    }

    return PopScope(
      // «Назад» решает главный экран: стопки разделов системе не видны.
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          _back();
        }
      },
      child: Scaffold(
        body: IndexedStack(
          index: _tab.index,
          children: [
            _stack(
              HomeTab.feed,
              () => AppScreen(
                // Заголовка нет: на главном экране в заголовке стоит
                // логотип (specs/000-ui.md, правило 14).
                padded: false,
                child: FeedTabs(
                  key: _feed,
                  token: widget.token,
                  onRefreshed: _checkUnread,
                  onOpenPost: _openPost,
                  onNewPost: _newPost,
                  onOpenAuthor: (author) => _openProfile(author.id),
                  onOpenTag: _openTag,
                ),
              ),
            ),
            if (_notificationsOpened)
              _stack(
                HomeTab.notifications,
                () => NotificationsScreen(
                  key: _notifications,
                  token: widget.token,
                  viewerId: user.id,
                  onOpenPost: _openNotificationPost,
                  onOpenProfile: _openNotificationProfile,
                  onSeen: _checkUnread,
                ),
              )
            else
              const SizedBox.shrink(),
            if (_profileOpened)
              _stack(
                HomeTab.profile,
                () => UserScreen(
                  key: _profile,
                  token: widget.token,
                  viewerId: user.id,
                  userId: user.id,
                  onPostChanged: (post) => _feed.currentState?.replace(post),
                  onPostDeleted: () => _feed.currentState?.refresh(),
                  onProfileEdited: _profileEdited,
                  onFollowChanged: _followChanged,
                ),
              )
            else
              const SizedBox.shrink(),
          ],
        ),
        bottomNavigationBar: AppBottomBar(
          tab: _tab,
          onSelect: _select,
          onNewPost: _newPost,
          unread: _unread,
        ),
      ),
    );
  }
}
