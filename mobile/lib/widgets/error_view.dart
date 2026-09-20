// Ошибка на экране.
//
// Текст ошибки пишет сервис (mobile/lib/api.dart), здесь только показ.
// Два правила, за которыми этот виджет и заведён (ADR-0012): ошибку видно
// не только по цвету — рядом с текстом есть значок; и у ошибки есть
// действие, если повторить попытку вообще можно.
import 'package:flutter/material.dart';

import '../theme.dart';

class ErrorView extends StatelessWidget {
  const ErrorView({super.key, required this.message, this.onRetry});

  final String message;

  /// Повторить то, что не получилось. null — повторять нечего: человек
  /// сначала должен что-то изменить сам.
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.error_outline, color: theme.colorScheme.error),
        const SizedBox(height: AppGap.small),
        Text(
          message,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.error,
          ),
          textAlign: TextAlign.center,
        ),
        if (onRetry != null) ...[
          const SizedBox(height: AppGap.medium),
          OutlinedButton(onPressed: onRetry, child: const Text('Повторить')),
        ],
      ],
    );
  }
}
