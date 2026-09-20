// Витрина общих виджетов: все состояния на одном экране.
//
// Макетов в проекте нет (ADR-0012), и проверять дизайн глазами всё равно
// надо. Этот экран заменяет макеты: на нём видно, как выглядят загрузка,
// пустой экран и ошибка, и что будет с ними в тёмной теме и при
// увеличенном системном шрифте. Показывается сценарием
// demo/stories/000-ui, в обычную сборку не попадает.
import 'package:flutter/material.dart';
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

class GalleryScreen extends StatefulWidget {
  const GalleryScreen({super.key});

  @override
  State<GalleryScreen> createState() => _GalleryScreenState();
}

class _GalleryScreenState extends State<GalleryScreen> {
  final TextEditingController _field = TextEditingController(text: '+7 900 ');

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
    final theme = Theme.of(context);

    return AppScreen(
      title: 'Витрина виджетов',
      // Витрина не ходит в сеть, строке состояния сервиса тут не место.
      showServerStatus: false,
      child: ListView(
        children: [
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
          const ErrorView(message: 'Код не подошёл'),

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
