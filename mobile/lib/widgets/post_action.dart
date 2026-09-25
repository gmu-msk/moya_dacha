// Действие под постом: значок и число рядом с ним.
//
// Так выглядят и сердечко, и комментарии — второе применение, поэтому
// виджет общий (ADR-0012). Подписи словом у них нет: значок понятен и
// сам, а подписи занимали половину ширины экрана (specs/000-ui.md,
// правило 15). Числа нет, пока считать нечего.
import 'package:flutter/material.dart';

import '../theme.dart';

class PostAction extends StatelessWidget {
  const PostAction({
    super.key,
    required this.icon,
    required this.count,
    required this.tooltip,
    required this.onPressed,
    this.color,
  });

  final IconData icon;

  /// Сколько: отметок или комментариев. Ноль не показывается.
  final int count;

  /// Что это: подпись для долгого нажатия и для чтения с экрана.
  final String tooltip;

  final VoidCallback onPressed;

  /// Цвет значка, когда он должен отличаться: отмеченное сердечко.
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: TextButton(
        onPressed: onPressed,
        style: TextButton.styleFrom(
          padding: const EdgeInsets.symmetric(horizontal: AppGap.small),
          // Число того же цвета, что значок: у сердечка оба маковые.
          foregroundColor: color,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, color: color),
            if (count > 0) ...[
              const SizedBox(width: AppGap.tiny),
              Text('$count'),
            ],
          ],
        ),
      ),
    );
  }
}
