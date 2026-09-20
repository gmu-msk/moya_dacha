// Ожидание ответа.
//
// Крутилка сама по себе ничего не объясняет, поэтому рядом с ней всегда
// написано, чего ждём (ADR-0011).
import 'package:flutter/material.dart';

import '../theme.dart';

class LoadingView extends StatelessWidget {
  const LoadingView({super.key, this.label});

  /// Чего ждём: «Загружаю ленту…».
  final String? label;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final label = this.label;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        CircularProgressIndicator(semanticsLabel: label),
        if (label != null) ...[
          const SizedBox(height: AppGap.medium),
          Text(
            label,
            style: theme.textTheme.bodyMedium,
            textAlign: TextAlign.center,
          ),
        ],
      ],
    );
  }
}
