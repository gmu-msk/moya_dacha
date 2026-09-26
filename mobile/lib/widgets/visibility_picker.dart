// Кто увидит пост: specs/013-post-visibility.md.
//
// Переключатель из трёх сегментов — «Все», «Друзья», «Только я» — и
// пояснение под ним (макет «Сад», 2a). Тот же выбор в меню своего поста.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';
import 'segment_tabs.dart';

/// Порядок сегментов.
const visibilityOptions = [
  PostVisibility.all,
  PostVisibility.friends,
  PostVisibility.me,
];

IconData visibilityIcon(PostVisibility value) => switch (value) {
  PostVisibility.friends => Icons.people_outline,
  PostVisibility.me => Icons.lock_outline,
  _ => Icons.public,
};

/// Название сегмента. У закрытого профиля «Все» — это подписчики
/// (требование 3).
String visibilityTitle(PostVisibility value, {required bool closed}) =>
    switch (value) {
      PostVisibility.friends => 'Друзья',
      PostVisibility.me => 'Только я',
      _ => closed ? 'Подписчики' : 'Все',
    };

String visibilityHint(PostVisibility value, {required bool closed}) =>
    switch (value) {
      PostVisibility.friends => 'Те, с кем вы подписаны друг на друга',
      PostVisibility.me => 'Пост виден только вам',
      _ => closed ? 'Только ваши подписчики' : 'Все дачники в «Моей даче»',
    };

String visibilityChanged(PostVisibility value, {required bool closed}) =>
    'Теперь пост видят: ${switch (value) {
      PostVisibility.friends => 'друзья',
      PostVisibility.me => 'только вы',
      _ => closed ? 'подписчики' : 'все',
    }}';

/// Отметка на карточке рядом со временем. У поста для всех её нет.
String? visibilityMark(PostVisibility value) => switch (value) {
  PostVisibility.friends => 'друзьям',
  PostVisibility.me => 'только мне',
  _ => null,
};

class VisibilityPicker extends StatelessWidget {
  const VisibilityPicker({
    super.key,
    required this.value,
    required this.closed,
    required this.onChanged,
    this.enabled = true,
  });

  final PostVisibility value;
  final bool closed;
  final ValueChanged<PostVisibility> onChanged;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final index = visibilityOptions.indexOf(value).clamp(0, 2);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: AppGap.small),
          child: Text('Кто увидит', style: theme.textTheme.titleMedium),
        ),
        IgnorePointer(
          ignoring: !enabled,
          child: AnimatedOpacity(
            opacity: enabled ? 1 : 0.5,
            duration: AppMotion.quick,
            child: SegmentTabs(
              margin: EdgeInsets.zero,
              labels: [
                for (final option in visibilityOptions)
                  visibilityTitle(option, closed: closed),
              ],
              selected: index,
              onSelect: (i) => onChanged(visibilityOptions[i]),
            ),
          ),
        ),
        const SizedBox(height: AppGap.small),
        AnimatedSwitcher(
          duration: AppMotion.quick,
          child: Text(
            visibilityHint(value, closed: closed),
            key: ValueKey(value),
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }
}

/// Выбор видимости своего поста из его меню. `null` — человек передумал.
Future<PostVisibility?> pickVisibility(
  BuildContext context, {
  required PostVisibility current,
  required bool closed,
}) => showModalBottomSheet<PostVisibility>(
  context: context,
  showDragHandle: true,
  isScrollControlled: true,
  builder: (context) => SafeArea(
    child: Padding(
      padding: const EdgeInsets.fromLTRB(
        AppGap.medium,
        0,
        AppGap.medium,
        AppGap.large,
      ),
      child: VisibilityPicker(
        value: current,
        closed: closed,
        onChanged: (picked) => Navigator.of(context).pop(picked),
      ),
    ),
  ),
);
