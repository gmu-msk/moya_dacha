// Строка состояния сервиса внизу экрана.
//
// Продуктовой ценности в ней немного, но она показывает, дошло ли
// приложение до сервиса и жива ли база, — это сквозной сценарий показа
// demo/stories/000-status. По ней же прогон в эмуляторе понимает, что
// приложение доехало до ответа сервиса.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../app_scope.dart';
import '../screens/server_screen.dart';
import '../theme.dart';

class ServerStatus extends StatefulWidget {
  const ServerStatus({super.key});

  @override
  State<ServerStatus> createState() => _ServerStatusState();
}

class _ServerStatusState extends State<ServerStatus> {
  bool? _alive;

  @override
  void initState() {
    super.initState();
    _check();
  }

  Future<void> _check() async {
    setState(() => _alive = null);

    Health? health;
    try {
      health = await OperationsApi(apiClient()).getHealth();
    } on Exception catch (error) {
      debugPrint('$logMarker health=error error=$error');
    }

    if (health != null) {
      debugPrint('$logMarker health=${health.status}');
    }
    if (!mounted) {
      return;
    }
    setState(() => _alive = health != null);
  }

  /// Открыть экран «Сервер». Если адрес там сменили, приложение начинает
  /// заново: сессия прошлого сервера уже не годится.
  Future<void> _openServer() async {
    final restart = AppScope.of(context)?.restart;

    final changed = await Navigator.of(context)
        .push<bool>(MaterialPageRoute(builder: (_) => const ServerScreen()));

    if (changed == true && restart != null) {
      await restart();
    }
    if (!mounted) {
      return;
    }
    _check();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final alive = _alive;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (alive == null)
          Text('Проверяю сервер…', style: theme.textTheme.bodyMedium)
        else
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                alive ? Icons.check_circle_outline : Icons.cloud_off,
                color: alive
                    ? theme.colorScheme.primary
                    : theme.colorScheme.error,
              ),
              const SizedBox(width: AppGap.small),
              Flexible(
                child: Text(
                  alive ? 'Сервер отвечает, база жива' : 'Сервер не отвечает',
                  style: theme.textTheme.bodyMedium,
                ),
              ),
            ],
          ),
        // Адрес не просто показан, а открывает экран «Сервер»: одна и та
        // же сборка ходит на любой стенд, и попасть к выбору адреса надо
        // с любого экрана, в том числе до входа (ADR-0013).
        TextButton(
          onPressed: _openServer,
          child: Text(
            apiBaseUrl,
            style: theme.textTheme.bodySmall,
            textAlign: TextAlign.center,
          ),
        ),
        TextButton(onPressed: _check, child: const Text('Проверить ещё раз')),
      ],
    );
  }
}
