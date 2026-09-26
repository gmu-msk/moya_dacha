// «О приложении»: версия, номер сборки, дата, коммит и «Что нового»
// (specs/017-app-updates.md). Открывается из «Изменить профиль».
import 'package:flutter/material.dart';

import '../api.dart';
import '../build_info.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/loading_view.dart';

class AboutScreen extends StatefulWidget {
  const AboutScreen({super.key, this.info});

  /// Сведения о сборке. Пусто — прочитать свои; витрина сценария
  /// показа подставляет образец.
  final BuildInfo? info;

  @override
  State<AboutScreen> createState() => _AboutScreenState();
}

class _AboutScreenState extends State<AboutScreen> {
  BuildInfo? _info;

  @override
  void initState() {
    super.initState();
    final info = widget.info;
    if (info != null) {
      _shown(info);
    } else {
      loadBuildInfo().then(_shown);
    }
  }

  void _shown(BuildInfo info) {
    debugPrint('$logMarker about=shown build=${info.build}');
    if (mounted) {
      setState(() => _info = info);
    }
  }

  @override
  Widget build(BuildContext context) {
    final info = _info;
    return AppScreen(
      title: 'О приложении',
      showServerStatus: false,
      child: info == null ? const Center(child: LoadingView()) : _body(info),
    );
  }

  Widget _body(BuildInfo info) {
    final theme = Theme.of(context);
    final date = info.date;

    Widget fact(String label, String value) => Padding(
      padding: const EdgeInsets.only(bottom: AppGap.medium),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: theme.textTheme.labelMedium),
          Text(value, style: theme.textTheme.bodyLarge),
        ],
      ),
    );

    return ListView(
      children: [
        Text('МояДача', style: theme.textTheme.headlineSmall),
        const SizedBox(height: AppGap.medium),
        if (info.isDevelopment)
          fact('Сборка', 'Сборка для разработки')
        else ...[
          fact('Версия', info.version),
          fact('Сборка', '${info.build}'),
          if (date != null) fact('Собрано', formatBuildDate(date)),
          if (info.commit.isNotEmpty) fact('Коммит', info.commit),
        ],
        if (info.whatsNew.isNotEmpty) ...[
          const Divider(),
          const SizedBox(height: AppGap.small),
          Text('Что нового', style: theme.textTheme.titleMedium),
          const SizedBox(height: AppGap.small),
          WhatsNewList(items: info.whatsNew),
        ],
      ],
    );
  }
}

/// Список «Что нового»: в окне после обновления и в «О приложении».
class WhatsNewList extends StatelessWidget {
  const WhatsNewList({super.key, required this.items});

  final List<String> items;

  @override
  Widget build(BuildContext context) {
    final style = Theme.of(context).textTheme.bodyMedium;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final item in items)
          Padding(
            padding: const EdgeInsets.only(bottom: AppGap.small),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('•  ', style: style),
                Expanded(child: Text(item, style: style)),
              ],
            ),
          ),
      ],
    );
  }
}

/// «26.09.2026 15:04» по местному времени.
String formatBuildDate(DateTime moment) {
  final local = moment.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.day)}.${two(local.month)}.${local.year} '
      '${two(local.hour)}:${two(local.minute)}';
}
