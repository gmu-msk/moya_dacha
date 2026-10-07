// Закладка с числом: specs/032-bookmarks.md.
//
// Отзывается сразу, не дожидаясь сервиса, как сердечко; при ошибке
// возвращается как было (требование 12). Сервер с main полей закладок
// не знает — тогда пустая закладка без числа (требование 18).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import 'post_action.dart';

class BookmarkButton extends StatefulWidget {
  const BookmarkButton({
    super.key,
    required this.post,
    required this.token,
    required this.onChanged,
  });

  final Post post;
  final String token;
  final void Function(Post post) onChanged;

  @override
  State<BookmarkButton> createState() => _BookmarkButtonState();
}

class _BookmarkButtonState extends State<BookmarkButton> {
  late bool _saved = widget.post.bookmarked ?? false;
  late int _count = widget.post.bookmarks ?? 0;
  bool _busy = false;

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  @override
  void didUpdateWidget(BookmarkButton old) {
    super.didUpdateWidget(old);
    if (widget.post.id != old.post.id ||
        widget.post.bookmarked != old.post.bookmarked ||
        widget.post.bookmarks != old.post.bookmarks) {
      _saved = widget.post.bookmarked ?? false;
      _count = widget.post.bookmarks ?? 0;
    }
  }

  Future<void> _toggle() async {
    if (_busy) {
      return;
    }

    final messenger = ScaffoldMessenger.of(context);
    final wasSaved = _saved;
    final wasCount = _count;
    setState(() {
      _busy = true;
      _saved = !wasSaved;
      _count = wasCount + (wasSaved ? -1 : 1);
    });
    // «Сохранено» — только при добавлении: пустой значок и так виден
    // (требование 17).
    if (!wasSaved) {
      messenger
        ..hideCurrentSnackBar()
        ..showSnackBar(
          const SnackBar(
            content: Text('Сохранено'),
            duration: Duration(seconds: 2),
          ),
        );
    }

    try {
      final post = wasSaved
          ? await _api.unbookmarkPost(widget.post.id)
          : await _api.bookmarkPost(widget.post.id);
      debugPrint(
        '$logMarker bookmark=${post?.bookmarked} count=${post?.bookmarks}',
      );
      if (!mounted) {
        return;
      }
      if (post != null) {
        setState(() {
          _saved = post.bookmarked ?? _saved;
          _count = post.bookmarks ?? _count;
        });
        widget.onChanged(post);
      }
    } on Exception catch (error) {
      debugPrint('$logMarker bookmark=failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _saved = wasSaved;
        _count = wasCount;
      });
      messenger
        ..hideCurrentSnackBar()
        ..showSnackBar(SnackBar(content: Text(errorMessage(error))));
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.secondary;
    return PostAction(
      icon: _saved ? Icons.bookmark : Icons.bookmark_border,
      count: _count,
      tooltip: _saved ? 'Убрать из сохранённых' : 'Сохранить',
      onPressed: _toggle,
      // Сохранено или нет — видно по заливке (specs/000-ui.md, правило 7).
      color: color,
    );
  }
}
