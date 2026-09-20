// Экран вошедшего пользователя.
//
// Пока показывать внутри нечего: профиль, лента и посты появятся
// следующими фичами (specs/000-overview.md). Поэтому экран отвечает
// ровно на один вопрос — кто вошёл, — и даёт выйти.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';

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
      final info = await AuthApi(apiClient(token: widget.token)).getSession();
      debugPrint('$logMarker screen=home user=${info?.user.id}');
      if (!mounted) {
        return;
      }
      setState(() => _user = info?.user);
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

    // Внутри пока пусто: лента, посты и профиль — следующие фичи
    // (specs/000-overview.md). Это и есть пустое состояние экрана.
    final Widget who;
    if (user != null) {
      who = Text(
        user.phone,
        style: theme.textTheme.bodyLarge,
        textAlign: TextAlign.center,
      );
    } else if (error != null) {
      who = ErrorView(message: error, onRetry: _load);
    } else {
      who = const LoadingView(label: 'Проверяю вход…');
    }

    return AppScreen(
      child: EmptyView(
        icon: Icons.eco_outlined,
        title: widget.isNewUser ? 'Добро пожаловать!' : 'Вы вошли',
        hint: 'Лента, посты и профиль появятся следующими фичами.',
        action: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            who,
            const SizedBox(height: AppGap.large),
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
