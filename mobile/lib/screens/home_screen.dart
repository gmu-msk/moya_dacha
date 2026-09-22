// Главный экран — лента (specs/004-feed.md).
//
// Всё, что до ленты, экран делает ради неё: узнаёт, кто вошёл, и, если
// человек ещё не знакомился, показывает знакомство (specs/002-profile.md).
// Дальше он отдаёт место постам: аватар в заголовке ведёт в свой профиль,
// имя автора поста — в его (specs/009-user-profile.md), кнопка внизу —
// к новому посту.
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
import 'user_screen.dart';

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

  /// Профиль человека: свой — из заголовка, чужой — по автору поста.
  Future<void> _openProfile(String userId) async {
    final user = _user;
    if (user == null) {
      return;
    }
    await openUserProfile(
      context,
      token: widget.token,
      viewerId: user.id,
      userId: userId,
      onPostChanged: (post) => _feed.currentState?.replace(post),
      onPostDeleted: () => _feed.currentState?.refresh(),
      onProfileEdited: (updated) {
        if (!mounted) {
          return;
        }
        setState(() => _user = updated);
        // Имя и аватар автора лежат в каждом посте, поэтому после правки
        // профиля лента показывает старые, пока её не перечитать.
        _feed.currentState?.refresh();
      },
    );
  }

  Future<void> _newPost() async {
    final user = _user;
    final post = await Navigator.of(context).push<Post>(
      MaterialPageRoute(builder: (_) => NewPostScreen(token: widget.token)),
    );
    if (post == null || user == null || !mounted) {
      return;
    }
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => PostScreen(
          post: post,
          token: widget.token,
          viewerId: user.id,
        ),
      ),
    );
    // Свой пост человек должен увидеть первым в ленте, вернувшись
    // с экрана поста (specs/004-feed.md, требование 8).
    _feed.currentState?.refresh();
  }

  Future<void> _openPost(Post post) async {
    final user = _user;
    if (user == null) {
      return;
    }

    final deleted = await Navigator.of(context).push<bool>(
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
      _feed.currentState?.refresh();
    }
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
        onOpenAuthor: (author) => _openProfile(author.id),
      );
    } else if (error != null) {
      body = ErrorView(message: error, onRetry: _load);
    } else {
      body = const LoadingView(label: 'Открываю ленту…');
    }

    return AppScreen(
      // Заголовка нет: на главном экране в заголовке стоит логотип
      // (specs/000-ui.md, правило 14).
      padded: false,
      actions: [
        if (user != null)
          IconButton(
            tooltip: 'Профиль',
            onPressed: () => _openProfile(user.id),
            icon: UserAvatar(user: user, radius: AvatarRadius.inBar),
          ),
      ],
      floatingActionButton: user == null
          ? null
          // Кнопка без подписи: значок фотоаппарата понятен и сам,
          // а подпись занимает половину ширины экрана.
          : FloatingActionButton(
              onPressed: _newPost,
              tooltip: 'Новый пост',
              child: const Icon(Icons.add_a_photo_outlined),
            ),
      child: body,
    );
  }
}
