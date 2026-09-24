// Кто увидит пост: specs/013-post-visibility.md.
//
// Три строки выбора — на экране нового поста и в меню своего поста —
// и отметка рядом со временем на карточке. Слова одни на всё
// приложение, поэтому живут здесь.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';

/// Значок видимости: земной шар, два человечка, замок.
IconData visibilityIcon(PostVisibility value) => switch (value) {
  PostVisibility.friends => Icons.people_outline,
  PostVisibility.me => Icons.lock_outline,
  _ => Icons.public,
};

/// Название пункта. У закрытого профиля «Всем» — это подписчики, и слово
/// честнее (требование 3).
String visibilityTitle(PostVisibility value, {required bool closed}) =>
    switch (value) {
      PostVisibility.friends => 'Друзьям',
      PostVisibility.me => 'Только мне',
      _ => closed ? 'Подписчикам' : 'Всем',
    };

/// Пояснение под названием (требование 8).
String visibilityHint(PostVisibility value, {required bool closed}) =>
    switch (value) {
      PostVisibility.friends => 'Те, с кем вы подписаны друг на друга',
      PostVisibility.me => 'Пост виден только вам',
      _ => closed ? 'Только ваши подписчики' : 'Все дачники в МоейДаче',
    };

/// Сообщение после смены видимости: «Теперь пост видят: друзья».
String visibilityChanged(PostVisibility value, {required bool closed}) =>
    'Теперь пост видят: ${switch (value) {
      PostVisibility.friends => 'друзья',
      PostVisibility.me => 'только вы',
      _ => closed ? 'подписчики' : 'все',
    }}';

/// Отметка на карточке рядом со временем: «друзьям», «только мне».
/// У поста для всех отметки нет (требование 7).
String? visibilityMark(PostVisibility value) => switch (value) {
  PostVisibility.friends => 'друзьям',
  PostVisibility.me => 'только мне',
  _ => null,
};

/// «Кто увидит»: три строки со значком, названием, пояснением и кружком
/// выбора; выбранная — на подложке (требование 8).
class VisibilityPicker extends StatelessWidget {
  const VisibilityPicker({
    super.key,
    required this.value,
    required this.closed,
    required this.onChanged,
    this.enabled = true,
  });

  final PostVisibility value;

  /// Закрыт ли профиль автора: от этого зависят слова первого пункта.
  final bool closed;
  final ValueChanged<PostVisibility> onChanged;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return RadioGroup<PostVisibility>(
      groupValue: value,
      onChanged: (picked) {
        if (enabled && picked != null) {
          onChanged(picked);
        }
      },
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.only(bottom: AppGap.tiny),
            child: Text('Кто увидит', style: theme.textTheme.titleMedium),
          ),
          for (final option in PostVisibility.values) _option(context, option),
        ],
      ),
    );
  }

  Widget _option(BuildContext context, PostVisibility option) {
    final theme = Theme.of(context);
    final picked = option == value;

    return Padding(
      padding: const EdgeInsets.only(top: AppGap.tiny),
      child: Material(
        color: picked
            ? theme.colorScheme.surfaceContainerHighest
            : Colors.transparent,
        borderRadius: BorderRadius.circular(AppGap.medium - 2),
        clipBehavior: Clip.antiAlias,
        child: RadioListTile<PostVisibility>(
          value: option,
          enabled: enabled,
          controlAffinity: ListTileControlAffinity.trailing,
          secondary: Icon(visibilityIcon(option)),
          title: Text(visibilityTitle(option, closed: closed)),
          subtitle: Text(visibilityHint(option, closed: closed)),
        ),
      ),
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
        AppGap.medium,
      ),
      child: VisibilityPicker(
        value: current,
        closed: closed,
        onChanged: (picked) => Navigator.of(context).pop(picked),
      ),
    ),
  ),
);
