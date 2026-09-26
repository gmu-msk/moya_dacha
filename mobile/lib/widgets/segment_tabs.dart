// Вкладки-сегменты: «Все» / «Подписки» в ленте, «Все» / «Друзья» /
// «Только я» в новом посте, списки подписок (specs/012-follows.md).
//
// Дорожка-«таблетка» на подложке, открытый сегмент — тёмный ползунок,
// который переезжает с лёгким перелётом (макет «Сад», 2a).
import 'package:flutter/material.dart';

import '../theme.dart';

/// Высота сегмента: не ниже цели касания; крупный шрифт его растягивает.
const _segmentHeight = 44.0;

class SegmentTabs extends StatelessWidget {
  const SegmentTabs({
    super.key,
    required this.labels,
    required this.selected,
    required this.onSelect,
    this.margin = const EdgeInsets.fromLTRB(
      AppGap.medium,
      0,
      AppGap.medium,
      AppGap.small,
    ),
  });

  final List<String> labels;
  final int selected;
  final ValueChanged<int> onSelect;
  final EdgeInsets margin;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final n = labels.length;
    final x = n == 1 ? 0.0 : -1 + 2 * selected / (n - 1);

    return Padding(
      padding: margin,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(AppShape.pill),
        ),
        child: Padding(
          padding: const EdgeInsets.all(AppGap.tiny),
          child: Stack(
            children: [
              Positioned.fill(
                child: AnimatedAlign(
                  alignment: Alignment(x, 0),
                  duration: AppMotion.standard,
                  curve: AppMotion.spring,
                  child: FractionallySizedBox(
                    widthFactor: 1 / n,
                    heightFactor: 1,
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: scheme.onSurface,
                        borderRadius: BorderRadius.circular(AppShape.pill),
                      ),
                    ),
                  ),
                ),
              ),
              Row(
                children: [
                  for (var i = 0; i < n; i++)
                    Expanded(child: _segment(context, i)),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _segment(BuildContext context, int index) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final open = index == selected;

    return Semantics(
      selected: open,
      button: true,
      child: InkWell(
        onTap: () => onSelect(index),
        customBorder: const StadiumBorder(),
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: _segmentHeight),
          child: Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: AppGap.small,
                vertical: AppGap.tiny,
              ),
              child: AnimatedDefaultTextStyle(
                duration: AppMotion.quick,
                style: (theme.textTheme.titleSmall ?? const TextStyle())
                    .copyWith(
                      color: open ? scheme.surface : scheme.onSurfaceVariant,
                    ),
                child: Text(labels[index], textAlign: TextAlign.center),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
