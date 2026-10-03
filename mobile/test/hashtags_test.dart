// Тэги — хэштеги в подписи: specs/028-post-tags.md, требование 23.
//
// Проверяется то, чего гейт проекта не видит: что в подписи выделены
// только хэштеги-тэги поста.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/hashtags.dart';

void main() {
  Widget app(Widget child) => MaterialApp(
    theme: appTheme(Brightness.light),
    home: Scaffold(body: ListView(children: [child])),
  );

  testWidgets('в подписи выделены и кликабельны только тэги поста', (
    tester,
  ) async {
    final opened = <String>[];
    await tester.pumpWidget(
      app(
        CaptionText(
          caption: 'Груши #Груша и #мелочь, яблоки#сорт',
          tags: const ['груша'],
          onOpenTag: opened.add,
        ),
      ),
    );
    final text = tester.widget<RichText>(find.byType(RichText).first);
    final spans = <TextSpan>[];
    text.text.visitChildren((span) {
      if (span is TextSpan && span.text != null) {
        spans.add(span);
      }
      return true;
    });
    final links = [
      for (final span in spans)
        if (span.recognizer != null) span.text,
    ];
    expect(links, ['#Груша']);
    expect(
      spans.map((span) => span.text).join(),
      'Груши #Груша и #мелочь, яблоки#сорт',
    );
  });
}
