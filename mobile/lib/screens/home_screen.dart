// Главный экран — лента и свой профиль под нижней панелью
// (specs/004-feed.md, specs/011-bottom-bar.md).
//
// Всё, что до ленты, экран делает ради неё: узнаёт, кто вошёл, и, если
// человек ещё не знакомился, показывает знакомство (specs/002-profile.md).
// Дальше внизу встаёт панель: «Лента», «Новый пост» и «Профиль». Лента и
// профиль — разделы. У каждого своя стопка экранов (Navigator): пост,
// чужой профиль, правка профиля открываются внутри раздела, и панель
// остаётся под ними. Поверх всего, без панели, — только новый пост
// (specs/011-bottom-bar.md, требование 1).
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../widgets/app_screen.dart';
import '../widgets/bottom_bar.dart';
import '../widgets/error_view.dart';
import '../widgets/feed_view.dart';
import '../widgets/loading_view.dart';
import 'intro_screen.dart';
import 'new_post_screen.dart';
import 'post_screen.dart';
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

  @override
  void initState() {
    super.initState();
    _load();
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

  /// Касание раздела в нижней панели. Касание уже открытого закрывает
  /// открытое в нём поверх, а если закрывать нечего — возвращает раздел
  /// к самому верху (specs/011-bottom-bar.md, требование 4).
  void _select(HomeTab tab) {
    if (tab != _tab) {
      setState(() {
        _tab = tab;
        _profileOpened = _profileOpened || tab == HomeTab.profile;
      });
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
    if (_tab == HomeTab.profile) {
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
    final post = await Navigator.of(context, rootNavigator: true).push<Post>(
      MaterialPageRoute(builder: (_) => NewPostScreen(token: widget.token)),
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
        onDone: (introduced) => setState(() => _user = introduced),
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
                  onOpenPost: _openPost,
                  onNewPost: _newPost,
                  onOpenAuthor: (author) => _openProfile(author.id),
                ),
              ),
            ),
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
        ),
      ),
    );
  }
}
