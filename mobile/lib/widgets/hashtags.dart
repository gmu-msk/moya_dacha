// Тэги поста — хэштеги в подписи: подпись с выделенными хэштегами
// (specs/028-post-tags.md, требование 23; ADR-0029).

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';

/// Хэштег, как его разбирает сервис (требование 3): `#` в начале или
/// после не-знака слова, дальше знаки слова подряд.
final _hashtag = RegExp(
  r'(?<![\p{L}\p{N}_-])#([\p{L}\p{N}_-]+)',
  unicode: true,
);

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
