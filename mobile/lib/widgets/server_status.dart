// Состояние сервиса — точкой в заголовке экрана.
//
// Продуктовой ценности в нём нет: это отметка для владельца о том, дошло ли
// приложение до сервиса и жива ли база, — сквозной сценарий показа
// demo/stories/000-status. По ней же прогон в эмуляторе понимает, что
// приложение доехало до ответа сервиса. Раз это не для дачника, места на
// экране она почти не занимает (specs/000-ui.md, правило 13).
//
// Нажатие на точку открывает экран «Сервер»: адрес стенда вводится там,
// и попасть туда надо с любого экрана, в том числе до входа (ADR-0013).
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
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final alive = _alive;

    final String state;
    final Color color;
    if (alive == null) {
      state = 'Проверяю сервер…';
      color = theme.colorScheme.outline;
    } else if (alive) {
      state = 'Сервер отвечает, база жива';
      color = theme.colorScheme.primary;
    } else {
      state = 'Сервер не отвечает';
      color = theme.colorScheme.error;
    }

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Беда видна не одним цветом: рядом с красной точкой значок
        // (specs/000-ui.md, правило 7). Когда всё хорошо, значка нет —
        // хорошие новости места занимать не должны.
        if (alive == false)
          Icon(
            Icons.cloud_off,
            size: AppGap.medium,
            color: theme.colorScheme.error,
          ),
        IconButton(
          onPressed: _openServer,
          tooltip: '$state\n$apiBaseUrl',
          icon: Container(
            width: AppGap.small,
            height: AppGap.small,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
          ),
        ),
        IconButton(
          onPressed: _check,
          tooltip: 'Проверить ещё раз',
          icon: const Icon(Icons.refresh),
        ),
      ],
    );
  }
}
