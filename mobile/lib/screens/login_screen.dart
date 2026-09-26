// Экран входа: номер телефона, затем код из СМС (specs/001-auth.md)
// или из приглашения (specs/015-invites.md).
//
// Шаг номера — как был. Шаг кода — по макету «Сад» (2a): четыре клетки,
// номер в подзаголовке не переносится на другую строку.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';
import '../widgets/masked_input.dart';

abstract class AuthGateway {
  Future<AuthCodeAccepted> requestCode(String phone);

  Future<SessionCreated> signIn(String phone, String code);
}

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

  final Future<void> Function(SessionCreated session) onSignedIn;
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
  bool _byInvite = false;
  String? _error;
  String? _offline;
  int _wait = 0;
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

  void _phoneChanged(String digits) {
    setState(() {
      _error = null;
      if (_waitOnPhone && _wait > 0) {
        _ticker?.cancel();
        _wait = 0;
      }
    });
  }

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
        _code.clear();
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
              _sentTo(theme),
            ],
            const SizedBox(height: AppGap.large),
            if (_codeSent) ..._codeFields(theme) else ..._phoneFields(theme),
            if (offline != null) ...[
              const SizedBox(height: AppGap.medium),
              ErrorView(message: offline),
            ],
          ],
        ),
      ),
    );
  }

  /// «Мы отправили СМС-код на +7(915)234-56-78»: номер — одним куском,
  /// на дефисах он не рвётся.
  Widget _sentTo(ThemeData theme) {
    final style = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    return Text.rich(
      TextSpan(
        text: _byInvite
            ? 'Введите код из приглашения для '
            : 'Мы отправили СМС-код на ',
        children: [
          WidgetSpan(
            alignment: PlaceholderAlignment.baseline,
            baseline: TextBaseline.alphabetic,
            child: Text(
              _phone.text,
              softWrap: false,
              style: style?.copyWith(
                color: theme.colorScheme.onSurface,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
        ],
      ),
      style: style,
      textAlign: TextAlign.center,
    );
  }

  Widget _underField(
    ThemeData theme,
    String text, {
    bool alarming = true,
    TextAlign align = TextAlign.start,
  }) {
    return Padding(
      padding: const EdgeInsets.only(top: AppGap.small),
      child: Text(
        text,
        style: theme.textTheme.bodyMedium?.copyWith(
          color: alarming
              ? theme.colorScheme.error
              : theme.colorScheme.onSurfaceVariant,
        ),
        textAlign: align,
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
        boxes: true,
        done: _busy && _code.complete,
        onChanged: _codeChanged,
      ),
      if (_busy)
        _underField(
          theme,
          'Проверяем код…',
          alarming: false,
          align: TextAlign.center,
        )
      else if (error != null)
        _underField(theme, error, align: TextAlign.center),
      const SizedBox(height: AppGap.medium),
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
