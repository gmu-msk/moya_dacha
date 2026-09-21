// Разговор под постом: specs/006-comments.md.
//
// Комментарии приходят целиком и от старого к новому, поэтому это не
// отдельный список со своей прокруткой, а продолжение экрана поста:
// человек листает пост и доходит до разговора.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import 'author_line.dart';
import 'error_view.dart';
import 'loading_view.dart';

/// Сколько символов помещается в комментарий. То же число, что и на
/// сервисе (specs/006-comments.md, требование 4).
const maxCommentLength = 1000;

class CommentsView extends StatefulWidget {
  const CommentsView({
    super.key,
    required this.postId,
    required this.token,
    required this.onAdded,
  });

  final String postId;
  final String token;

  /// Комментарий оставлен: у поста стало другое число, и показать его
  /// должны все, кто этот пост показывает (specs/006-comments.md,
  /// требование 7).
  final VoidCallback onAdded;

  @override
  State<CommentsView> createState() => _CommentsViewState();
}

class _CommentsViewState extends State<CommentsView> {
  final TextEditingController _text = TextEditingController();

  List<Comment>? _comments;
  String? _error;
  bool _sending = false;

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final page = await _api.getComments(widget.postId);
      debugPrint('$logMarker comments=loaded count=${page?.items.length}');
      if (!mounted) {
        return;
      }
      setState(() => _comments = page?.items ?? const []);
    } on Exception catch (error) {
      debugPrint('$logMarker comments=failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() => _error = errorMessage(error));
    }
  }

  Future<void> _send() async {
    final text = _text.text.trim();
    if (text.isEmpty || _sending) {
      return;
    }
    setState(() => _sending = true);

    try {
      final comment = await _api.addComment(
        widget.postId,
        CommentDraft(text: text),
      );
      debugPrint('$logMarker comment=added id=${comment?.id}');
      if (!mounted) {
        return;
      }
      if (comment != null) {
        setState(() => _comments = [...?_comments, comment]);
        // Поле очищается только после ответа сервиса: пока не дошло,
        // написанное остаётся на месте (требование 13).
        _text.clear();
        widget.onAdded();
      }
    } on Exception catch (error) {
      debugPrint('$logMarker comment=failed error=$error');
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(errorMessage(error))));
    } finally {
      if (mounted) {
        setState(() => _sending = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final comments = _comments;
    final error = _error;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Комментарии', style: theme.textTheme.titleMedium),
        const SizedBox(height: AppGap.small),
        if (error != null)
          ErrorView(message: error, onRetry: _load)
        else if (comments == null)
          const LoadingView(label: 'Открываю комментарии…')
        else if (comments.isEmpty)
          Text(
            'Комментариев пока нет. Напишите первым.',
            style: theme.textTheme.bodyMedium,
          )
        else
          for (final comment in comments) _CommentTile(comment: comment),
        const SizedBox(height: AppGap.medium),
        _Composer(controller: _text, sending: _sending, onSend: _send),
      ],
    );
  }
}

/// Один комментарий: кто, когда и что написал.
class _CommentTile extends StatelessWidget {
  const _CommentTile({required this.comment});

  final Comment comment;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.only(bottom: AppGap.small),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          AuthorLine(author: comment.author, when: comment.createdAt),
          Text(comment.text, style: theme.textTheme.bodyLarge),
        ],
      ),
    );
  }
}

/// Поле для своего комментария и кнопка «Отправить».
class _Composer extends StatelessWidget {
  const _Composer({
    required this.controller,
    required this.sending,
    required this.onSend,
  });

  final TextEditingController controller;
  final bool sending;
  final VoidCallback onSend;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        TextField(
          controller: controller,
          enabled: !sending,
          maxLength: maxCommentLength,
          maxLines: null,
          minLines: 2,
          textCapitalization: TextCapitalization.sentences,
          decoration: const InputDecoration(
            labelText: 'Ваш комментарий',
            hintText: 'Что скажете?',
          ),
        ),
        const SizedBox(height: AppGap.small),
        FilledButton.icon(
          onPressed: sending ? null : onSend,
          icon: const Icon(Icons.send_outlined),
          label: Text(sending ? 'Отправляю…' : 'Отправить'),
        ),
      ],
    );
  }
}
