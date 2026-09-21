// Сердечко с числом: specs/005-likes.md.
//
// Отзывается сразу, не дожидаясь сервиса: лайк тем и хорош, что
// мгновенный. Если сервис ответил ошибкой, сердечко возвращается как
// было и человек видит, что не получилось (требование 9).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';

class LikeButton extends StatefulWidget {
  const LikeButton({
    super.key,
    required this.post,
    required this.token,
    required this.onChanged,
  });

  final Post post;
  final String token;

  /// Пост, каким его вернул сервис: ленте и экрану поста нужно показать
  /// одно и то же число.
  final void Function(Post post) onChanged;

  @override
  State<LikeButton> createState() => _LikeButtonState();
}

class _LikeButtonState extends State<LikeButton> {
  late bool _liked = widget.post.liked;
  late int _likes = widget.post.likes;
  bool _busy = false;

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  @override
  void didUpdateWidget(LikeButton old) {
    super.didUpdateWidget(old);
    // Пост сверху стал другим: либо на этом месте теперь другой пост,
    // либо тот же, но отмеченный не здесь — на экране поста, откуда
    // лента получает его заново. Сверять только идентификатор мало:
    // лайк, поставленный внутри поста, в ленту тогда не доезжает.
    //
    // Своё касание при этом не теряется: пока сверху ничего не менялось,
    // показывается то, что мы поставили сами, не дожидаясь сервиса
    // (specs/005-likes.md, требование 9).
    if (widget.post.id != old.post.id ||
        widget.post.liked != old.post.liked ||
        widget.post.likes != old.post.likes) {
      _liked = widget.post.liked;
      _likes = widget.post.likes;
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
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(errorMessage(error))));
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return TextButton.icon(
      onPressed: _toggle,
      icon: Icon(
        _liked ? Icons.favorite : Icons.favorite_border,
        color: _liked ? theme.colorScheme.primary : null,
      ),
      label: Text(_likes == 0 ? 'Нравится' : '$_likes'),
      style: TextButton.styleFrom(
        padding: const EdgeInsets.symmetric(horizontal: AppGap.small),
      ),
    );
  }
}
