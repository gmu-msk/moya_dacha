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
import 'confirm.dart';
import 'edit_text_dialog.dart';
import 'error_view.dart';
import 'loading_view.dart';
import 'question_line.dart';
import 'report_dialog.dart';

/// Сколько символов помещается в комментарий. То же число, что и на
/// сервисе (specs/006-comments.md, требование 4).
const maxCommentLength = 1000;

class CommentsView extends StatefulWidget {
  const CommentsView({
    super.key,
    required this.postId,
    required this.token,
    required this.viewerId,
    required this.onChanged,
    this.onOpenAuthor,
    this.question = false,
    this.answerCommentId,
    this.onToggleAnswer,
  });

  final String postId;
  final String token;

  /// Кто смотрит: у своего комментария «Удалить», у чужого —
  /// «Пожаловаться» (specs/007-deletion.md, требование 12,
  /// specs/008-reports.md, требование 11).
  final String viewerId;

  /// Комментарий оставлен или удалён: у поста стало другое число, и
  /// показать его должны все, кто этот пост показывает
  /// (specs/006-comments.md, требование 7).
  final VoidCallback onChanged;

  /// Открыть профиль того, кто написал комментарий
  /// (specs/009-user-profile.md, требование 9).
  final void Function(Author author)? onOpenAuthor;

  /// Пост — вопрос: поле подсказывает «Напишите свой ответ»
  /// (specs/033-question-posts.md, требование 18).
  final bool question;

  /// Комментарий-решение: подсвечен и с меткой (требование 17).
  final String? answerCommentId;

  /// Отметить комментарий решением или снять отметку. Есть только
  /// у автора вопроса (требование 16).
  final void Function(Comment comment)? onToggleAnswer;

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
        widget.onChanged();
      }
    } on Exception catch (error) {
      debugPrint('$logMarker comment=failed error=$error');
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(errorMessage(error))));
    } finally {
      if (mounted) {
        setState(() => _sending = false);
      }
    }
  }

  /// Удалить свой комментарий. Пост остаётся, число у него уменьшается
  /// (specs/007-deletion.md, требование 2).
  Future<void> _delete(Comment comment) async {
    final agreed = await confirmDelete(
      context,
      title: 'Удалить комментарий?',
      question: 'Написанное исчезнет безвозвратно.',
    );
    if (!agreed || !mounted) {
      return;
    }

    try {
      await _api.deleteComment(widget.postId, comment.id);
      debugPrint('$logMarker comment=deleted id=${comment.id}');
      if (!mounted) {
        return;
      }
      setState(
        () => _comments = [
          for (final item in _comments ?? const <Comment>[])
            if (item.id != comment.id) item,
        ],
      );
      widget.onChanged();
    } on Exception catch (error) {
      debugPrint('$logMarker comment=delete_failed error=$error');
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(errorMessage(error))));
    }
  }

  /// Поправить свой комментарий: место в разговоре то же
  /// (specs/022-edit-block-delete.md, требование 3).
  Future<void> _edit(Comment comment) async {
    final updated = await editText<Comment>(
      context,
      title: 'Изменить комментарий',
      initial: comment.text,
      label: 'Комментарий',
      maxLength: maxCommentLength,
      allowEmpty: false,
      save: (text) =>
          _api.editComment(widget.postId, comment.id, CommentDraft(text: text)),
    );
    if (!mounted || updated == null) {
      return;
    }
    debugPrint('$logMarker comment=edited id=${comment.id}');
    setState(
      () => _comments = [
        for (final item in _comments ?? const <Comment>[])
          if (item.id == comment.id) updated else item,
      ],
    );
  }

  /// Пожаловаться на чужой комментарий. Комментарий остаётся на месте,
  /// и число комментариев у поста не меняется
  /// (specs/008-reports.md, требование 1).
  Future<void> _report(Comment comment) async {
    await askAndReport(
      context,
      title: 'Пожаловаться на комментарий?',
      question:
          'Жалобу посмотрит владелец сервиса. Комментарий '
          'останется на месте, и автор о ней не узнает.',
      send: (reason) => _api.reportComment(
        widget.postId,
        comment.id,
        reportDraft: ReportDraft(reason: reason),
      ),
    );
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
          for (final comment in comments)
            _CommentTile(
              comment: comment,
              mine: comment.author.id == widget.viewerId,
              onEdit: () => _edit(comment),
              onDelete: () => _delete(comment),
              onReport: () => _report(comment),
              answer: comment.id == widget.answerCommentId,
              onToggleAnswer: widget.onToggleAnswer == null
                  ? null
                  : () => widget.onToggleAnswer!(comment),
              onOpenAuthor: widget.onOpenAuthor == null
                  ? null
                  : () => widget.onOpenAuthor!(comment.author),
            ),
        const SizedBox(height: AppGap.medium),
        _Composer(
          controller: _text,
          sending: _sending,
          onSend: _send,
          question: widget.question,
        ),
      ],
    );
  }
}

/// Один комментарий: кто, когда и что написал.
class _CommentTile extends StatelessWidget {
  const _CommentTile({
    required this.comment,
    required this.mine,
    required this.onEdit,
    required this.onDelete,
    required this.onReport,
    this.answer = false,
    this.onToggleAnswer,
    this.onOpenAuthor,
  });

  final Comment comment;

  /// Этот комментарий — решение вопроса.
  final bool answer;

  /// Галочка «Отметить как решение» — только у автора вопроса.
  final VoidCallback? onToggleAnswer;

  /// Свой ли это комментарий. Своё удаляют, на чужое жалуются — и
  /// никогда наоборот (specs/008-reports.md, требование 3).
  final bool mine;

  final VoidCallback onEdit;
  final VoidCallback onDelete;
  final VoidCallback onReport;
  final VoidCallback? onOpenAuthor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final toggleAnswer = onToggleAnswer;

    final body = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (answer)
          const Padding(
            padding: EdgeInsets.only(bottom: AppGap.tiny),
            child: AnswerBadge(),
          ),
        Row(
          children: [
            Expanded(
              child: AuthorLine(
                author: comment.author,
                when: comment.createdAt,
                edited: comment.editedAt != null,
                onTap: onOpenAuthor,
              ),
            ),
            if (toggleAnswer != null)
              IconButton(
                tooltip: answer
                    ? 'Снять отметку решения'
                    : 'Отметить как решение',
                onPressed: toggleAnswer,
                color: answer ? theme.colorScheme.primary : null,
                icon: Icon(
                  answer ? Icons.check_circle : Icons.check_circle_outline,
                ),
              ),
            if (mine)
              IconButton(
                tooltip: 'Изменить комментарий',
                onPressed: onEdit,
                icon: const Icon(Icons.edit_outlined),
              ),
            if (mine)
              IconButton(
                tooltip: 'Удалить комментарий',
                onPressed: onDelete,
                icon: const Icon(Icons.delete_outline),
              )
            else
              IconButton(
                tooltip: 'Пожаловаться на комментарий',
                onPressed: onReport,
                icon: const Icon(Icons.flag_outlined),
              ),
          ],
        ),
        Text(comment.text, style: theme.textTheme.bodyLarge),
      ],
    );

    // Решение подсвечено фоном на своём месте: порядок разговора не
    // меняется (требование 17).
    if (!answer) {
      return Padding(
        padding: const EdgeInsets.only(bottom: AppGap.small),
        child: body,
      );
    }
    return Padding(
      padding: const EdgeInsets.only(bottom: AppGap.small),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: theme.colorScheme.primaryContainer,
          borderRadius: BorderRadius.circular(AppShape.small),
        ),
        child: Padding(
          padding: const EdgeInsets.all(AppGap.small),
          child: body,
        ),
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
    this.question = false,
  });

  final TextEditingController controller;
  final bool sending;
  final VoidCallback onSend;
  final bool question;

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
          decoration: InputDecoration(
            labelText: question ? 'Ваш ответ' : 'Ваш комментарий',
            hintText: question ? 'Напишите свой ответ' : 'Что скажете?',
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
