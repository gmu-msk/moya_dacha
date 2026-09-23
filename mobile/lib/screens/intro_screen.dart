// Экран знакомства.
//
// Показывается тому, кто ещё не выбрал никнейм, и не пускает дальше,
// пока не выберет: никнеймом человек подписан везде (specs/010-nicknames.md).
// Полное имя и «о себе» — здесь же, но необязательны (specs/002-profile.md).
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';

class IntroScreen extends StatefulWidget {
  const IntroScreen({
    super.key,
    required this.token,
    required this.user,
    required this.onDone,
  });

  final String token;

  /// Каким человек пришёл: у того, кто завёлся до никнеймов, уже есть
  /// имя и «о себе», и терять их нельзя.
  final CurrentUser user;

  /// Знакомство состоялось: дальше приложение живёт с этим профилем.
  final void Function(CurrentUser user) onDone;

  @override
  State<IntroScreen> createState() => _IntroScreenState();
}

class _IntroScreenState extends State<IntroScreen> {
  final TextEditingController _nickname = TextEditingController();
  late final TextEditingController _name = TextEditingController(
    text: widget.user.name,
  );
  late final TextEditingController _about = TextEditingController(
    text: widget.user.about,
  );

  /// Ошибка никнейма — под его полем, остальные — под формой.
  String? _nicknameError;
  String? _error;
  bool _busy = false;

  @override
  void dispose() {
    _nickname.dispose();
    _name.dispose();
    _about.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _nicknameError = null;
      _error = null;
    });

    final api = ProfileApi(apiClient(token: widget.token));
    try {
      // Сначала никнейм: он может оказаться занят, и тогда имя
      // сохранять незачем. Повторная отправка своего же никнейма —
      // не ошибка, так что после сбоя на втором шаге можно просто
      // нажать кнопку ещё раз.
      await api.setNickname(NicknameUpdate(nickname: _nickname.text));
      final user = await api.updateMe(
        ProfileUpdate(name: _name.text, about: _about.text),
      );
      debugPrint('$logMarker profile=introduced nickname=${user?.nickname}');
      if (user == null) {
        throw ApiException(200, 'Сервис не вернул профиль');
      }
      widget.onDone(user);
    } on Exception catch (error) {
      debugPrint('$logMarker profile=intro_failed error=$error');
      if (!mounted) {
        return;
      }
      final code = serviceErrorCode(error);
      setState(() {
        _busy = false;
        if (code == 'invalid_nickname' || code == 'nickname_taken') {
          _nicknameError = errorMessage(error);
        } else {
          _error = errorMessage(error);
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    final error = _error;

    return AppScreen(
      title: 'Знакомство',
      // Знакомство — разговор с человеком, состояние сервиса тут лишнее.
      showServerStatus: false,
      child: ListView(
        children: [
          Text('Придумайте никнейм', style: theme.textTheme.headlineSmall),
          const SizedBox(height: AppGap.small),
          Text(
            'Под ним вас увидят соседи по ленте. Никнейм можно поменять '
            'потом в профиле.',
            style: theme.textTheme.bodyMedium,
          ),
          const SizedBox(height: AppGap.large),
          TextField(
            controller: _nickname,
            autofocus: true,
            enabled: !_busy,
            // Латинская раскладка без подсказок и автозамены: никнейм —
            // не слово, исправлять его клавиатуре нечего.
            keyboardType: TextInputType.visiblePassword,
            autocorrect: false,
            enableSuggestions: false,
            inputFormatters: [LengthLimitingTextInputFormatter(20)],
            decoration: InputDecoration(
              labelText: 'Никнейм',
              helperText: 'Латиница, цифры и _, от 3 до 20 символов',
              errorText: _nicknameError,
              errorMaxLines: 2,
            ),
          ),
          const SizedBox(height: AppGap.medium),
          TextField(
            controller: _name,
            enabled: !_busy,
            textCapitalization: TextCapitalization.words,
            inputFormatters: [LengthLimitingTextInputFormatter(50)],
            decoration: const InputDecoration(
              labelText: 'Полное имя (необязательно)',
              helperText: 'Его видно в вашем профиле под никнеймом',
            ),
          ),
          const SizedBox(height: AppGap.medium),
          TextField(
            controller: _about,
            enabled: !_busy,
            inputFormatters: [LengthLimitingTextInputFormatter(200)],
            decoration: const InputDecoration(
              labelText: 'О себе (необязательно)',
              helperText: 'Например: три сотки под картошку',
            ),
          ),
          if (error != null) ...[
            const SizedBox(height: AppGap.medium),
            // Повторять нечего: человек исправляет поле и нажимает кнопку.
            ErrorView(message: error),
          ],
          const SizedBox(height: AppGap.large),
          FilledButton(
            onPressed: _busy ? null : _save,
            child: const Text('Продолжить'),
          ),
        ],
      ),
    );
  }
}
