// Правка своего текста: подписи поста и комментария
// (specs/022-edit-block-delete.md, требования 1–3).
//
// Одно окно на оба случая: поле с нынешним текстом, «Сохранить» и
// «Отмена». Окно закрывается только после ответа сервиса: пока не
// сохранилось, написанное остаётся в поле, а ошибка — под ним.
import 'package:flutter/material.dart';

import '../api.dart';

/// Сколько символов помещается в подпись. То же число, что и на сервисе
/// (specs/003-posts.md, требование 5).
const maxCaptionLength = 1000;

/// Показать окно правки. [save] отправляет текст на сервис; окно
/// закрывается с его результатом, когда он удался, и с `null` по
/// «Отмене». [allowEmpty] — можно ли сохранить пустой текст: подпись
/// можно, комментарий нет. [below] — что показать под полем: у подписи
/// это подсказки хэштегов (specs/028-post-tags.md, требование 25).
Future<T?> editText<T>(
  BuildContext context, {
  required String title,
  required String initial,
  required String label,
  required int maxLength,
  required bool allowEmpty,
  required Future<T?> Function(String text) save,
  String? helper,
  Widget Function(TextEditingController text, bool enabled)? below,
}) => showDialog<T>(
  context: context,
  builder: (context) => _EditTextDialog<T>(
    title: title,
    initial: initial,
    label: label,
    maxLength: maxLength,
    allowEmpty: allowEmpty,
    save: save,
    helper: helper,
    below: below,
  ),
);

class _EditTextDialog<T> extends StatefulWidget {
  const _EditTextDialog({
    required this.title,
    required this.initial,
    required this.label,
    required this.maxLength,
    required this.allowEmpty,
    required this.save,
    this.helper,
    this.below,
  });

  final String title;
  final String initial;
  final String label;
  final int maxLength;
  final bool allowEmpty;
  final Future<T?> Function(String text) save;
  final String? helper;
  final Widget Function(TextEditingController text, bool enabled)? below;

  @override
  State<_EditTextDialog<T>> createState() => _EditTextDialogState<T>();
}

class _EditTextDialogState<T> extends State<_EditTextDialog<T>> {
  late final TextEditingController _text = TextEditingController(
    text: widget.initial,
  );
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  bool get _canSave => widget.allowEmpty || _text.text.trim().isNotEmpty;

  Future<void> _save() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final result = await widget.save(_text.text.trim());
      if (!mounted) {
        return;
      }
      Navigator.of(context).pop(result);
    } on Exception catch (error) {
      debugPrint('$logMarker edit=failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _saving = false;
        _error = serviceErrorCode(error) == null
            ? 'Не получилось сохранить. Проверьте связь и попробуйте ещё раз'
            : errorMessage(error);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final field = TextField(
      controller: _text,
      enabled: !_saving,
      autofocus: true,
      maxLength: widget.maxLength,
      maxLines: null,
      minLines: 3,
      textCapitalization: TextCapitalization.sentences,
      onChanged: (_) => setState(() {}),
      decoration: InputDecoration(
        labelText: widget.label,
        errorText: _error,
        helperText: widget.helper,
        helperMaxLines: 2,
      ),
    );
    final below = widget.below;
    return AlertDialog(
      title: Text(widget.title),
      content: below == null
          ? field
          : SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [field, below(_text, !_saving)],
              ),
            ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.of(context).pop(),
          child: const Text('Отмена'),
        ),
        FilledButton(
          onPressed: _saving || !_canSave ? null : _save,
          child: Text(_saving ? 'Сохраняю…' : 'Сохранить'),
        ),
      ],
    );
  }
}
