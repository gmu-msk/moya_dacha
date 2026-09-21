// Витрина общих виджетов: все состояния на одном экране.
//
// Макетов в проекте нет (ADR-0012), и проверять дизайн глазами всё равно
// надо. Этот экран заменяет макеты: на нём видно, как выглядят загрузка,
// пустой экран и ошибка, и что будет с ними в тёмной теме и при
// увеличенном системном шрифте. Показывается сценарием
// demo/stories/000-ui, в обычную сборку не попадает.
//
// Сверху у витрины песочница: величины темы крутятся прямо на телефоне,
// экран перерисовывается сразу. Это способ задать вид не словами —
// покрутить, нажать «Значения для темы» и прислать их в задачу; в
// приложение они попадают правкой констант в mobile/lib/theme.dart.
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/user_avatar.dart';

/// Высота коробки, в которой показан пустой экран: сам он занимает всё
/// свободное место, а в витрине место надо чем-то ограничить.
const _emptyViewBoxHeight = 360.0;

/// Выдуманный человек: витрине нужен кто-то, чтобы нарисовать аватар.
final _someone = CurrentUser(
  id: '00000000-0000-0000-0000-000000000000',
  phone: '+79000000000',
  createdAt: DateTime(2026),
  name: 'Пётр',
  about: '',
);

/// Цвета-семёна, между которыми переключается песочница. Первый —
/// нынешний цвет приложения.
const _seeds = <String, Color>{
  'Огород': Color(0xFF3F7D3F),
  'Трава': Color(0xFF6B7D2E),
  'Вода': Color(0xFF2E6F8E),
  'Земля': Color(0xFF8E5A2E),
  'Кирпич': Color(0xFFA5402E),
  'Слива': Color(0xFF7D3F6B),
};

class GalleryScreen extends StatefulWidget {
  const GalleryScreen({super.key});

  @override
  State<GalleryScreen> createState() => _GalleryScreenState();
}

class _GalleryScreenState extends State<GalleryScreen> {
  final TextEditingController _field = TextEditingController(text: '+7 900 ');

  /// Что показывает витрина сейчас. Пока ничего не крутили — ровно ту
  /// тему, с которой живёт приложение.
  ThemeTuning _tuning = const ThemeTuning();

  /// Светлая или тёмная. Пусто — как решила система: в приложении
  /// своего переключателя нет и не будет (ADR-0012), а здесь он нужен,
  /// чтобы не ходить за этим в настройки Android.
  Brightness? _brightness;

  @override
  void initState() {
    super.initState();
    // По этой строке прогон сценария понимает, что витрина открыта,
    // и снимает экран (demo/stories/000-ui/story.env).
    debugPrint('$logMarker gallery=shown');
  }

  @override
  void dispose() {
    _field.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Пока переключатель не трогали — та тема, которую уже выбрала
    // система: её выбрал MaterialApp выше по дереву.
    final brightness = _brightness ?? Theme.of(context).brightness;
    final theme = appTheme(brightness, tuning: _tuning);

    // Вся витрина живёт под собственной темой: настройки песочницы
    // действуют на неё и ни на что больше.
    return Theme(
      data: theme,
      child: Builder(builder: (context) => _body(context)),
    );
  }

  Widget _body(BuildContext context) {
    final theme = Theme.of(context);

    return AppScreen(
      title: 'Витрина виджетов',
      // Витрина не ходит в сеть, строке состояния сервиса тут не место.
      showServerStatus: false,
      child: ListView(
        children: [
          _playground(theme),

          _section(theme, 'Текст'),
          Text('Заголовок экрана', style: theme.textTheme.titleLarge),
          Text(
            'Основной текст, им написано почти всё',
            style: theme.textTheme.bodyMedium,
          ),
          Text(
            'Мелкий текст: адреса и подписи',
            style: theme.textTheme.bodySmall,
          ),

          _section(theme, 'Кнопки'),
          FilledButton(onPressed: () {}, child: const Text('Главное действие')),
          const SizedBox(height: AppGap.small),
          OutlinedButton(
            onPressed: () {},
            child: const Text('Второе действие'),
          ),
          const SizedBox(height: AppGap.small),
          const FilledButton(onPressed: null, child: Text('Недоступно')),

          _section(theme, 'Поле ввода'),
          TextField(
            controller: _field,
            keyboardType: TextInputType.phone,
            decoration: const InputDecoration(labelText: 'Номер телефона'),
          ),

          _section(theme, 'Аватар'),
          Row(
            children: [
              UserAvatar(user: _someone, radius: AvatarRadius.inBar),
              const SizedBox(width: AppGap.medium),
              UserAvatar(user: _someone, radius: AvatarRadius.onScreen),
              const SizedBox(width: AppGap.medium),
              UserAvatar(user: _someone, radius: AvatarRadius.inProfile),
            ],
          ),

          _section(theme, 'Ожидание'),
          const LoadingView(label: 'Загружаю ленту…'),

          _section(theme, 'Ошибка, которую можно повторить'),
          ErrorView(message: 'Сервер не отвечает', onRetry: () {}),

          _section(theme, 'Ошибка, которую повторять нечем'),
          const ErrorView(message: 'Неверный код'),

          _section(theme, 'Пустой экран'),
          SizedBox(
            height: _emptyViewBoxHeight,
            child: EmptyView(
              icon: Icons.photo_library_outlined,
              title: 'Постов пока нет',
              hint: 'Первый пост появится здесь',
              action: FilledButton(
                onPressed: () {},
                child: const Text('Добавить пост'),
              ),
            ),
          ),
          const SizedBox(height: AppGap.large),
        ],
      ),
    );
  }

  /// Песочница: свёрнута, пока её не открыли, — витрина остаётся
  /// витриной, и снимок сценария показывает то же, что показывал.
  Widget _playground(ThemeData theme) {
    return Card(
      margin: EdgeInsets.zero,
      child: ExpansionTile(
        leading: const Icon(Icons.tune),
        title: const Text('Песочница'),
        subtitle: Text(
          'Покрутить вид и прислать значения',
          style: theme.textTheme.bodySmall,
        ),
        childrenPadding: const EdgeInsets.fromLTRB(
          AppGap.medium,
          0,
          AppGap.medium,
          AppGap.medium,
        ),
        children: [
          SegmentedButton<Brightness?>(
            segments: const [
              ButtonSegment(value: null, label: Text('Как в системе')),
              ButtonSegment(value: Brightness.light, label: Text('Светлая')),
              ButtonSegment(value: Brightness.dark, label: Text('Тёмная')),
            ],
            selected: {_brightness},
            showSelectedIcon: false,
            onSelectionChanged: (selected) {
              setState(() => _brightness = selected.first);
            },
          ),

          const SizedBox(height: AppGap.medium),
          Align(
            alignment: Alignment.centerLeft,
            child: Text('Цвет', style: theme.textTheme.labelLarge),
          ),
          const SizedBox(height: AppGap.small),
          Wrap(
            spacing: AppGap.small,
            runSpacing: AppGap.small,
            children: [
              for (final seed in _seeds.entries)
                ChoiceChip(
                  label: Text(seed.key),
                  avatar: CircleAvatar(backgroundColor: seed.value),
                  selected: _tuning.seed == seed.value,
                  onSelected: (_) {
                    setState(
                      () => _tuning = _tuning.copyWith(seed: seed.value),
                    );
                  },
                ),
            ],
          ),

          _slider(
            theme,
            label: 'Размер текста',
            value: _tuning.fontScale,
            min: 1.0,
            max: 1.6,
            divisions: 12,
            format: (v) => '×${v.toStringAsFixed(2)}',
            onChanged: (v) =>
                setState(() => _tuning = _tuning.copyWith(fontScale: v)),
          ),
          _slider(
            theme,
            label: 'Высота кнопки',
            value: _tuning.tapTargetHeight,
            min: 40,
            max: 80,
            divisions: 10,
            format: (v) => '${v.round()} dp',
            onChanged: (v) =>
                setState(() => _tuning = _tuning.copyWith(tapTargetHeight: v)),
          ),
          _slider(
            theme,
            label: 'Скругление полей',
            value: _tuning.inputRadius,
            min: 0,
            max: 28,
            divisions: 14,
            format: (v) => '${v.round()} dp',
            onChanged: (v) =>
                setState(() => _tuning = _tuning.copyWith(inputRadius: v)),
          ),

          const SizedBox(height: AppGap.medium),
          Row(
            children: [
              Expanded(
                child: FilledButton(
                  onPressed: () => _showValues(context),
                  child: const Text('Значения для темы'),
                ),
              ),
              const SizedBox(width: AppGap.small),
              OutlinedButton(
                onPressed: () {
                  setState(() {
                    _tuning = const ThemeTuning();
                    _brightness = null;
                  });
                },
                child: const Text('Сбросить'),
              ),
            ],
          ),
          const SizedBox(height: AppGap.small),
          Text(
            'Отступы между блоками отсюда не крутятся: они подставляются '
            'при сборке. Если тесно или просторно — так и напишите словами.',
            style: theme.textTheme.bodySmall,
          ),
        ],
      ),
    );
  }

  Widget _slider(
    ThemeData theme, {
    required String label,
    required double value,
    required double min,
    required double max,
    required int divisions,
    required String Function(double) format,
    required ValueChanged<double> onChanged,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: AppGap.small),
        Text('$label — ${format(value)}', style: theme.textTheme.labelLarge),
        Slider(
          value: value,
          min: min,
          max: max,
          divisions: divisions,
          label: format(value),
          onChanged: onChanged,
        ),
      ],
    );
  }

  /// Что накрутили — в том виде, в каком это ложится в theme.dart.
  /// Кнопка «Скопировать» нужна, чтобы отправить это в задачу с телефона.
  void _showValues(BuildContext context) {
    final values = _tuning.asThemeConstants();

    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Значения для темы'),
        content: SingleChildScrollView(
          child: SelectableText(
            values,
            style: const TextStyle(fontFamily: 'monospace'),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Закрыть'),
          ),
          FilledButton(
            onPressed: () async {
              await Clipboard.setData(ClipboardData(text: values));
              if (context.mounted) {
                Navigator.of(context).pop();
              }
            },
            child: const Text('Скопировать'),
          ),
        ],
      ),
    );
  }

  Widget _section(ThemeData theme, String title) {
    return Padding(
      padding: const EdgeInsets.only(top: AppGap.large, bottom: AppGap.small),
      child: Text(
        title,
        style: theme.textTheme.titleMedium?.copyWith(
          color: theme.colorScheme.primary,
        ),
      ),
    );
  }
}
