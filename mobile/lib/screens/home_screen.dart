// Экран вошедшего пользователя.
//
// Пока показывать внутри нечего: профиль, лента и посты появятся
// следующими фичами (specs/000-overview.md). Поэтому экран отвечает
// ровно на один вопрос — кто вошёл, — и даёт выйти.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../widgets/server_status.dart';

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

    return Scaffold(
      appBar: AppBar(title: const Text('МояДача')),
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
                      Icon(
                        Icons.eco_outlined,
                        size: 72,
                        color: theme.colorScheme.primary,
                      ),
                      const SizedBox(height: 16),
                      Text(
                        widget.isNewUser ? 'Добро пожаловать!' : 'Вы вошли',
                        style: theme.textTheme.titleLarge,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 8),
                      if (user != null)
                        Text(
                          user.phone,
                          style: theme.textTheme.bodyLarge,
                          textAlign: TextAlign.center,
                        )
                      else if (_error != null)
                        Text(
                          _error!,
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.error,
                          ),
                          textAlign: TextAlign.center,
                        )
                      else
                        const CircularProgressIndicator(),
                      const SizedBox(height: 24),
                      Text(
                        'Лента, посты и профиль появятся следующими фичами.',
                        style: theme.textTheme.bodySmall,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 24),
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
