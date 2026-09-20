// Экран входа: номер телефона, затем код из него (specs/001-auth.md).
//
// Регистрация и вход — одно действие, поэтому экран один: новый человек
// и вернувшийся проходят одинаковый путь.
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../widgets/server_status.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key, required this.onSignedIn});

  /// Вызывается, когда сервис выдал токен: дальше решает приложение.
  final Future<void> Function(SessionCreated session) onSignedIn;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final TextEditingController _phone = TextEditingController();
  final TextEditingController _code = TextEditingController();

  bool _codeSent = false;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _phone.dispose();
    _code.dispose();
    super.dispose();
  }

  Future<void> _requestCode() async {
    setState(() {
      _busy = true;
      _error = null;
    });

    try {
      await AuthApi(apiClient()).requestAuthCode(
        AuthCodeRequest(phone: _phone.text),
      );
      debugPrint('$logMarker auth=code_sent');
      if (!mounted) {
        return;
      }
      setState(() {
        _codeSent = true;
        _code.clear();
      });
    } on Exception catch (error) {
      debugPrint('$logMarker auth=code_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() => _error = errorMessage(error));
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  Future<void> _signIn() async {
    setState(() {
      _busy = true;
      _error = null;
    });

    try {
      final session = await AuthApi(apiClient()).createSession(
        SessionRequest(phone: _phone.text, code: _code.text),
      );
      if (session == null) {
        throw const FormatException('сервис ответил пустым телом');
      }
      debugPrint('$logMarker auth=signed_in new=${session.isNewUser}');
      await widget.onSignedIn(session);
    } on Exception catch (error) {
      debugPrint('$logMarker auth=sign_in_failed error=$error');
      if (!mounted) {
        return;
      }
      // Код истёк или попытки кончились — начинать надо с нового кода.
      final code = serviceErrorCode(error);
      setState(() {
        _error = errorMessage(error);
        if (code == 'code_expired' || code == 'too_many_attempts') {
          _codeSent = false;
        }
      });
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  void _changePhone() {
    setState(() {
      _codeSent = false;
      _error = null;
      _code.clear();
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('МояДача')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            children: [
              Expanded(
                child: SingleChildScrollView(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const SizedBox(height: 24),
                      Text(
                        _codeSent ? 'Введите код' : 'Вход по номеру телефона',
                        style: theme.textTheme.titleLarge,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        _codeSent
                            ? 'Мы отправили код на ${_phone.text}'
                            : 'Пароля нет: придёт код из четырёх цифр',
                        style: theme.textTheme.bodyMedium,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 24),
                      if (_codeSent) ..._codeFields() else ..._phoneFields(),
                      if (_error != null) ...[
                        const SizedBox(height: 16),
                        Text(
                          _error!,
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.error,
                          ),
                          textAlign: TextAlign.center,
                        ),
                      ],
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

  List<Widget> _phoneFields() {
    return [
      TextField(
        controller: _phone,
        autofocus: true,
        keyboardType: TextInputType.phone,
        decoration: const InputDecoration(
          labelText: 'Номер телефона',
          hintText: '+7 900 123-45-67',
          border: OutlineInputBorder(),
        ),
        onSubmitted: (_) {
          if (!_busy) {
            _requestCode();
          }
        },
      ),
      const SizedBox(height: 16),
      FilledButton(
        onPressed: _busy ? null : _requestCode,
        child: Text(_busy ? 'Отправляю…' : 'Получить код'),
      ),
    ];
  }

  List<Widget> _codeFields() {
    return [
      TextField(
        controller: _code,
        autofocus: true,
        keyboardType: TextInputType.number,
        inputFormatters: [
          FilteringTextInputFormatter.digitsOnly,
          const LengthLimitingTextInputFormatter(4),
        ],
        textAlign: TextAlign.center,
        style: const TextStyle(fontSize: 28, letterSpacing: 8),
        decoration: const InputDecoration(
          labelText: 'Код из четырёх цифр',
          border: OutlineInputBorder(),
        ),
        onSubmitted: (_) {
          if (!_busy) {
            _signIn();
          }
        },
      ),
      const SizedBox(height: 16),
      FilledButton(
        onPressed: _busy ? null : _signIn,
        child: Text(_busy ? 'Проверяю…' : 'Войти'),
      ),
      const SizedBox(height: 8),
      TextButton(
        onPressed: _busy ? null : _requestCode,
        child: const Text('Отправить код ещё раз'),
      ),
      TextButton(
        onPressed: _busy ? null : _changePhone,
        child: const Text('Другой номер'),
      ),
    ];
  }
}
