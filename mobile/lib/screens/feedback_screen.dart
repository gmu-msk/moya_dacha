// «Написать разработчику»: отзыв со скриншотом и список своих отзывов
// с тем, что с ними стало (specs/019-feedback.md, требования 14–17).
// Открывается из «Изменить профиль».
import 'package:device_info_plus/device_info_plus.dart';
import 'package:flutter/material.dart' hide Feedback;
import 'package:flutter/services.dart';
import 'package:http/http.dart' show MultipartFile;
import 'package:image_picker/image_picker.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../build_info.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';

class FeedbackScreen extends StatefulWidget {
  const FeedbackScreen({super.key, required this.token});

  final String token;

  @override
  State<FeedbackScreen> createState() => _FeedbackScreenState();
}

class _FeedbackScreenState extends State<FeedbackScreen> {
  final _text = TextEditingController();
  XFile? _screenshot;
  bool _busy = false;
  String? _error;
  String? _sent;

  List<Feedback>? _items;
  String? _loadError;

  FeedbackApi get _api => FeedbackApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    usage.screen('feedback');
    _load();
  }

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() => _loadError = null);
    try {
      final list = await _api.getMyFeedback();
      if (mounted) {
        setState(() => _items = list?.items ?? const []);
      }
    } on Exception catch (error) {
      if (mounted) {
        setState(() => _loadError = errorMessage(error));
      }
    }
  }

  Future<void> _pick() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (picked != null && mounted) {
      setState(() => _screenshot = picked);
    }
  }

  Future<void> _send() async {
    final text = _text.text.trim();
    if (text.isEmpty) {
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
      _sent = null;
    });
    try {
      final shot = _screenshot;
      final file = shot == null
          ? null
          : await MultipartFile.fromPath(
              'screenshot',
              shot.path,
              filename: shot.name,
            );
      final created = await _api.sendFeedback(
        text,
        screenshot: file,
        appVersion: await _version(),
        device: await _device(),
      );
      debugPrint('$logMarker feedback=sent');
      if (!mounted) {
        return;
      }
      setState(() {
        _text.clear();
        _screenshot = null;
        _sent = 'Спасибо! Отзыв отправлен.';
        if (created != null) {
          _items = [created, ...?_items];
        }
      });
    } on Exception catch (error) {
      if (mounted) {
        setState(() => _error = errorMessage(error));
      }
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  /// «1.0.0 (386900)» из сведений о сборке (specs/017-app-updates.md).
  static Future<String?> _version() async {
    final info = await loadBuildInfo();
    if (info.isDevelopment) {
      return 'сборка для разработки';
    }
    return '${info.version} (${info.build})';
  }

  /// «Google Pixel 7, Android 14». Не узнали — не беда: отзыв важнее.
  static Future<String?> _device() async {
    try {
      final android = await DeviceInfoPlugin().androidInfo;
      return '${android.manufacturer} ${android.model}, '
          'Android ${android.version.release}';
    } on Exception {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final shot = _screenshot;
    final error = _error;
    final sent = _sent;
    return AppScreen(
      title: 'Написать разработчику',
      child: ListView(
        children: [
          TextField(
            controller: _text,
            enabled: !_busy,
            minLines: 4,
            maxLines: 10,
            inputFormatters: [LengthLimitingTextInputFormatter(4000)],
            textCapitalization: TextCapitalization.sentences,
            onChanged: (_) => setState(() {}),
            decoration: const InputDecoration(
              hintText: 'Идея или что сломалось',
            ),
          ),
          const SizedBox(height: AppGap.small),
          if (shot == null)
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton.icon(
                onPressed: _busy ? null : _pick,
                icon: const Icon(Icons.image_outlined),
                label: const Text('Приложить скриншот'),
              ),
            )
          else
            Row(
              children: [
                const Icon(Icons.image_outlined),
                const SizedBox(width: AppGap.small),
                Expanded(
                  child: Text(
                    shot.name,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.bodyMedium,
                  ),
                ),
                IconButton(
                  tooltip: 'Убрать скриншот',
                  onPressed: _busy
                      ? null
                      : () => setState(() => _screenshot = null),
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
          const SizedBox(height: AppGap.small),
          FilledButton(
            onPressed: _busy || _text.text.trim().isEmpty ? null : _send,
            child: const Text('Отправить'),
          ),
          if (error != null) ...[
            const SizedBox(height: AppGap.medium),
            ErrorView(message: error),
          ],
          if (sent != null) ...[
            const SizedBox(height: AppGap.medium),
            Text(sent, style: theme.textTheme.bodyMedium),
          ],
          const SizedBox(height: AppGap.large),
          const Divider(),
          const SizedBox(height: AppGap.small),
          Text('Мои отзывы', style: theme.textTheme.titleMedium),
          const SizedBox(height: AppGap.small),
          ..._list(theme),
        ],
      ),
    );
  }

  List<Widget> _list(ThemeData theme) {
    final loadError = _loadError;
    if (loadError != null) {
      return [ErrorView(message: loadError, onRetry: _load)];
    }
    final items = _items;
    if (items == null) {
      return const [LoadingView()];
    }
    if (items.isEmpty) {
      return const [
        EmptyView(
          icon: Icons.forum_outlined,
          title: 'Здесь появятся ваши отзывы и что с ними стало.',
        ),
      ];
    }
    return [
      for (final item in items)
        ListTile(
          contentPadding: EdgeInsets.zero,
          title: Text(item.text, maxLines: 2, overflow: TextOverflow.ellipsis),
          subtitle: Text('${_date(item.createdAt)} · ${statusText(item)}'),
        ),
    ];
  }

  static String _date(DateTime moment) {
    final local = moment.toLocal();
    return '${local.day.toString().padLeft(2, '0')}.'
        '${local.month.toString().padLeft(2, '0')}.${local.year}';
  }
}

/// Статус отзыва словами (specs/019-feedback.md, требование 2).
String statusText(Feedback item) {
  final issue = item.issue;
  final number = issue == null ? '' : '#$issue';
  switch (item.status) {
    case FeedbackStatusEnum.sent:
      return 'Отправлено';
    case FeedbackStatusEnum.accepted:
      return 'Задача $number';
    case FeedbackStatusEnum.approved:
      return 'Одобрено, $number';
    case FeedbackStatusEnum.declined:
      return 'Не будем делать, $number';
    case FeedbackStatusEnum.done:
      return 'Сделано, $number';
    case FeedbackStatusEnum.released:
      return 'Вышло в сборке ${item.build ?? ''}';
  }
}
