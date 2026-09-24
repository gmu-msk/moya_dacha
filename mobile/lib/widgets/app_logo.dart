// Логотип в заголовке главного экрана: штакетник, из-за которого
// выглядывает мак, и надпись «моя дача».
//
// Выбран владельцем на холсте макетов (specs/000-ui.md, правило 14 и
// раздел «Логотип»). Знак нарисован кодом, а не картинкой: краски
// берутся из темы, поэтому в тёмной теме и в песочнице витрины он
// перекрашивается вместе с экраном. Та же фигура — в иконке приложения
// (исходник mobile/tool/logo/znak.svg).
import 'package:flutter/material.dart';

import '../theme.dart';

class AppLogo extends StatelessWidget {
  const AppLogo({super.key});

  /// Знак выше строчных букв надписи: штакетник стоит вровень с ней,
  /// а мак выглядывает поверх.
  static const _markToText = 1.8;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final style = theme.textTheme.bodyLarge?.copyWith(
      color: colors.secondary,
      fontWeight: FontWeight.w500,
      height: 1,
    );
    final markSize = (style?.fontSize ?? 20) * _markToText;

    // Логотип — картинка, а не текст для чтения: системный размер шрифта
    // его не увеличивает, иначе при крупном шрифте он не влез бы в
    // заголовок. Для TalkBack он называется словом.
    return Semantics(
      label: 'МояДача',
      excludeSemantics: true,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          CustomPaint(
            size: Size.square(markSize),
            painter: FenceMarkPainter(
              fence: colors.secondary,
              poppy: colors.primary,
              poppyHeart: colors.onSurface,
              stem: AppBrand.stem(theme.brightness),
            ),
          ),
          const SizedBox(width: AppGap.small),
          Text('моя дача', style: style, textScaler: TextScaler.noScaling),
        ],
      ),
    );
  }
}

/// Знак: три штакетины с перекладиной и мак на стебле справа.
///
/// Координаты — в квадрате 100×100, как в mobile/tool/logo/znak.svg;
/// рисунок растягивается под размер холста.
class FenceMarkPainter extends CustomPainter {
  const FenceMarkPainter({
    required this.fence,
    required this.poppy,
    required this.poppyHeart,
    required this.stem,
  });

  final Color fence;
  final Color poppy;
  final Color poppyHeart;
  final Color stem;

  /// Левый край и верхушка каждой штакетины; средняя выше соседних.
  static const _pickets = [(12.0, 34.0), (41.0, 20.0), (70.0, 34.0)];
  static const _picketWidth = 18.0;
  static const _picketTip = 10.0;
  static const _ground = 92.0;

  /// Мак: четыре лепестка вокруг тёмной середины.
  static const _poppyCenter = Offset(82, 18);
  static const _petalShift = 7.44;
  static const _petalRadius = 8.64;
  static const _heartRadius = 4.08;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 100, size.height / 100);

    canvas.drawPath(
      Path()
        ..moveTo(84, 44)
        ..cubicTo(86, 36, 86, 28, 83, 20),
      Paint()
        ..color = stem
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3
        ..strokeCap = StrokeCap.round,
    );

    final fencePaint = Paint()..color = fence;
    for (final (x, top) in _pickets) {
      canvas.drawPath(
        Path()
          ..moveTo(x, top + _picketTip)
          ..lineTo(x + _picketWidth / 2, top)
          ..lineTo(x + _picketWidth, top + _picketTip)
          ..lineTo(x + _picketWidth, _ground)
          ..lineTo(x, _ground)
          ..close(),
        fencePaint,
      );
    }
    canvas.drawRRect(
      RRect.fromLTRBR(6, 60, 94, 67, const Radius.circular(2)),
      fencePaint,
    );

    final petalPaint = Paint()..color = poppy;
    for (final (dx, dy) in const [
      (-1.0, -1.0),
      (1.0, -1.0),
      (1.0, 1.0),
      (-1.0, 1.0),
    ]) {
      canvas.drawCircle(
        _poppyCenter + Offset(dx, dy) * _petalShift,
        _petalRadius,
        petalPaint,
      );
    }
    canvas.drawCircle(_poppyCenter, _heartRadius, Paint()..color = poppyHeart);
  }

  @override
  bool shouldRepaint(FenceMarkPainter oldDelegate) =>
      oldDelegate.fence != fence ||
      oldDelegate.poppy != poppy ||
      oldDelegate.poppyHeart != poppyHeart ||
      oldDelegate.stem != stem;
}
