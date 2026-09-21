// Вопрос перед тем, чего не вернуть: specs/007-deletion.md.
//
// Удаление в проекте жёсткое, восстанавливать нечем (ADR-0007), поэтому
// спросить обязательно — и спросить одинаково везде, где спрашиваем.
import 'package:flutter/material.dart';

/// Спросить и дождаться ответа. `true` — человек согласился.
Future<bool> confirmDelete(
  BuildContext context, {
  required String title,
  required String question,
}) async {
  final agreed = await showDialog<bool>(
    context: context,
    builder: (context) => AlertDialog(
      title: Text(title),
      content: Text(question),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: const Text('Отмена'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(true),
          style: FilledButton.styleFrom(
            backgroundColor: Theme.of(context).colorScheme.error,
          ),
          child: const Text('Удалить'),
        ),
      ],
    ),
  );
  return agreed ?? false;
}
