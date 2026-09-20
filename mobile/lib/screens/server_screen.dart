// Экран «Сервер»: к какому стенду подключено приложение.
//
// Одна и та же тестовая сборка из PR годится для любого стенда — адрес
// задаётся здесь, а не при сборке (ADR-0013). Адрес временного стенда
// приходит комментарием в PR, его сюда и вставляют.
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../server.dart';
import '../session.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';

class ServerScreen extends StatefulWidget {
  const ServerScreen({super.key});

  @override
  State<ServerScreen> createState() => _ServerScreenState();
}

class _ServerScreenState extends State<ServerScreen> {
  late final TextEditingController _url = TextEditingController(
    text: apiBaseUrl,
  );

  String? _error;
  String? _checked;
  bool _busy = false;

  @override
  void dispose() {
    _url.dispose();
    super.dispose();
  }

  /// Вставить адрес из буфера: с телефона его копируют из комментария в PR.
  Future<void> _paste() async {
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    final text = data?.text;
    if (text == null || text.trim().isEmpty || !mounted) {
      return;
    }
    setState(() {
      _url.text = text.trim();
      _error = null;
      _checked = null;
    });
  }

  /// Спросить у адреса `/health` до того, как его сохранять: ошибиться
  /// в адресе легко, а искать причину потом — по немому экрану входа.
  Future<void> _check() async {
    final url = normalizeServerUrl(_url.text);
    if (url == null) {
      setState(() {
        _error = 'Не похоже на адрес сервера';
        _checked = null;
      });
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
      _checked = null;
    });

    try {
      await OperationsApi(ApiClient(basePath: url)).getHealth();
      debugPrint('$logMarker server=check_ok url=$url');
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _url.text = url;
        _checked = 'Сервер по этому адресу отвечает';
      });
    } on Exception catch (error) {
      debugPrint('$logMarker server=check_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _error = 'По этому адресу никто не ответил. Стенд поднят?';
      });
    }
  }

  /// Сохранить адрес. Сессия принадлежит тому стенду, на котором её выдали,
  /// поэтому при смене адреса вход начинается заново.
  Future<void> _save() async {
    final url = normalizeServerUrl(_url.text);
    if (url == null) {
      setState(() {
        _error = 'Не похоже на адрес сервера';
        _checked = null;
      });
      return;
    }

    setState(() => _busy = true);
    await ServerStore().write(url);
    await SessionStore().clear();
    debugPrint('$logMarker server=saved url=$url');
    if (!mounted) {
      return;
    }
    Navigator.of(context).pop(true);
  }

  /// Вернуться к адресу, зашитому при сборке: так приложение снова работает
  /// с эмулятором, не требуя вспоминать адрес.
  Future<void> _reset() async {
    setState(() => _busy = true);
    await ServerStore().clear();
    await SessionStore().clear();
    debugPrint('$logMarker server=reset');
    if (!mounted) {
      return;
    }
    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    final error = _error;
    final checked = _checked;

    return AppScreen(
      title: 'Сервер',
      // Строка состояния внизу спрашивает нынешний адрес — на этом экране
      // она только путала бы: адрес здесь как раз и меняют.
      showServerStatus: false,
      child: ListView(
        children: [
          Text(
            'К какому серверу подключено приложение',
            style: theme.textTheme.headlineSmall,
          ),
          const SizedBox(height: AppGap.small),
          Text(
            'Адрес временного стенда приходит комментарием в PR. Скопируйте '
            'его и вставьте сюда — приложение пересобирать не нужно.',
            style: theme.textTheme.bodyMedium,
          ),
          const SizedBox(height: AppGap.large),
          TextField(
            controller: _url,
            enabled: !_busy,
            autocorrect: false,
            keyboardType: TextInputType.url,
            decoration: const InputDecoration(
              labelText: 'Адрес сервера',
              helperText: 'Например: https://зелёный-огурец.trycloudflare.com',
              helperMaxLines: 2,
            ),
          ),
          const SizedBox(height: AppGap.medium),
          Wrap(
            spacing: AppGap.small,
            runSpacing: AppGap.small,
            children: [
              OutlinedButton.icon(
                onPressed: _busy ? null : _paste,
                icon: const Icon(Icons.content_paste),
                label: const Text('Вставить'),
              ),
              OutlinedButton.icon(
                onPressed: _busy ? null : _check,
                icon: const Icon(Icons.wifi_tethering),
                label: const Text('Проверить'),
              ),
            ],
          ),
          if (checked != null) ...[
            const SizedBox(height: AppGap.medium),
            Row(
              children: [
                Icon(
                  Icons.check_circle_outline,
                  color: theme.colorScheme.primary,
                ),
                const SizedBox(width: AppGap.small),
                Flexible(
                  child: Text(checked, style: theme.textTheme.bodyMedium),
                ),
              ],
            ),
          ],
          if (error != null) ...[
            const SizedBox(height: AppGap.medium),
            // Повторять нечего: человек сначала правит адрес.
            ErrorView(message: error),
          ],
          const SizedBox(height: AppGap.large),
          FilledButton(
            onPressed: _busy ? null : _save,
            child: const Text('Сохранить'),
          ),
          const SizedBox(height: AppGap.small),
          Text(
            'После смены адреса вход придётся повторить: сессия живёт на том '
            'сервере, где её выдали.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: AppGap.medium),
          TextButton(
            onPressed: _busy ? null : _reset,
            child: const Text('Вернуть адрес по умолчанию'),
          ),
          Text(
            apiBaseUrlDefault,
            style: theme.textTheme.bodySmall,
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}
