// Экран входа: номер телефона, затем код из СМС (specs/001-auth.md)
// или из приглашения (specs/015-invites.md).
//
// Регистрация и вход — одно действие, поэтому экран один: новый человек
// и вернувшийся проходят одинаковый путь.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';
import '../widgets/masked_input.dart';

/// Поход в сервис за кодом и за сессией.
///
/// Отдельным слоем он существует ради проверок: экран живёт таймерами и
/// отказами сервиса, и без подмены сервиса их не проверить
/// (`mobile/test/login_screen_test.dart`).
abstract class AuthGateway {
  Future<AuthCodeAccepted> requestCode(String phone);

  Future<SessionCreated> signIn(String phone, String code);
}

/// Настоящий сервис.
class ApiAuthGateway implements AuthGateway {
  const ApiAuthGateway();

  @override
  Future<AuthCodeAccepted> requestCode(String phone) async {
    final accepted = await AuthApi(apiClient())
        .requestAuthCode(AuthCodeRequest(phone: phone));
    if (accepted == null) {
      throw const FormatException('сервис ответил пустым телом');
    }
    return accepted;
  }

  @override
  Future<SessionCreated> signIn(String phone, String code) async {
    final session = await AuthApi(apiClient())
        .createSession(SessionRequest(phone: phone, code: code));
    if (session == null) {
      throw const FormatException('сервис ответил пустым телом');
    }
    return session;
  }
}

class LoginScreen extends StatefulWidget {
  const LoginScreen({
    super.key,
    required this.onSignedIn,
    this.auth = const ApiAuthGateway(),
  });

  /// Вызывается, когда сервис выдал токен: дальше решает приложение.
  final Future<void> Function(SessionCreated session) onSignedIn;

  /// Сервис, у которого экран просит код и сессию.
  final AuthGateway auth;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final MaskedController _phone = MaskedController(
    mask: phoneMask,
    boldPrefix: 2,
  );
  final MaskedController _code = MaskedController(mask: codeMask);

  bool _codeSent = false;
  bool _busy = false;

  /// Код у человека в приглашении: сервис ничего не отправлял, и
  /// отправить ещё раз тоже нечего (specs/015-invites.md, требование 13).
  bool _byInvite = false;

  /// Что не так с введённым: стоит под полем, как и все ошибки поля
  /// (specs/000-ui.md, правило 16).
  String? _error;

  /// Беда связи: она не про поле, и место ей внизу экрана.
  String? _offline;

  /// Сколько секунд осталось до повторной отправки кода. Пока счётчик
  /// идёт, «Получить код» и «Отправить ещё раз» неактивны.
  int _wait = 0;

  /// Отказ пришёл на запрос кода, а не на вход: тогда счётчик виден
  /// сообщением под полем номера.
  bool _waitOnPhone = false;

  Timer? _ticker;

  @override
  void dispose() {
    _ticker?.cancel();
    _phone.dispose();
    _code.dispose();
    super.dispose();
  }

  void _countDown(int seconds) {
    _ticker?.cancel();
    _wait = seconds;
    if (seconds <= 0) {
      return;
    }
    _ticker = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      setState(() {
        _wait -= 1;
        if (_wait <= 0) {
          _wait = 0;
          timer.cancel();
        }
      });
    });
  }

  /// Номер меняют — от этого зависит кнопка, а прошлый отказ больше
  /// не про этот номер.
  void _phoneChanged(String digits) {
    setState(() {
      _error = null;
      if (_waitOnPhone && _wait > 0) {
        _ticker?.cancel();
        _wait = 0;
      }
    });
  }

  /// Код набран целиком — входим сами: отдельной кнопке тут делать нечего.
  void _codeChanged(String digits) {
    if (_error != null) {
      setState(() => _error = null);
    }
    if (_code.complete && !_busy) {
      _signIn();
    }
  }

  Future<void> _requestCode() async {
    setState(() {
      _busy = true;
      _error = null;
      _offline = null;
    });

    try {
      final accepted = await widget.auth.requestCode(_phone.text);
      debugPrint('$logMarker auth=code_sent');
      if (!mounted) {
        return;
      }
      setState(() {
        _codeSent = true;
        _byInvite = accepted.delivery == AuthCodeAcceptedDeliveryEnum.invite;
        _waitOnPhone = false;
        _code.clear();
        _countDown(accepted.resendAfter);
      });
    } on Exception catch (error) {
      debugPrint('$logMarker auth=code_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() => _failed(error, onPhone: !_codeSent));
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
      _offline = null;
    });

    try {
      final session = await widget.auth.signIn(_phone.text, _code.digits);
      debugPrint('$logMarker auth=signed_in new=${session.isNewUser}');
      await widget.onSignedIn(session);
    } on Exception catch (error) {
      debugPrint('$logMarker auth=sign_in_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _failed(error, onPhone: false);
        // Набирать поверх неверного кода нечего: поле чистое, и человек
        // сразу вводит следующий.
        _code.clear();
        // Код истёк или попытки кончились — нужен новый, и ждать нечего.
        final code = serviceErrorCode(error);
        if (code == 'code_expired' || code == 'too_many_attempts') {
          _ticker?.cancel();
          _wait = 0;
        }
      });
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  /// Разбор отказа: сервис сказал, что не так, или до него не дошли.
  void _failed(Exception error, {required bool onPhone}) {
    final message = serviceErrorCode(error) == null
        ? null
        : errorMessage(error);
    if (message == null) {
      _offline = errorMessage(error);
      return;
    }
    final seconds = retryAfterSeconds(error);
    if (seconds != null) {
      _waitOnPhone = onPhone;
      _countDown(seconds);
      return;
    }
    _error = message;
  }

  void _changePhone() {
    setState(() {
      _codeSent = false;
      _error = null;
      _offline = null;
      _ticker?.cancel();
      _wait = 0;
      // Номер вводится заново: человек вернулся сюда именно за этим.
      _phone.clear();
      _code.clear();
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final offline = _offline;

    return AppScreen(
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const SizedBox(height: AppGap.large),
            Text(
              _codeSent ? 'Введите код' : 'Вход в аккаунт',
              style: theme.textTheme.titleLarge,
              textAlign: TextAlign.center,
            ),
            if (_codeSent) ...[
              const SizedBox(height: AppGap.small),
              Text(
                _byInvite
                    ? 'Введите код из приглашения для ${_phone.text}'
                    : 'Мы отправили СМС-код на ${_phone.text}',
                style: theme.textTheme.bodyMedium,
                textAlign: TextAlign.center,
              ),
            ],
            const SizedBox(height: AppGap.large),
            if (_codeSent) ..._codeFields(theme) else ..._phoneFields(theme),
            if (offline != null) ...[
              const SizedBox(height: AppGap.medium),
              // Повторять нечего: следующий шаг человек делает сам —
              // исправляет номер или код и нажимает кнопку выше.
              ErrorView(message: offline),
            ],
          ],
        ),
      ),
    );
  }

  /// Строка под полем: что не так или чего ждём.
  Widget _underField(ThemeData theme, String text, {bool alarming = true}) {
    return Padding(
      padding: const EdgeInsets.only(top: AppGap.small),
      child: Text(
        text,
        style: theme.textTheme.bodyMedium?.copyWith(
          color: alarming
              ? theme.colorScheme.error
              : theme.colorScheme.onSurfaceVariant,
        ),
        textAlign: TextAlign.start,
      ),
    );
  }

  List<Widget> _phoneFields(ThemeData theme) {
    final waiting = _waitOnPhone && _wait > 0;
    final error = _error;

    return [
      MaskedField(
        controller: _phone,
        label: 'Номер телефона',
        autofocus: true,
        onChanged: _phoneChanged,
      ),
      if (waiting)
        _underField(
          theme,
          'На номер ${_phone.text} код уже был отправлен. Если номер введён '
          'верно, пожалуйста, выполните повторный запрос через $_wait сек',
        )
      else if (error != null)
        _underField(theme, error),
      const SizedBox(height: AppGap.medium),
      FilledButton(
        onPressed: _busy || waiting || !_phone.complete ? null : _requestCode,
        child: Text(_busy ? 'Отправляю…' : 'Получить код'),
      ),
    ];
  }

  List<Widget> _codeFields(ThemeData theme) {
    final error = _error;

    return [
      MaskedField(
        controller: _code,
        label: 'Код',
        autofocus: true,
        centered: true,
        textStyle: theme.textTheme.headlineSmall?.copyWith(letterSpacing: 8),
        onChanged: _codeChanged,
      ),
      if (_busy)
        _underField(theme, 'Проверяем код…', alarming: false)
      else if (error != null)
        _underField(theme, error),
      const SizedBox(height: AppGap.medium),
      // Второстепенность видна видом, а не размером: цели касания мельче
      // кнопки из темы не бывают (specs/000-ui.md, правило 8).
      if (!_byInvite)
        TextButton(
          onPressed: _busy || _wait > 0 ? null : _requestCode,
          child: Text(
            _wait > 0
                ? 'Отправить код ещё раз через $_wait сек'
                : 'Отправить код ещё раз',
          ),
        ),
      TextButton(onPressed: _changePhone, child: const Text('Другой номер')),
    ];
  }
}
