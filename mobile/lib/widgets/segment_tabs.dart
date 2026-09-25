// Вкладки-сегменты на подложке: «Все» и «Подписки» в ленте, подписчики
// и подписки в списках (specs/012-follows.md, требование 20).
//
// Открытая вкладка — на полотне с кантом, закрытая — приглушённым
// текстом без подложки. Та же подложка, что у открытого раздела нижней
// панели: один приём выделения на всё приложение.
import 'package:flutter/material.dart';

import '../theme.dart';

/// Высота сегмента: не ниже цели касания, даже при крупном шрифте текст
/// растягивает его, а не обрезается.
const _segmentHeight = 44.0;

class SegmentTabs extends StatelessWidget {
  const SegmentTabs({
    super.key,
    required this.labels,
    required this.selected,
    required this.onSelect,
  });

  final List<String> labels;
  final int selected;
  final ValueChanged<int> onSelect;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppGap.medium,
        0,
        AppGap.medium,
        AppGap.small,
      ),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(AppGap.small),
        ),
        child: Padding(
          padding: const EdgeInsets.all(AppGap.tiny),
          child: Row(
            children: [
              for (var i = 0; i < labels.length; i++) ...[
                if (i > 0) const SizedBox(width: AppGap.tiny),
                Expanded(child: _segment(context, i)),
              ],
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
      child: Material(
        color: open ? scheme.surface : Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppShape.small),
          side: open
              ? BorderSide(
                  color: scheme.outlineVariant,
                  width: AppShape.hairline,
                )
              : BorderSide.none,
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: () => onSelect(index),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: _segmentHeight),
            child: Center(
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: AppGap.small,
                  vertical: AppGap.tiny,
                ),
                child: Text(
                  labels[index],
                  textAlign: TextAlign.center,
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: open ? scheme.onSurface : scheme.onSurfaceVariant,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
