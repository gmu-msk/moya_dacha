// Сердечко с числом: specs/005-likes.md.
//
// Отзывается сразу, не дожидаясь сервиса. При отметке сердечко
// подпрыгивает и рассыпает шесть лепестков (макет, раунд 1). Если сервис
// ответил ошибкой, сердечко возвращается как было (требование 9).
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import 'post_action.dart';

class LikeButton extends StatefulWidget {
  const LikeButton({
    super.key,
    required this.post,
    required this.token,
    required this.onChanged,
  });

  final Post post;
  final String token;
  final void Function(Post post) onChanged;

  @override
  State<LikeButton> createState() => LikeButtonState();
}

class LikeButtonState extends State<LikeButton> with TickerProviderStateMixin {
  late bool _liked = widget.post.liked;
  late int _likes = widget.post.likes;
  bool _busy = false;

  late final AnimationController _pop = AnimationController(
    vsync: this,
    duration: AppMotion.heartPop,
  );
  late final AnimationController _petals = AnimationController(
    vsync: this,
    duration: AppMotion.petals,
  );

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  @override
  void didUpdateWidget(LikeButton oldWidget) {
    super.didUpdateWidget(oldWidget);
    final old = oldWidget;
    if (widget.post.id != old.post.id ||
        widget.post.liked != old.post.liked ||
        widget.post.likes != old.post.likes) {
      _liked = widget.post.liked;
      _likes = widget.post.likes;
    }
  }

  @override
  void dispose() {
    _pop.dispose();
    _petals.dispose();
    super.dispose();
  }

  /// Двойное касание фото только ставит отметку и никогда её не снимает.
  void likeByDoubleTap() {
    if (!_liked) {
      _toggle();
    }
  }

  Future<void> _toggle() async {
    if (_busy) {
      return;
    }

    final wasLiked = _liked;
    final wasLikes = _likes;
    setState(() {
      _busy = true;
      _liked = !wasLiked;
      _likes = wasLikes + (wasLiked ? -1 : 1);
    });
    if (!wasLiked) {
      _pop.forward(from: 0);
      _petals.forward(from: 0);
    }

    try {
      final post = wasLiked
          ? await _api.unlikePost(widget.post.id)
          : await _api.likePost(widget.post.id);
      debugPrint('$logMarker like=${post?.liked} count=${post?.likes}');
      if (!mounted) {
        return;
      }
      if (post != null) {
        setState(() {
          _liked = post.liked;
          _likes = post.likes;
        });
        widget.onChanged(post);
      }
    } on Exception catch (error) {
      debugPrint('$logMarker like=failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _liked = wasLiked;
        _likes = wasLikes;
      });
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(errorMessage(error))));
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final color = scheme.tertiary;
    final size = IconTheme.of(context).size ?? 24;

    // Подскок 1 → 1.38 → 1.
    final scale = TweenSequence<double>([
      TweenSequenceItem(tween: Tween(begin: 1.0, end: 1.38), weight: 45),
      TweenSequenceItem(
        tween: Tween(
          begin: 1.38,
          end: 1.0,
        ).chain(CurveTween(curve: AppMotion.spring)),
        weight: 55,
      ),
    ]).animate(_pop);

    return PostAction(
      iconWidget: SizedBox.square(
        dimension: size,
        child: Stack(
          clipBehavior: Clip.none,
          alignment: Alignment.center,
          children: [
            Positioned.fill(
              child: IgnorePointer(
                child: CustomPaint(
                  painter: _PetalsPainter(
                    progress: _petals,
                    colors: [color, scheme.primary],
                  ),
                ),
              ),
            ),
            ScaleTransition(
              scale: scale,
              child: Icon(
                _liked ? Icons.favorite : Icons.favorite_border,
                color: color,
              ),
            ),
          ],
        ),
      ),
      count: _likes,
      tooltip: _liked ? 'Убрать отметку' : 'Нравится',
      onPressed: _toggle,
      // Отмечено или нет — видно по заливке, а не по цвету (правило 7).
      color: color,
    );
  }
}

/// Шесть лепестков разлетаются от центра и гаснут.
class _PetalsPainter extends CustomPainter {
  _PetalsPainter({required this.progress, required this.colors})
    : super(repaint: progress);

  final Animation<double> progress;
  final List<Color> colors;

  @override
  void paint(Canvas canvas, Size size) {
    final t = progress.value;
    if (t == 0 || t == 1) {
      return;
    }
    final eased = Curves.easeOut.transform(t);
    final center = size.center(Offset.zero);
    final reach = size.width * 0.5 + size.width * 0.6 * eased;
    for (var k = 0; k < 6; k++) {
      final angle = -math.pi / 2 + k * math.pi / 3;
      final at = center + Offset(math.cos(angle), math.sin(angle)) * reach;
      canvas.drawCircle(
        at,
        2.5 * (0.4 + 0.6 * eased),
        Paint()..color = colors[k % 2].withValues(alpha: 1 - t),
      );
    }
  }

  @override
  bool shouldRepaint(_PetalsPainter oldDelegate) => false;
}

/// Большое белое сердце поверх фото при двойном касании.
class BigHeart extends StatefulWidget {
  const BigHeart({super.key});

  @override
  State<BigHeart> createState() => BigHeartState();
}

class BigHeartState extends State<BigHeart>
    with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(
    vsync: this,
    duration: AppMotion.bigHeart,
  );

  void play() => _c.forward(from: 0);

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  // Появление с перелётом, пауза, лёгкое увеличение и угасание.
  static final _scale = TweenSequence<double>([
    TweenSequenceItem(tween: Tween(begin: 0, end: 1.18), weight: 22),
    TweenSequenceItem(tween: Tween(begin: 1.18, end: 0.94), weight: 18),
    TweenSequenceItem(tween: Tween(begin: 0.94, end: 1), weight: 30),
    TweenSequenceItem(tween: Tween(begin: 1, end: 1.08), weight: 30),
  ]);
  static final _opacity = TweenSequence<double>([
    TweenSequenceItem(tween: Tween(begin: 0, end: 1), weight: 22),
    TweenSequenceItem(tween: ConstantTween(1), weight: 48),
    TweenSequenceItem(tween: Tween(begin: 1, end: 0), weight: 30),
  ]);

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: AnimatedBuilder(
        animation: _c,
        builder: (context, _) {
          if (_c.value == 0 || _c.isCompleted) {
            return const SizedBox.shrink();
          }
          return Center(
            child: Opacity(
              opacity: _opacity.evaluate(_c),
              child: Transform.scale(
                scale: _scale.evaluate(_c),
                child: const Icon(
                  Icons.favorite,
                  size: 104,
                  color: Colors.white,
                  shadows: [Shadow(blurRadius: 20, color: Colors.black38)],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
