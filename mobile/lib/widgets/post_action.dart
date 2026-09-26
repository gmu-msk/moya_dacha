// Действие под постом: значок и число рядом с ним.
//
// Так выглядят и сердечко, и комментарии (ADR-0012). Подписи словом нет
// (specs/000-ui.md, правило 15). Числа нет, пока считать нечего.
import 'package:flutter/material.dart';

import '../theme.dart';

class PostAction extends StatelessWidget {
  const PostAction({
    super.key,
    this.icon,
    this.iconWidget,
    required this.count,
    required this.tooltip,
    required this.onPressed,
    this.color,
  }) : assert(icon != null || iconWidget != null);

  /// Значок из шрифта Material…
  final IconData? icon;

  /// …или свой: нарисованное облачко, сердечко с лепестками.
  final Widget? iconWidget;

  final int count;
  final String tooltip;
  final VoidCallback onPressed;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: TextButton(
        onPressed: onPressed,
        style: TextButton.styleFrom(
          padding: const EdgeInsets.symmetric(horizontal: AppGap.snug),
          foregroundColor: color,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            iconWidget ?? Icon(icon, color: color),
            if (count > 0) ...[
              const SizedBox(width: AppGap.small),
              Text('$count'),
            ],
          ],
        ),
      ),
    );
  }
}
