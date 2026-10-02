// Кто увидит пост: specs/013-post-visibility.md и
// specs/031-group-visibility.md.
//
// Вертикальный список: «Все», «Друзья», «Только я» и «Только участники
// <группа>» — по строке на группу. У строки значок, название, пояснение
// и кружок выбора, выбранная — на подложке. Тот же выбор в меню своего
// поста.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';

/// Кто увидит пост: одна из трёх видимостей или участники группы.
@immutable
class Audience {
  const Audience(this.visibility) : group = null;

  /// В запросе у поста группы видимость `me`: сервер без этой фичи
  /// выложит его «только мне», а не всем (031, требование 3).
  const Audience.group(GroupBrief this.group) : visibility = PostVisibility.me;

  final PostVisibility visibility;
  final GroupBrief? group;

  @override
  bool operator ==(Object other) =>
      other is Audience &&
      other.visibility == visibility &&
      other.group?.id == group?.id;

  @override
  int get hashCode => Object.hash(visibility, group?.id);
}

/// Кто видит пост по ответу сервиса: группа видимости главнее
/// старого поля (031, требование 11).
Audience audienceOf(Post post) {
  final group = post.visibilityGroup;
  return group == null ? Audience(post.visibility) : Audience.group(group);
}

/// Три видимости — первыми.
const visibilityOptions = [
  Audience(PostVisibility.all),
  Audience(PostVisibility.friends),
  Audience(PostVisibility.me),
];

IconData visibilityIcon(Audience value) => value.group != null
    ? Icons.groups_outlined
    : switch (value.visibility) {
        PostVisibility.friends => Icons.people_outline,
        PostVisibility.me => Icons.lock_outline,
        _ => Icons.public,
      };

/// Название строки. У закрытого профиля «Все» — это подписчики
/// (013, требование 3).
String visibilityTitle(Audience value, {required bool closed}) {
  final group = value.group;
  if (group != null) {
    return 'Только участники ${group.name}';
  }
  return switch (value.visibility) {
    PostVisibility.friends => 'Друзья',
    PostVisibility.me => 'Только я',
    _ => closed ? 'Подписчики' : 'Все',
  };
}

String visibilityHint(Audience value, {required bool closed}) =>
    value.group != null
    ? 'Пост увидят только участники группы'
    : switch (value.visibility) {
        PostVisibility.friends => 'Те, с кем вы подписаны друг на друга',
        PostVisibility.me => 'Пост виден только вам',
        _ => closed ? 'Только ваши подписчики' : 'Все дачники в «Моей даче»',
      };

String visibilityChanged(Audience value, {required bool closed}) {
  final group = value.group;
  if (group != null) {
    return 'Теперь пост видят: участники ${group.name}';
  }
  return 'Теперь пост видят: ${switch (value.visibility) {
    PostVisibility.friends => 'друзья',
    PostVisibility.me => 'только вы',
    _ => closed ? 'подписчики' : 'все',
  }}';
}

/// Отметка на карточке рядом со временем. У поста для всех её нет.
String? visibilityMark(Audience value) => value.group != null
    ? 'участникам группы'
    : switch (value.visibility) {
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
    this.groups = const [],
    this.enabled = true,
  });

  final Audience value;
  final bool closed;
  final ValueChanged<Audience> onChanged;

  /// Группы для «Только участники», уже в нужном порядке
  /// (031, требование 14).
  final List<GroupBrief> groups;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final options = [
      ...visibilityOptions,
      for (final group in groups) Audience.group(group),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: AppGap.small),
          child: Text('Кто увидит', style: theme.textTheme.titleMedium),
        ),
        AnimatedOpacity(
          opacity: enabled ? 1 : 0.5,
          duration: AppMotion.quick,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              for (final option in options)
                _AudienceRow(
                  option: option,
                  closed: closed,
                  selected: option == value,
                  onTap: enabled ? () => onChanged(option) : null,
                ),
            ],
          ),
        ),
      ],
    );
  }
}

class _AudienceRow extends StatelessWidget {
  const _AudienceRow({
    required this.option,
    required this.closed,
    required this.selected,
    required this.onTap,
  });

  final Audience option;
  final bool closed;
  final bool selected;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final radius = BorderRadius.circular(AppShape.medium);

    return Semantics(
      selected: selected,
      inMutuallyExclusiveGroup: true,
      child: AnimatedContainer(
        duration: AppMotion.quick,
        curve: AppMotion.ease,
        decoration: BoxDecoration(
          color: selected ? scheme.surfaceContainerHighest : null,
          borderRadius: radius,
        ),
        child: InkWell(
          borderRadius: radius,
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppGap.snug,
              vertical: AppGap.small,
            ),
            child: Row(
              children: [
                Icon(visibilityIcon(option), color: scheme.onSurfaceVariant),
                const SizedBox(width: AppGap.snug),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        visibilityTitle(option, closed: closed),
                        style: theme.textTheme.bodyLarge,
                      ),
                      Text(
                        visibilityHint(option, closed: closed),
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: AppGap.small),
                Icon(
                  selected
                      ? Icons.radio_button_checked
                      : Icons.radio_button_unchecked,
                  color: selected ? scheme.primary : scheme.outline,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Выбор видимости своего поста из его меню. `null` — человек передумал.
/// Групп в списке нет, кроме текущей (031, требования 13 и 18).
Future<Audience?> pickVisibility(
  BuildContext context, {
  required Audience current,
  required bool closed,
}) => showModalBottomSheet<Audience>(
  context: context,
  showDragHandle: true,
  isScrollControlled: true,
  builder: (context) => SafeArea(
    child: SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(
        AppGap.medium,
        0,
        AppGap.medium,
        AppGap.large,
      ),
      child: VisibilityPicker(
        value: current,
        closed: closed,
        groups: [?current.group],
        onChanged: (picked) => Navigator.of(context).pop(picked),
      ),
    ),
  ),
);

/// Порядок групп в «Кто увидит»: геогруппы, потом по интересам, внутри —
/// по названию без учёта регистра (031, требование 14).
List<Group> audienceGroupOrder(Iterable<Group> groups) =>
    groups.toList()..sort((a, b) {
      final kind = (a.kind == GroupKindEnum.place ? 0 : 1).compareTo(
        b.kind == GroupKindEnum.place ? 0 : 1,
      );
      return kind != 0
          ? kind
          : a.name.toLowerCase().compareTo(b.name.toLowerCase());
    });
