// Нижняя панель: specs/011-bottom-bar.md.
//
// Четыре кнопки без подписей — «Лента», «Новый пост», «Уведомления»,
// «Профиль» (specs/014-notifications.md, требование 9). Лента, уведомления
// и профиль — разделы, новый пост — действие. Значки одного цвета,
// нарисованы кодом: штакетник, плюс в квадрате, колокольчик, человечек.
// Непрочитанное — маковая точка на колокольчике. Открытый раздел стоит на
// тёплой подложке и залит — вариант «Подложка» с холста макетов
// (требование 10).
import 'package:flutter/material.dart';

import '../theme.dart';

/// Разделы главного экрана.
enum HomeTab { feed, notifications, profile }

/// Высота панели без системного отступа снизу. Материаловая с подписями —
/// 80; без подписей хватает цели касания из темы (specs/000-ui.md,
/// правило 8) и чуть воздуха.
const bottomBarHeight = 60.0;

/// Размер значков панели.
const _iconSize = 26.0;

/// Подложка открытого раздела: шире значка, скругление как у карточки,
/// уменьшенной вдвое.
const _indicatorSize = Size(64, 40);
const _indicatorRadius = 14.0;

class AppBottomBar extends StatelessWidget {
  const AppBottomBar({
    super.key,
    required this.tab,
    required this.onSelect,
    required this.onNewPost,
    this.unread = false,
  });

  /// Открытый раздел.
  final HomeTab tab;

  /// Касание раздела, в том числе уже открытого: тогда раздел
  /// возвращается к самому верху (требование 4).
  final void Function(HomeTab tab) onSelect;

  /// «Новый пост» — не раздел, а действие (требование 5).
  final VoidCallback onNewPost;

  /// Есть ли непрочитанное: точка на колокольчике
  /// (specs/014-notifications.md, требование 5).
  final bool unread;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;

    return DecoratedBox(
      // Кант сверху — как у карточки поста; тени нет (требование 12).
      decoration: BoxDecoration(
        color: colors.surface,
        border: Border(
          top: BorderSide(
            color: colors.outlineVariant,
            width: AppShape.hairline,
          ),
        ),
      ),
      child: SafeArea(
        top: false,
        child: SizedBox(
          height: bottomBarHeight,
          child: Row(
            children: [
              _BarButton(
                label: 'Лента',
                selected: tab == HomeTab.feed,
                onTap: () => onSelect(HomeTab.feed),
                painter: (color, filled) =>
                    FenceIconPainter(color: color, filled: filled),
              ),
              _BarButton(
                label: 'Новый пост',
                onTap: onNewPost,
                painter: (color, _) => NewPostIconPainter(color: color),
              ),
              _BarButton(
                label: unread ? 'Уведомления, есть новые' : 'Уведомления',
                selected: tab == HomeTab.notifications,
                onTap: () => onSelect(HomeTab.notifications),
                painter: (color, filled) => BellIconPainter(
                  color: color,
                  filled: filled,
                  dot: unread ? colors.primary : null,
                  dotBorder: colors.surface,
                ),
              ),
              _BarButton(
                label: 'Профиль',
                selected: tab == HomeTab.profile,
                onTap: () => onSelect(HomeTab.profile),
                painter: (color, filled) =>
                    PersonIconPainter(color: color, filled: filled),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Кнопка панели. Подписи на экране нет, но она есть у TalkBack и
/// всплывает при долгом нажатии.
class _BarButton extends StatelessWidget {
  const _BarButton({
    required this.label,
    required this.onTap,
    required this.painter,
    this.selected,
  });

  final String label;
  final VoidCallback onTap;
  final CustomPainter Function(Color color, bool filled) painter;

  /// Открыт ли раздел; у действия — пусто.
  final bool? selected;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final selected = this.selected ?? false;

    return Expanded(
      child: Semantics(
        button: true,
        selected: this.selected,
        label: label,
        onTap: onTap,
        excludeSemantics: true,
        child: Tooltip(
          message: label,
          child: InkResponse(
            onTap: onTap,
            containedInkWell: true,
            highlightShape: BoxShape.rectangle,
            child: Center(
              child: AnimatedContainer(
                duration: kThemeAnimationDuration,
                width: _indicatorSize.width,
                height: _indicatorSize.height,
                decoration: BoxDecoration(
                  // Открытый раздел отличается и подложкой, и заливкой
                  // значка — не одним цветом (specs/000-ui.md, правило 7).
                  color: selected
                      ? colors.surfaceContainerHighest
                      : Colors.transparent,
                  borderRadius: BorderRadius.circular(_indicatorRadius),
                ),
                child: Center(
                  child: CustomPaint(
                    size: const Size.square(_iconSize),
                    painter: painter(colors.onSurface, selected),
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

/// Значки рисуются в квадрате 24×24, как материаловые, и растягиваются
/// под размер холста.
abstract class _BarIconPainter extends CustomPainter {
  const _BarIconPainter({required this.color, this.filled = false});

  final Color color;
  final bool filled;

  static const _stroke = 1.7;

  Paint get fill => Paint()..color = color;

  Paint get stroke => Paint()
    ..color = color
    ..style = PaintingStyle.stroke
    ..strokeWidth = _stroke
    ..strokeJoin = StrokeJoin.round
    ..strokeCap = StrokeCap.round;

  void paintIcon(Canvas canvas);

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 24, size.height / 24);
    paintIcon(canvas);
  }

  @override
  bool shouldRepaint(_BarIconPainter oldDelegate) =>
      oldDelegate.color != color || oldDelegate.filled != filled;
}

/// Лента — штакетник: три штакетины и перекладина, как в логотипе.
class FenceIconPainter extends _BarIconPainter {
  const FenceIconPainter({required super.color, super.filled});

  /// Левый край и верхушка каждой штакетины; средняя выше соседних.
  static const _pickets = [(3.5, 6.5), (10.0, 4.0), (16.5, 6.5)];
  static const _width = 4.0;
  static const _tip = 2.5;
  static const _ground = 20.5;
  static const _railTop = 12.5;
  static const _railBottom = 15.5;

  @override
  void paintIcon(Canvas canvas) {
    final pickets = Path();
    for (final (x, top) in _pickets) {
      pickets
        ..moveTo(x, top + _tip)
        ..lineTo(x + _width / 2, top)
        ..lineTo(x + _width, top + _tip)
        ..lineTo(x + _width, _ground)
        ..lineTo(x, _ground)
        ..close();
    }

    if (filled) {
      canvas
        ..drawPath(pickets, fill)
        ..drawRRect(
          RRect.fromLTRBR(
            2,
            _railTop,
            22,
            _railBottom,
            const Radius.circular(0.8),
          ),
          fill,
        );
      return;
    }

    // Контуром перекладина рисуется только в просветах между штакетинами
    // и по краям: поперёк самих штакетин она бы их перечеркнула.
    final rail = Path();
    for (final y in const [_railTop, _railBottom]) {
      for (final (from, to) in const [
        (2.0, 3.5),
        (7.5, 10.0),
        (14.0, 16.5),
        (20.5, 22.0),
      ]) {
        rail
          ..moveTo(from, y)
          ..lineTo(to, y);
      }
    }
    canvas
      ..drawPath(pickets, stroke)
      ..drawPath(rail, stroke);
  }
}

/// Новый пост — плюс в квадрате со скруглением. Это действие, а не
/// раздел, поэтому залитым он не бывает.
class NewPostIconPainter extends _BarIconPainter {
  const NewPostIconPainter({required super.color});

  @override
  void paintIcon(Canvas canvas) {
    canvas
      ..drawRRect(
        RRect.fromLTRBR(3.5, 3.5, 20.5, 20.5, const Radius.circular(5)),
        stroke,
      )
      ..drawPath(
        Path()
          ..moveTo(12, 8)
          ..lineTo(12, 16)
          ..moveTo(8, 12)
          ..lineTo(16, 12),
        stroke,
      );
  }
}

/// Уведомления — колокольчик. Точка непрочитанного — маковый кружок
/// в правом верхнем углу с каймой цвета полотна (требование 9).
class BellIconPainter extends _BarIconPainter {
  const BellIconPainter({
    required super.color,
    super.filled,
    this.dot,
    this.dotBorder,
  });

  /// Цвет точки или null, если непрочитанного нет.
  final Color? dot;
  final Color? dotBorder;

  @override
  void paintIcon(Canvas canvas) {
    final bell = Path()
      ..moveTo(6, 16.5)
      ..lineTo(6, 11)
      ..arcToPoint(const Offset(18, 11), radius: const Radius.circular(6))
      ..lineTo(18, 16.5)
      ..lineTo(19.5, 18.5)
      ..lineTo(4.5, 18.5)
      ..close();
    if (filled) {
      canvas.drawPath(bell, fill);
    }
    canvas
      ..drawPath(bell, stroke)
      ..drawPath(
        Path()
          ..moveTo(10, 20.5)
          ..arcToPoint(
            const Offset(14, 20.5),
            radius: const Radius.circular(2.2),
            clockwise: false,
          ),
        stroke,
      );

    final dot = this.dot;
    if (dot != null) {
      const center = Offset(18.5, 5.5);
      canvas
        ..drawCircle(center, 4.2, Paint()..color = dotBorder ?? Colors.white)
        ..drawCircle(center, 3, Paint()..color = dot);
    }
  }

  @override
  bool shouldRepaint(BellIconPainter oldDelegate) =>
      super.shouldRepaint(oldDelegate) ||
      oldDelegate.dot != dot ||
      oldDelegate.dotBorder != dotBorder;
}

/// Профиль — человечек: голова и плечи.
class PersonIconPainter extends _BarIconPainter {
  const PersonIconPainter({required super.color, super.filled});

  @override
  void paintIcon(Canvas canvas) {
    final paint = filled ? fill : stroke;
    final head = filled ? 4.0 : 3.6;
    final shoulders = filled
        ? (Path()
            ..moveTo(4.5, 20.5)
            ..cubicTo(4.5, 16.4, 7.9, 13.8, 12, 13.8)
            ..cubicTo(16.1, 13.8, 19.5, 16.4, 19.5, 20.5)
            ..close())
        : (Path()
            ..moveTo(4.8, 20.2)
            ..cubicTo(4.8, 16.5, 8, 14.2, 12, 14.2)
            ..cubicTo(16, 14.2, 19.2, 16.5, 19.2, 20.2)
            ..close());
    canvas
      ..drawCircle(const Offset(12, 8), head, paint)
      ..drawPath(shoulders, paint);
  }
}

/// Время, за которое раздел доезжает до верха.
const _toTopDuration = Duration(milliseconds: 350);

/// Вернуть раздел к самому верху — повторное касание в панели
/// (требование 4). Если долистали далеко, сначала прыжок поближе:
/// иначе прокрутка через сотню постов тянется и дёргается.
Future<void> scrollBackToTop(ScrollController scroll) async {
  if (!scroll.hasClients) {
    return;
  }
  final position = scroll.position;
  final near = position.viewportDimension * 2;
  if (position.pixels > near) {
    scroll.jumpTo(near);
  }
  await scroll.animateTo(0, duration: _toTopDuration, curve: Curves.easeOut);
}
