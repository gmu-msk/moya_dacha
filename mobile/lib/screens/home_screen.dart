// Экран вошедшего пользователя.
//
// Пока показывать внутри нечего: лента и посты появятся следующими
// фичами (specs/000-overview.md). Поэтому экран отвечает на два вопроса —
// кто вошёл и как ему открыть свой профиль, — и даёт выйти.
//
// Пользователя без имени экран не показывает вовсе: сначала знакомство
// (specs/002-profile.md).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../widgets/server_status.dart';
import '../widgets/user_avatar.dart';
import 'intro_screen.dart';
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

    // Имя пустое — пользователь ещё не знакомился. Это единственное
    // состояние, в котором приложение не пускает дальше.
    if (user != null && user.name.isEmpty) {
      return IntroScreen(
        token: widget.token,
        onDone: (introduced) => setState(() => _user = introduced),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('МояДача'),
        actions: [
          if (user != null)
            IconButton(
              tooltip: 'Профиль',
              onPressed: () => _openProfile(user),
              icon: UserAvatar(user: user, radius: 16),
            ),
        ],
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            children: [
              Expanded(
                child: Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (user != null)
                        UserAvatar(user: user, radius: 44)
                      else
                        Icon(
                          Icons.eco_outlined,
                          size: 72,
                          color: theme.colorScheme.primary,
                        ),
                      const SizedBox(height: 16),
                      Text(
                        user != null
                            ? (widget.isNewUser
                                  ? 'Добро пожаловать, ${user.name}!'
                                  : 'С возвращением, ${user.name}!')
                            : 'Вы вошли',
                        style: theme.textTheme.titleLarge,
                        textAlign: TextAlign.center,
                      ),
                      if (user != null && user.about.isNotEmpty) ...[
                        const SizedBox(height: 8),
                        Text(
                          user.about,
                          style: theme.textTheme.bodyLarge,
                          textAlign: TextAlign.center,
                        ),
                      ],
                      if (user == null) ...[
                        const SizedBox(height: 8),
                        if (_error != null)
                          Text(
                            _error!,
                            style: theme.textTheme.bodyMedium?.copyWith(
                              color: theme.colorScheme.error,
                            ),
                            textAlign: TextAlign.center,
                          )
                        else
                          const CircularProgressIndicator(),
                      ],
                      const SizedBox(height: 24),
                      Text(
                        'Лента и посты появятся следующими фичами.',
                        style: theme.textTheme.bodySmall,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 24),
                      if (user != null)
                        FilledButton.tonal(
                          onPressed: () => _openProfile(user),
                          child: const Text('Мой профиль'),
                        ),
                      const SizedBox(height: 8),
                      OutlinedButton(
                        onPressed: _busy ? null : _signOut,
                        child: const Text('Выйти'),
                      ),
                    ],
                  ),
                ),
              ),
              const ServerStatus(),
            ],
          ),
        ),
      ),
    );
  }
}
