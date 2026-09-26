// Значок комментария — круглое облачко линией 1.7, как значки нижней
// панели (холст «Моя дача — редизайн», вариант 3a).
import 'package:flutter/material.dart';

class CommentIcon extends StatelessWidget {
  const CommentIcon({super.key, this.color, this.size = 24});

  final Color? color;
  final double size;

  @override
  Widget build(BuildContext context) {
    final color = this.color ?? IconTheme.of(context).color ?? Colors.black;
    return CustomPaint(
      size: Size.square(size),
      painter: CommentIconPainter(color: color),
    );
  }
}

class CommentIconPainter extends CustomPainter {
  const CommentIconPainter({required this.color});

  final Color color;

  static const _radius = Radius.elliptical(8.5, 8);

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 24, size.height / 24);
    final bubble = Path()
      ..moveTo(20.5, 11.5)
      ..arcToPoint(const Offset(8.3, 18.7), radius: _radius)
      ..lineTo(3.5, 20.5)
      ..lineTo(5.1, 16.2)
      ..arcToPoint(const Offset(20.5, 11.5), radius: _radius, largeArc: true)
      ..close();
    canvas.drawPath(
      bubble,
      Paint()
        ..color = color
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.7
        ..strokeJoin = StrokeJoin.round,
    );
  }

  @override
  bool shouldRepaint(CommentIconPainter oldDelegate) =>
      oldDelegate.color != color;
}
