// Экран знакомства.
//
// Показывается тому, у кого ещё нет имени, и не пускает дальше, пока имя
// не введено: безымянный автор в ленте — дыра, которую потом нечем
// закрыть (specs/002-profile.md).
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';

class IntroScreen extends StatefulWidget {
  const IntroScreen({super.key, required this.token, required this.onDone});

  final String token;

  /// Знакомство состоялось: дальше приложение живёт с этим профилем.
  final void Function(CurrentUser user) onDone;

  @override
  State<IntroScreen> createState() => _IntroScreenState();
}

class _IntroScreenState extends State<IntroScreen> {
  final TextEditingController _name = TextEditingController();
  final TextEditingController _about = TextEditingController();

  String? _error;
  bool _busy = false;

  @override
  void dispose() {
    _name.dispose();
    _about.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _error = null;
    });

    try {
      final user = await ProfileApi(apiClient(token: widget.token)).updateMe(
        ProfileUpdate(name: _name.text, about: _about.text),
      );
      debugPrint('$logMarker profile=introduced name=${user?.name}');
      if (user == null) {
        throw ApiException(200, 'Сервис не вернул профиль');
      }
      widget.onDone(user);
    } on Exception catch (error) {
      debugPrint('$logMarker profile=intro_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _error = errorMessage(error);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Знакомство')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(24),
          children: [
            Text('Как вас зовут?', style: theme.textTheme.headlineSmall),
            const SizedBox(height: 8),
            Text(
              'Под этим именем вас увидят соседи по ленте. Имя можно '
              'поменять в любой момент.',
              style: theme.textTheme.bodyMedium,
            ),
            const SizedBox(height: 24),
            TextField(
              controller: _name,
              autofocus: true,
              enabled: !_busy,
              textCapitalization: TextCapitalization.words,
              inputFormatters: [LengthLimitingTextInputFormatter(50)],
              decoration: const InputDecoration(
                labelText: 'Имя',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: _about,
              enabled: !_busy,
              inputFormatters: [LengthLimitingTextInputFormatter(200)],
              decoration: const InputDecoration(
                labelText: 'О себе (необязательно)',
                helperText: 'Например: три сотки под картошку',
                border: OutlineInputBorder(),
              ),
            ),
            if (_error != null) ...[
              const SizedBox(height: 16),
              Text(
                _error!,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.error,
                ),
              ),
            ],
            const SizedBox(height: 24),
            FilledButton(
              onPressed: _busy ? null : _save,
              child: const Text('Продолжить'),
            ),
          ],
        ),
      ),
    );
  }
}
