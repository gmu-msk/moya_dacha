// Тэги — хэштеги в подписи: specs/028-post-tags.md, требования 21–23.
//
// Проверяется то, чего гейт проекта не видит: что касание подсказки
// дописывает хэштег или дополняет набираемый, и что в подписи выделены
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

  Future<List<String?>> pumpSuggestions(
    WidgetTester tester,
    TextEditingController caption, {
    int maxLength = 1000,
  }) async {
    final prefixes = <String?>[];
    await tester.pumpWidget(
      app(
        HashtagSuggestions(
          token: 'т',
          caption: caption,
          maxLength: maxLength,
          suggest: (text, prefix) async {
            prefixes.add(prefix);
            return ['груша', 'сорт'];
          },
        ),
      ),
    );
    await tester.pump();
    return prefixes;
  }

  testWidgets('касание дописывает хэштег в конец подписи через пробел', (
    tester,
  ) async {
    final caption = TextEditingController(text: 'Груши мелкие');
    await pumpSuggestions(tester, caption);
    await tester.tap(find.text('#груша'));
    await tester.pump();
    expect(caption.text, 'Груши мелкие #груша');
    expect(caption.selection.baseOffset, caption.text.length);
    expect(find.text('#груша'), findsNothing);
    await tester.pump(const Duration(seconds: 1));
  });

  testWidgets('набираемый хэштег дополняется, prefix уходит в запрос', (
    tester,
  ) async {
    final caption = TextEditingController();
    caption.value = const TextEditingValue(
      text: 'Груши #со и всё',
      selection: TextSelection.collapsed(offset: 9),
    );
    final prefixes = await pumpSuggestions(tester, caption);
    expect(prefixes, ['со']);
    await tester.tap(find.text('#сорт'));
    await tester.pump();
    expect(caption.text, 'Груши #сорт и всё');
    expect(caption.selection.baseOffset, 'Груши #сорт'.length);
    await tester.pump(const Duration(seconds: 1));
  });

  testWidgets('не влезает в длину подписи — не дописывается', (tester) async {
    final caption = TextEditingController(text: 'а' * 995);
    await pumpSuggestions(tester, caption, maxLength: 1000);
    await tester.tap(find.text('#груша'));
    await tester.pump();
    expect(caption.text, 'а' * 995);
    await tester.pump(const Duration(seconds: 1));
  });

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
