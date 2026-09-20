// Экран вошедшего пользователя.
//
// Ленты пока нет (specs/000-overview.md), поэтому экран отвечает на три
// вопроса — кто вошёл, как ему опубликовать пост и как открыть свой
// профиль, — и даёт выйти.
//
// Пользователя без имени экран не показывает вовсе: сначала знакомство
// (specs/002-profile.md).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
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
    required this.isNewUser,
    required this.onSignedOut,
  });

  final String token;

  /// Этим входом пользователь зарегистрировался впервые.
  final bool isNewUser;

  /// Выход: токен забывает и приложение, и сервис.
  final Future<void> Function() onSignedOut;

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  CurrentUser? _user;
  String? _error;
  bool _busy = false;

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
        builder: (_) => ProfileScreen(token: widget.token, user: user),
      ),
    );
    if (updated != null && mounted) {
      setState(() => _user = updated);
    }
  }

  Future<void> _newPost() async {
    final post = await Navigator.of(context).push<Post>(
      MaterialPageRoute(builder: (_) => NewPostScreen(token: widget.token)),
    );
    if (post == null || !mounted) {
      return;
    }
    await Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (_) => PostScreen(post: post)));
  }

  Future<void> _signOut() async {
    setState(() => _busy = true);
    try {
      await AuthApi(apiClient(token: widget.token)).deleteSession();
    } on Exception catch (error) {
      // Сервис мог не ответить, но на этом устройстве человек уже вышел.
      debugPrint('$logMarker auth=sign_out_failed error=$error');
    }
    await widget.onSignedOut();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
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

    // Ленты пока нет (specs/000-overview.md), и показывать на главном
    // экране нечего. Это и есть его пустое состояние.
    final String title;
    if (user == null) {
      title = 'Вы вошли';
    } else if (widget.isNewUser) {
      title = 'Добро пожаловать, ${user.name}!';
    } else {
      title = 'С возвращением, ${user.name}!';
    }

    final Widget? about;
    if (user != null && user.about.isNotEmpty) {
      about = Text(
        user.about,
        style: theme.textTheme.bodyLarge,
        textAlign: TextAlign.center,
      );
    } else if (user == null && error != null) {
      about = ErrorView(message: error, onRetry: _load);
    } else if (user == null) {
      about = const LoadingView(label: 'Открываю профиль…');
    } else {
      about = null;
    }

    return AppScreen(
      actions: [
        if (user != null)
          IconButton(
            tooltip: 'Профиль',
            onPressed: () => _openProfile(user),
            icon: UserAvatar(user: user, radius: AvatarRadius.inBar),
          ),
      ],
      child: EmptyView(
        icon: Icons.eco_outlined,
        art: user == null
            ? null
            : UserAvatar(user: user, radius: AvatarRadius.onScreen),
        title: title,
        hint: 'Лента появится следующей фичей.',
        action: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (about != null) ...[about, const SizedBox(height: AppGap.large)],
            if (user != null) ...[
              FilledButton.icon(
                onPressed: _newPost,
                icon: const Icon(Icons.add_a_photo_outlined),
                label: const Text('Новый пост'),
              ),
              const SizedBox(height: AppGap.small),
              FilledButton.tonal(
                onPressed: () => _openProfile(user),
                child: const Text('Мой профиль'),
              ),
              const SizedBox(height: AppGap.small),
            ],
            OutlinedButton(
              onPressed: _busy ? null : _signOut,
              child: const Text('Выйти'),
            ),
          ],
        ),
      ),
    );
  }
}
