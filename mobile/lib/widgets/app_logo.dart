// Заглушка логотипа в заголовке экрана.
//
// Название словом («МояДача», «Лента») в заголовке не пишется: место
// в заголовке занимает логотип. Пока его нет, здесь рамка со словом —
// её видно ровно там, где потом встанет картинка (specs/000-ui.md,
// правило 14).
import 'package:flutter/material.dart';

import '../theme.dart';

class AppLogo extends StatelessWidget {
  const AppLogo({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      height: 32,
      padding: const EdgeInsets.symmetric(horizontal: AppGap.small),
      alignment: Alignment.center,
      decoration: BoxDecoration(
        border: Border.all(color: theme.colorScheme.outline),
        borderRadius: BorderRadius.circular(AppGap.small),
      ),
      child: Text(
        'логотип',
        style: theme.textTheme.bodySmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }
}
