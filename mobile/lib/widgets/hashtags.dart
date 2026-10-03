// Тэги поста — хэштеги в подписи: подсказки хэштегов под полем подписи и
// подпись с выделенными хэштегами (specs/028-post-tags.md, требования
// 20–23, 25; ADR-0029).
import 'dart:async';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';

/// Через сколько после правки подписи просить подсказки (требование 21).
const _suggestDelay = Duration(milliseconds: 500);

/// Подсказка под полем подписи (требование 20).
const hashtagHelper = 'Тэги — хэштеги в подписи: #груша #советы';

/// Хэштег, как его разбирает сервис (требование 3): `#` в начале или
/// после не-знака слова, дальше знаки слова подряд.
final _hashtag = RegExp(
  r'(?<![\p{L}\p{N}_-])#([\p{L}\p{N}_-]+)',
  unicode: true,
);

/// Хэштег, который набирается прямо перед курсором: `#` и, может быть,
/// начало слова.
final _typing = RegExp(
  r'(?<![\p{L}\p{N}_-])#([\p{L}\p{N}_-]*)$',
  unicode: true,
);

final _tagSymbol = RegExp(r'[\p{L}\p{N}_-]', unicode: true);

/// Набираемый хэштег: где стоит его `#` и его слово в нижнем регистре.
typedef _Typing = ({int start, String prefix});

/// Набираемый хэштег у курсора или `null`, если курсор не сразу за ним.
_Typing? _typingAt(TextEditingValue value) {
  final cursor = value.selection.baseOffset;
  if (!value.selection.isCollapsed ||
      cursor < 0 ||
      cursor > value.text.length) {
    return null;
  }
  // Курсор посреди слова — это правка, а не набор хэштега.
  if (cursor < value.text.length && _tagSymbol.hasMatch(value.text[cursor])) {
    return null;
  }
  final match = _typing.firstMatch(value.text.substring(0, cursor));
  if (match == null) {
    return null;
  }
  return (start: match.start, prefix: match.group(1)!.toLowerCase());
}

/// Строка подсказок хэштегов под полем подписи (требования 21–22).
/// Касание подсказки дописывает `#тэг` в [caption] или заменяет им
/// набираемый у курсора хэштег.
class HashtagSuggestions extends StatefulWidget {
  const HashtagSuggestions({
    super.key,
    required this.token,
    required this.caption,
    required this.maxLength,
    this.enabled = true,
    this.suggest,
  });

  final String token;
  final TextEditingController caption;
  final int maxLength;
  final bool enabled;

  /// Откуда брать подсказки; по умолчанию — `GET /tags/suggestions`.
  final Future<List<String>> Function(String text, String? prefix)? suggest;

  @override
  State<HashtagSuggestions> createState() => _HashtagSuggestionsState();
}

class _HashtagSuggestionsState extends State<HashtagSuggestions> {
  List<String> _suggestions = const [];
  Timer? _debounce;
  String _lastText = '';
  String? _lastPrefix;

  /// Номер запроса подсказок: ответ на устаревший не показывается.
  int _request = 0;

  @override
  void initState() {
    super.initState();
    _lastText = widget.caption.text;
    _lastPrefix = _typingAt(widget.caption.value)?.prefix;
    widget.caption.addListener(_captionChanged);
    _suggest();
  }

  @override
  void didUpdateWidget(HashtagSuggestions oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.caption != widget.caption) {
      oldWidget.caption.removeListener(_captionChanged);
      widget.caption.addListener(_captionChanged);
    }
  }

  @override
  void dispose() {
    widget.caption.removeListener(_captionChanged);
    _debounce?.cancel();
    super.dispose();
  }

  /// Подпись или курсор поменялись: подсказки — через полсекунды тишины.
  void _captionChanged() {
    final text = widget.caption.text;
    final prefix = _typingAt(widget.caption.value)?.prefix;
    if (text == _lastText && prefix == _lastPrefix) {
      return;
    }
    _lastText = text;
    _lastPrefix = prefix;
    _debounce?.cancel();
    _debounce = Timer(_suggestDelay, _suggest);
  }

  Future<void> _suggest() async {
    final request = ++_request;
    final prefix = _typingAt(widget.caption.value)?.prefix;
    try {
      final text = widget.caption.text;
      final items = await (widget.suggest ?? _fromService)(
        text,
        prefix == null || prefix.isEmpty ? null : prefix,
      );
      if (!mounted || request != _request) {
        return;
      }
      setState(() => _suggestions = items);
    } on Exception catch (error) {
      // Нет подсказок — нет строки, публиковать это не мешает.
      debugPrint('$logMarker tags=suggest_failed error=$error');
      if (mounted && request == _request) {
        setState(() => _suggestions = const []);
      }
    }
  }

  Future<List<String>> _fromService(String text, String? prefix) async {
    final answer = await PostsApi(apiClient(token: widget.token))
        .getTagSuggestions(text: text, prefix: prefix);
    return answer?.items ?? const [];
  }

  /// Вставить `#тэг` (требование 22).
  void _insert(String tag) {
    final value = widget.caption.value;
    final text = value.text;
    final typing = _typingAt(value);

    final String updated;
    final int cursor;
    if (typing != null) {
      final end = value.selection.baseOffset;
      final rest = text.substring(end);
      final inserted = rest.startsWith(RegExp(r'\s')) ? '#$tag' : '#$tag ';
      updated = text.substring(0, typing.start) + inserted + rest;
      cursor = typing.start + inserted.length;
    } else {
      final separator = text.isEmpty || RegExp(r'\s$').hasMatch(text)
          ? ''
          : ' ';
      updated = '$text$separator#$tag';
      cursor = updated.length;
    }
    if (updated.runes.length > widget.maxLength) {
      debugPrint('$logMarker tags=suggestion_too_long');
      return;
    }

    debugPrint('$logMarker tags=suggestion_used');
    setState(() => _suggestions = [..._suggestions]..remove(tag));
    widget.caption.value = TextEditingValue(
      text: updated,
      selection: TextSelection.collapsed(offset: cursor),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (_suggestions.isEmpty) {
      return const SizedBox.shrink();
    }
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
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
            for (final tag in _suggestions)
              ActionChip(
                label: Text('#$tag'),
                tooltip: 'Добавить тэг $tag в подпись',
                onPressed: widget.enabled ? () => _insert(tag) : null,
              ),
          ],
        ),
      ],
    );
  }
}

/// Подпись поста, в которой хэштеги из [tags] выделены цветом ссылки и
/// открывают экран «#тэг» через [onOpenTag] (требование 23). Хэштег,
/// которого в [tags] нет, — обычный текст.
class CaptionText extends StatefulWidget {
  const CaptionText({
    super.key,
    required this.caption,
    required this.tags,
    this.onOpenTag,
    this.style,
    this.maxLines,
    this.overflow,
  });

  final String caption;
  final List<String> tags;
  final void Function(String tag)? onOpenTag;
  final TextStyle? style;
  final int? maxLines;
  final TextOverflow? overflow;

  @override
  State<CaptionText> createState() => _CaptionTextState();
}

class _CaptionTextState extends State<CaptionText> {
  final List<TapGestureRecognizer> _recognizers = [];

  @override
  void dispose() {
    _disposeRecognizers();
    super.dispose();
  }

  void _disposeRecognizers() {
    for (final recognizer in _recognizers) {
      recognizer.dispose();
    }
    _recognizers.clear();
  }

  @override
  Widget build(BuildContext context) {
    _disposeRecognizers();
    final theme = Theme.of(context);
    final linkStyle = TextStyle(
      color: theme.colorScheme.primary,
      fontWeight: FontWeight.w600,
    );
    final open = widget.onOpenTag;
    final caption = widget.caption;

    final spans = <InlineSpan>[];
    var from = 0;
    for (final match in _hashtag.allMatches(caption)) {
      final tag = match.group(1)!.toLowerCase();
      if (!widget.tags.contains(tag)) {
        continue;
      }
      if (match.start > from) {
        spans.add(TextSpan(text: caption.substring(from, match.start)));
      }
      TapGestureRecognizer? recognizer;
      if (open != null) {
        recognizer = TapGestureRecognizer()..onTap = () => open(tag);
        _recognizers.add(recognizer);
      }
      spans.add(
        TextSpan(
          text: match.group(0),
          style: linkStyle,
          recognizer: recognizer,
          semanticsLabel: 'Тэг $tag',
        ),
      );
      from = match.end;
    }
    if (from < caption.length) {
      spans.add(TextSpan(text: caption.substring(from)));
    }

    return Text.rich(
      TextSpan(children: spans),
      style: widget.style,
      maxLines: widget.maxLines,
      overflow: widget.overflow,
    );
  }
}
