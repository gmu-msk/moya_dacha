// Жалоба на чужое: specs/008-reports.md.
//
// Жалоба ничего не скрывает и ничего не меняет на экране, поэтому
// единственное, что человек от неё видит, — это вопрос «что не так?» и
// ответ «принято». Оба живут здесь, чтобы у поста и у комментария они
// были одинаковыми.
import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';

/// Сколько символов помещается в причину. То же число, что и на
/// сервисе (specs/008-reports.md, требование 2).
const maxReasonLength = 1000;

/// Спросить, что не так, отправить жалобу и сказать, чем кончилось.
///
/// Вопрос закрывается только после ответа сервиса: пока жалоба не
/// дошла, написанное остаётся в поле, а ошибка видна тут же
/// (specs/008-reports.md, «Если связь пропала»).
Future<void> askAndReport(
  BuildContext context, {
  required String title,
  required String question,
  required Future<void> Function(String? reason) send,
}) async {
  final sent = await showDialog<bool>(
    context: context,
    builder: (context) =>
        _ReportDialog(title: title, question: question, send: send),
  );
  if (sent != true || !context.mounted) {
    return;
  }

  // Что будет дальше, человеку решать не нужно: разбирает жалобу
  // владелец сервиса (ADR-0017).
  ScaffoldMessenger.of(context).showSnackBar(
    const SnackBar(content: Text('Жалоба отправлена. Мы её посмотрим')),
  );
}

class _ReportDialog extends StatefulWidget {
  const _ReportDialog({
    required this.title,
    required this.question,
    required this.send,
  });

  final String title;
  final String question;

  /// Отправить жалобу. null — жалоба без причины: она и так жалоба.
  final Future<void> Function(String? reason) send;

  @override
  State<_ReportDialog> createState() => _ReportDialogState();
}

class _ReportDialogState extends State<_ReportDialog> {
  final TextEditingController _reason = TextEditingController();

  bool _sending = false;
  String? _error;

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final reason = _reason.text.trim();
    setState(() {
      _sending = true;
      _error = null;
    });

    try {
      await widget.send(reason.isEmpty ? null : reason);
      debugPrint('$logMarker report=sent');
      if (mounted) {
        Navigator.of(context).pop(true);
      }
    } on Exception catch (error) {
      debugPrint('$logMarker report=failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _error = errorMessage(error);
        _sending = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AlertDialog(
      title: Text(widget.title),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(widget.question),
          const SizedBox(height: AppGap.small),
          TextField(
            controller: _reason,
            enabled: !_sending,
            maxLength: maxReasonLength,
            maxLines: null,
            minLines: 2,
            textCapitalization: TextCapitalization.sentences,
            decoration: const InputDecoration(
              labelText: 'Что не так?',
              hintText: 'Можно не объяснять',
            ),
          ),
          if (_error != null)
            Text(
              _error!,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.error,
              ),
            ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: _sending ? null : () => Navigator.of(context).pop(false),
          child: const Text('Отмена'),
        ),
        FilledButton(
          onPressed: _sending ? null : _send,
          child: Text(_sending ? 'Отправляю…' : 'Пожаловаться'),
        ),
      ],
    );
  }
}
