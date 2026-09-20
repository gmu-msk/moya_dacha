// Экран, на котором пока ничего нет: ни постов, ни комментариев.
//
// Пустое состояние — часть спецификации фичи, а не то, что додумывается
// в коде (specs/TEMPLATE.md). Здесь его форма: значок, что произошло,
// и что человек может сделать дальше.
import 'package:flutter/material.dart';

import '../theme.dart';

class EmptyView extends StatelessWidget {
  const EmptyView({
    super.key,
    required this.icon,
    required this.title,
    this.hint,
    this.action,
  });

  final IconData icon;

  /// Что человек видит одной строкой: «Постов пока нет».
  final String title;

  /// Что с этим делать, если делать есть что.
  final String? hint;

  /// Что человек может сделать дальше: кнопка «Добавить пост» и то,
  /// что к ней относится.
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final hint = this.hint;
    final action = this.action;

    return Center(
      // При системном увеличении шрифта содержимое перестаёт помещаться
      // на экран — тогда оно прокручивается, а не обрезается.
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 72, color: theme.colorScheme.primary),
            const SizedBox(height: AppGap.medium),
            Text(
              title,
              style: theme.textTheme.titleLarge,
              textAlign: TextAlign.center,
            ),
            if (hint != null) ...[
              const SizedBox(height: AppGap.small),
              Text(
                hint,
                style: theme.textTheme.bodyMedium,
                textAlign: TextAlign.center,
              ),
            ],
            if (action != null) ...[
              const SizedBox(height: AppGap.large),
              action,
            ],
          ],
        ),
      ),
    );
  }
}
