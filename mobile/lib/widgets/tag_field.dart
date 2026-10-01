// Тэги поста: поле с чипами и подсказками, строка тэгов в карточке
// (specs/028-post-tags.md, требования 20–23, 25).
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';

/// Сколько тэгов у поста — то же число, что и на сервисе (требование 5).
const maxPostTags = 10;

/// Сколько знаков в тэге (требование 1).
const maxTagLength = 30;

/// Через сколько после правки подписи просить подсказки (требование 21).
const _suggestDelay = Duration(milliseconds: 500);

final _tagSymbols = RegExp(r'^[\p{L}\p{N}_-]+$', unicode: true);
final _meaningful = RegExp(r'[\p{L}\p{N}]', unicode: true);

/// Что не так с тэгом: текст для человека, тот же, что у сервиса.
const invalidTagMessage = 'Тэг — одно слово из букв и цифр, до 30 знаков';
const _tooManyMessage = 'Не больше $maxPostTags тэгов';

/// Тэг в том виде, в каком его хранит сервис (требование 2): без пробелов
/// по краям и `#` в начале, в нижнем регистре. Пустая строка — тэга нет;
/// `null` — тэг не годится.
String? normalizeTag(String raw) {
  final tag = raw.trim().replaceFirst(RegExp(r'^#+'), '').toLowerCase();
  if (tag.isEmpty) {
    return '';
  }
  if (tag.runes.length > maxTagLength ||
      !_tagSymbols.hasMatch(tag) ||
      !_meaningful.hasMatch(tag)) {
    return null;
  }
  return tag;
}

/// Поле тэгов: выбранные чипами, поле ввода и строка подсказок.
/// Выбранные поднимаются наверх через [onChanged]. Набранный и не
/// превращённый в чип текст добирает [TagFieldState.collect].
class TagField extends StatefulWidget {
  const TagField({
    super.key,
    required this.token,
    required this.tags,
    required this.onChanged,
    this.caption,
    this.enabled = true,
  });

  final String token;
  final List<String> tags;
  final void Function(List<String> tags) onChanged;

  /// Подпись черновика: по ней сервис подбирает подсказки.
  final TextEditingController? caption;
  final bool enabled;

  @override
  State<TagField> createState() => TagFieldState();
}

class TagFieldState extends State<TagField> {
  final TextEditingController _input = TextEditingController();
  final FocusNode _focus = FocusNode();

  List<String> _suggestions = const [];
  String? _error;
  Timer? _debounce;
  String _lastCaption = '';

  /// Номер запроса подсказок: ответ на устаревший не показывается.
  int _request = 0;

  @override
  void initState() {
    super.initState();
    _lastCaption = widget.caption?.text ?? '';
    widget.caption?.addListener(_captionChanged);
    _suggest();
  }

  @override
  void didUpdateWidget(TagField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.caption != widget.caption) {
      oldWidget.caption?.removeListener(_captionChanged);
      widget.caption?.addListener(_captionChanged);
    }
  }

  @override
  void dispose() {
    widget.caption?.removeListener(_captionChanged);
    _debounce?.cancel();
    _input.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _captionChanged() {
    final text = widget.caption?.text ?? '';
    if (text == _lastCaption) {
      return;
    }
    _lastCaption = text;
    _debounce?.cancel();
    _debounce = Timer(_suggestDelay, _suggest);
  }

  Future<void> _suggest() async {
    final request = ++_request;
    try {
      final answer = await PostsApi(apiClient(token: widget.token))
          .getTagSuggestions(text: widget.caption?.text, exclude: widget.tags);
      if (!mounted || request != _request) {
        return;
      }
      setState(() => _suggestions = answer?.items ?? const []);
    } on Exception catch (error) {
      // Нет подсказок — нет строки, публиковать это не мешает.
      debugPrint('$logMarker tags=suggest_failed error=$error');
      if (mounted && request == _request) {
        setState(() => _suggestions = const []);
      }
    }
  }

  /// Добавить тэги к выбранным. Возвращает false, если какой-то не
  /// годится или их стало бы больше десяти: тогда под полем ошибка.
  bool _add(Iterable<String> raw) {
    final tags = [...widget.tags];
    for (final item in raw) {
      final tag = normalizeTag(item);
      if (tag == null) {
        setState(() => _error = invalidTagMessage);
        return false;
      }
      if (tag.isEmpty || tags.contains(tag)) {
        continue;
      }
      if (tags.length >= maxPostTags) {
        setState(() => _error = _tooManyMessage);
        return false;
      }
      tags.add(tag);
    }
    setState(() => _error = null);
    if (tags.length != widget.tags.length) {
      debugPrint('$logMarker tags=added count=${tags.length}');
      widget.onChanged(tags);
      _suggestAfterChange(tags);
    }
    return true;
  }

  void _remove(String tag) {
    final tags = [...widget.tags]..remove(tag);
    setState(() => _error = null);
    widget.onChanged(tags);
    _suggestAfterChange(tags);
  }

  /// Подсказки после смены тэгов: выбранный уходит сразу, не дожидаясь
  /// сервиса, на его место приходит следующий с ответом.
  void _suggestAfterChange(List<String> tags) {
    setState(() {
      _suggestions = [
        for (final tag in _suggestions)
          if (!tags.contains(tag)) tag,
      ];
    });
    // Родитель ещё не перестроился: подсказки просятся со свежим списком.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _suggest();
      }
    });
  }

  /// Пробел или запятая превращают набранное в чип (требование 20).
  void _typed(String text) {
    final parts = text.split(RegExp(r'[\s,]+'));
    if (parts.length == 1) {
      if (_error != null) {
        setState(() => _error = null);
      }
      return;
    }
    final done = parts.sublist(0, parts.length - 1);
    if (_add(done)) {
      _input.value = TextEditingValue(
        text: parts.last,
        selection: TextSelection.collapsed(offset: parts.last.length),
      );
    }
  }

  void _submitted(String text) {
    if (_add(text.split(RegExp(r'[\s,]+')))) {
      _input.clear();
    }
    _focus.requestFocus();
  }

  /// Тэги для отправки, вместе с набранным и не превращённым в чип.
  /// `null` — набранное не годится, ошибка уже под полем.
  List<String>? collect() {
    final pending = _input.text.trim();
    if (pending.isEmpty) {
      return widget.tags;
    }
    final tags = [...widget.tags];
    for (final item in pending.split(RegExp(r'[\s,]+'))) {
      final tag = normalizeTag(item);
      if (tag == null) {
        setState(() => _error = invalidTagMessage);
        return null;
      }
      if (tag.isNotEmpty && !tags.contains(tag)) {
        tags.add(tag);
      }
    }
    if (tags.length > maxPostTags) {
      setState(() => _error = _tooManyMessage);
      return null;
    }
    return tags;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final suggestions = [
      for (final tag in _suggestions)
        if (!widget.tags.contains(tag)) tag,
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        TextField(
          controller: _input,
          focusNode: _focus,
          enabled: widget.enabled,
          textInputAction: TextInputAction.done,
          onChanged: _typed,
          onSubmitted: _submitted,
          decoration: InputDecoration(
            labelText: 'Тэги',
            hintText: 'Добавить тэг',
            prefixText: '#',
            errorText: _error,
            helperText: _error == null
                ? 'Одно слово: груша, поделюсь, советы'
                : null,
          ),
        ),
        if (widget.tags.isNotEmpty) ...[
          const SizedBox(height: AppGap.small),
          Wrap(
            spacing: AppGap.small,
            runSpacing: AppGap.small,
            children: [
              for (final tag in widget.tags)
                InputChip(
                  label: Text('#$tag'),
                  isEnabled: widget.enabled,
                  onDeleted: () => _remove(tag),
                  deleteButtonTooltipMessage: 'Убрать тэг',
                ),
            ],
          ),
        ],
        if (suggestions.isNotEmpty) ...[
          const SizedBox(height: AppGap.small),
          Text(
            'Подходящие тэги',
            style: theme.textTheme.labelLarge?.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: AppGap.tiny),
          Wrap(
            spacing: AppGap.small,
            runSpacing: AppGap.small,
            children: [
              for (final tag in suggestions)
                ActionChip(
                  avatar: Icon(Icons.add, size: 18, color: scheme.primary),
                  label: Text(tag),
                  onPressed: widget.enabled ? () => _add([tag]) : null,
                ),
            ],
          ),
        ],
      ],
    );
  }
}

/// Тэги поста строкой «#груша #сорт» цветом ссылки (требование 23).
/// Касание тэга — [onOpen]; без него тэги не кликабельны.
class PostTagsLine extends StatelessWidget {
  const PostTagsLine({super.key, required this.tags, this.onOpen});

  final List<String> tags;
  final void Function(String tag)? onOpen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final style = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.primary,
      fontWeight: FontWeight.w600,
    );
    final open = onOpen;

    return Wrap(
      spacing: AppGap.small,
      runSpacing: AppGap.tiny,
      children: [
        for (final tag in tags)
          Semantics(
            button: open != null,
            label: 'Тэг $tag',
            excludeSemantics: true,
            child: InkWell(
              onTap: open == null ? null : () => open(tag),
              borderRadius: BorderRadius.circular(AppShape.small),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: AppGap.tiny),
                child: Text('#$tag', style: style),
              ),
            ),
          ),
      ],
    );
  }
}

/// Окно «Изменить тэги» (требование 25): то же поле с подсказками,
/// «Сохранить» и «Отмена». Закрывается после ответа сервиса с его
/// результатом; по «Отмене» — с `null`.
Future<T?> editTags<T>(
  BuildContext context, {
  required String token,
  required List<String> initial,
  required String caption,
  required Future<T?> Function(List<String> tags) save,
}) => showDialog<T>(
  context: context,
  builder: (context) => _EditTagsDialog<T>(
    token: token,
    initial: initial,
    caption: caption,
    save: save,
  ),
);

class _EditTagsDialog<T> extends StatefulWidget {
  const _EditTagsDialog({
    required this.token,
    required this.initial,
    required this.caption,
    required this.save,
  });

  final String token;
  final List<String> initial;
  final String caption;
  final Future<T?> Function(List<String> tags) save;

  @override
  State<_EditTagsDialog<T>> createState() => _EditTagsDialogState<T>();
}

class _EditTagsDialogState<T> extends State<_EditTagsDialog<T>> {
  late List<String> _tags = [...widget.initial];
  late final TextEditingController _caption = TextEditingController(
    text: widget.caption,
  );
  final _field = GlobalKey<TagFieldState>();
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _caption.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final field = _field.currentState;
    final tags = field == null ? _tags : field.collect();
    if (tags == null) {
      return;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final result = await widget.save(tags);
      if (!mounted) {
        return;
      }
      Navigator.of(context).pop(result);
    } on Exception catch (error) {
      debugPrint('$logMarker tags=save_failed error=$error');
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
    final error = _error;
    return AlertDialog(
      title: const Text('Изменить тэги'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            TagField(
              key: _field,
              token: widget.token,
              tags: _tags,
              caption: _caption,
              enabled: !_saving,
              onChanged: (tags) => setState(() => _tags = tags),
            ),
            if (error != null) ...[
              const SizedBox(height: AppGap.small),
              Text(
                error,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
          ],
        ),
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
