// Правка подписи своего поста вместе с подписями его фото
// (specs/022-edit-block-delete.md, требование 1;
// specs/034-photo-captions.md, требование 15).
//
// Как edit_text_dialog: окно закрывается только после ответа сервиса,
// пока не сохранилось — написанное остаётся, ошибка — под полями.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import 'edit_text_dialog.dart';

/// Показать окно правки подписей. [save] получает подпись поста и все
/// подписи фото (id → текст, пустой — подписи нет); окно закрывается
/// с его результатом, когда он удался, и с `null` по «Отмене».
Future<Post?> editCaptions(
  BuildContext context, {
  required Post post,
  required Future<Post?> Function(String caption, Map<String, String> photos)
  save,
}) => showDialog<Post>(
  context: context,
  builder: (context) => _EditCaptionsDialog(post: post, save: save),
);

class _EditCaptionsDialog extends StatefulWidget {
  const _EditCaptionsDialog({required this.post, required this.save});

  final Post post;
  final Future<Post?> Function(String caption, Map<String, String> photos) save;

  @override
  State<_EditCaptionsDialog> createState() => _EditCaptionsDialogState();
}

class _EditCaptionsDialogState extends State<_EditCaptionsDialog> {
  late final _caption = TextEditingController(text: widget.post.caption);
  late final _photos = {
    for (final m in widget.post.media)
      m.id: TextEditingController(text: m.caption ?? ''),
  };
  bool _saving = false;
  String? _error;

  /// Подписи фото — у карусели, а у одиночного фото, только если подпись
  /// уже есть: её должно быть можно снять.
  bool get _perPhoto =>
      widget.post.media.length > 1 ||
      widget.post.media.any((m) => (m.caption ?? '').isNotEmpty);

  @override
  void dispose() {
    _caption.dispose();
    for (final c in _photos.values) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _save() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final result = await widget.save(_caption.text.trim(), {
        if (_perPhoto)
          for (final e in _photos.entries) e.key: e.value.text.trim(),
      });
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
    final media = widget.post.media;
    return AlertDialog(
      title: const Text('Изменить подпись'),
      scrollable: true,
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            controller: _caption,
            enabled: !_saving,
            autofocus: true,
            maxLength: maxCaptionLength,
            maxLines: null,
            minLines: 3,
            textCapitalization: TextCapitalization.sentences,
            decoration: InputDecoration(
              labelText: 'Подпись поста',
              errorText: _error,
            ),
          ),
          if (_perPhoto)
            for (var i = 0; i < media.length; i++)
              Padding(
                padding: const EdgeInsets.only(top: AppGap.small),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(AppShape.small),
                      child: Image.network(
                        mediaUrl(media[i].url),
                        width: 56,
                        height: 70,
                        fit: BoxFit.cover,
                      ),
                    ),
                    const SizedBox(width: AppGap.snug),
                    Expanded(
                      child: TextField(
                        controller: _photos[media[i].id],
                        enabled: !_saving,
                        maxLength: maxCaptionLength,
                        maxLines: null,
                        textCapitalization: TextCapitalization.sentences,
                        decoration: InputDecoration(
                          labelText: 'Подпись к фото ${i + 1}',
                        ),
                      ),
                    ),
                  ],
                ),
              ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.of(context).pop(),
          child: const Text('Отмена'),
        ),
        FilledButton(
          onPressed: _saving ? null : _save,
          child: Text(_saving ? 'Сохраняю…' : 'Сохранить'),
        ),
      ],
    );
  }
}
