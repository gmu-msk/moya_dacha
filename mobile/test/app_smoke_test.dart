// Приложение запускается и рисуется.
//
// Это не гейт и не проверка поведения: гейт в проекте один — интеграционные
// тесты сервера (ADR-0002), а фичи проверяются глазами по сценариям показа
// (ADR-0009). Здесь проверяется ровно одно: экран собирается без исключения,
// в обеих темах и при увеличенном системном шрифте. Такая ошибка иначе
// находится только прогоном в эмуляторе, а он идёт минуты.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/main.dart';
import 'package:moya_dacha/screens/gallery_screen.dart';
import 'package:moya_dacha/screens/login_screen.dart';
import 'package:moya_dacha/theme.dart';

void main() {
  testWidgets('приложение открывается', (tester) async {
    await tester.pumpWidget(const MoyaDachaApp());
    await tester.pump();
  });

  for (final brightness in Brightness.values) {
    final theme = brightness == Brightness.light ? 'светлой' : 'тёмной';

    testWidgets('экран входа рисуется в $theme теме', (tester) async {
      await _pump(tester, brightness, LoginScreen(onSignedIn: (_) async {}));
    });

    testWidgets('витрина рисуется в $theme теме', (tester) async {
      await _pump(tester, brightness, const GalleryScreen());
    });

    testWidgets('экран входа выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(
        tester,
        brightness,
        LoginScreen(onSignedIn: (_) async {}),
        textScale: 2,
      );
    });

    testWidgets('витрина выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(tester, brightness, const GalleryScreen(), textScale: 2);
    });
  }
}

/// Показать экран так, как его увидит человек: с нашей темой и, если надо,
/// с увеличенным системным шрифтом.
Future<void> _pump(
  WidgetTester tester,
  Brightness brightness,
  Widget screen, {
  double textScale = 1,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(brightness),
      home: MediaQuery(
        data: MediaQueryData(textScaler: TextScaler.linear(textScale)),
        child: screen,
      ),
    ),
  );
  await tester.pump();
}
