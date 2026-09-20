// Главный экран — лента (specs/004-feed.md).
//
// Всё, что до ленты, экран делает ради неё: узнаёт, кто вошёл, и, если
// человек ещё не знакомился, показывает знакомство (specs/002-profile.md).
// Дальше он отдаёт место постам: аватар в заголовке ведёт в профиль,
// кнопка внизу — к новому посту.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';
import '../widgets/feed_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/user_avatar.dart';
import 'intro_screen.dart';
import 'new_post_screen.dart';
import 'post_screen.dart';
import 'profile_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({
    super.key,
    required this.token,
    required this.onSignedOut,
  });

  final String token;

  /// Выход: токен забывает и приложение, и сервис.
  final Future<void> Function() onSignedOut;

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  final GlobalKey<FeedViewState> _feed = GlobalKey<FeedViewState>();

  CurrentUser? _user;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final user = await ProfileApi(apiClient(token: widget.token)).getMe();
      debugPrint('$logMarker screen=home user=${user?.id} name=${user?.name}');
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

  Future<void> _openProfile(CurrentUser user) async {
    final updated = await Navigator.of(context).push<CurrentUser>(
      MaterialPageRoute(
        builder: (_) => ProfileScreen(
          token: widget.token,
          user: user,
          onSignedOut: widget.onSignedOut,
        ),
      ),
    );
    if (updated != null && mounted) {
      setState(() => _user = updated);
      // Имя и аватар автора лежат в каждом посте, поэтому после правки
      // профиля лента показывает старые, пока её не перечитать.
      _feed.currentState?.refresh();
    }
  }

  Future<void> _newPost() async {
    final post = await Navigator.of(context).push<Post>(
      MaterialPageRoute(builder: (_) => NewPostScreen(token: widget.token)),
    );
    if (post == null || !mounted) {
      return;
    }
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => PostScreen(post: post, token: widget.token),
      ),
    );
    // Свой пост человек должен увидеть первым в ленте, вернувшись
    // с экрана поста (specs/004-feed.md, требование 8).
    _feed.currentState?.refresh();
  }

  void _openPost(Post post) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => PostScreen(
          post: post,
          token: widget.token,
          onChanged: (updated) => _feed.currentState?.replace(updated),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final user = _user;
    final error = _error;

    // Имя пустое — пользователь ещё не знакомился. Это единственное
    // состояние, в котором приложение не пускает дальше.
    if (user != null && user.name.isEmpty) {
      return IntroScreen(
        token: widget.token,
        onDone: (introduced) => setState(() => _user = introduced),
      );
    }

    final Widget body;
    if (user != null) {
      body = FeedView(
        key: _feed,
        token: widget.token,
        onOpenPost: _openPost,
        onNewPost: _newPost,
      );
    } else if (error != null) {
      body = ErrorView(message: error, onRetry: _load);
    } else {
      body = const LoadingView(label: 'Открываю ленту…');
    }

    return AppScreen(
      title: 'Лента',
      padded: false,
      actions: [
        if (user != null)
          IconButton(
            tooltip: 'Профиль',
            onPressed: () => _openProfile(user),
            icon: UserAvatar(user: user, radius: AvatarRadius.inBar),
          ),
      ],
      floatingActionButton: user == null
          ? null
          : FloatingActionButton.extended(
              onPressed: _newPost,
              icon: const Icon(Icons.add_a_photo_outlined),
              label: const Text('Новый пост'),
            ),
      child: body,
    );
  }
}
