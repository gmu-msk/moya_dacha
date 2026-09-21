// Разговор под постом: specs/006-comments.md.
//
// Сети в виджет-тесте нет: любой запрос отвечает ошибкой. Это и нужно —
// проверяется то, что без сети и ломается: написанное не должно
// пропадать, когда сервис не ответил (требование 13).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/comments_view.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('написанное не теряется, если сервис не ответил', (tester) async {
    await _pump(tester);

    await tester.enterText(find.byType(TextField), 'Брызгали содой');
    await tester.tap(find.text('Отправить'));
    await tester.pumpAndSettle();

    expect(find.text('Брызгали содой'), findsOneWidget);
    expect(find.byType(SnackBar), findsOneWidget);
  });

  testWidgets('пустой комментарий не отправляется', (tester) async {
    await _pump(tester);

    await tester.tap(find.text('Отправить'));
    await tester.pumpAndSettle();

    // Ни ошибки, ни отправки: отправлять нечего, и сказать об этом
    // человеку нечего тоже.
    expect(find.byType(SnackBar), findsNothing);
  });

  testWidgets('комментарий из одних пробелов не отправляется', (tester) async {
    await _pump(tester);

    await tester.enterText(find.byType(TextField), '   ');
    await tester.tap(find.text('Отправить'));
    await tester.pumpAndSettle();

    expect(find.byType(SnackBar), findsNothing);
  });

  for (final brightness in Brightness.values) {
    final theme = brightness == Brightness.light ? 'светлой' : 'тёмной';

    testWidgets('комментарии рисуются в $theme теме', (tester) async {
      await _pump(tester, brightness: brightness);
      expect(find.text('Комментарии'), findsOneWidget);
    });

    testWidgets('комментарии выживают при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(tester, brightness: brightness, textScale: 2);
    });
  }
}

Future<void> _pump(
  WidgetTester tester, {
  Brightness brightness = Brightness.light,
  double textScale = 1,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(brightness),
      home: MediaQuery(
        data: MediaQueryData(textScaler: TextScaler.linear(textScale)),
        child: Scaffold(
          body: SingleChildScrollView(
            child: CommentsView(
              postId: '00000000-0000-0000-0000-000000000002',
              token: 'т',
              onAdded: () {},
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}
