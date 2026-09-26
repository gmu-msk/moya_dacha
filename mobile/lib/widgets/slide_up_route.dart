// Переход «выезжает снизу»: новый пост (макет, раунд 1 — одобрено).
//
// Экран поднимается снизу с плавным торможением; в заголовке — крестик,
// а не стрелка: это отдельное действие поверх всего (fullscreenDialog).
import 'package:flutter/material.dart';

import '../theme.dart';

class SlideUpRoute<T> extends PageRouteBuilder<T> {
  SlideUpRoute({required WidgetBuilder builder})
    : super(
        fullscreenDialog: true,
        opaque: true,
        transitionDuration: AppMotion.sheet,
        reverseTransitionDuration: AppMotion.standard,
        pageBuilder: (context, _, _) => builder(context),
        transitionsBuilder: (context, animation, _, child) {
          final curved = CurvedAnimation(
            parent: animation,
            curve: const Cubic(0.2, 0.9, 0.2, 1),
            reverseCurve: Curves.easeInCubic,
          );
          return SlideTransition(
            position: Tween(
              begin: const Offset(0, 1),
              end: Offset.zero,
            ).animate(curved),
            child: DecoratedBox(
              decoration: const BoxDecoration(
                boxShadow: [BoxShadow(color: Colors.black26, blurRadius: 40)],
              ),
              child: child,
            ),
          );
        },
      );
}
