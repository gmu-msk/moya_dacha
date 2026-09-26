// Логотип в заголовке главного экрана (specs/000-ui.md, правило 14).
//
// Проверка не про вид — его смотрят глазами по сценарию 000-ui, — а про
// то, что заглушка не вернулась, знак берёт краски из темы и для TalkBack
// называется словом. Гейт от этого не зависит (ADR-0012).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/app_logo.dart';

Future<void> pumpLogo(WidgetTester tester, Brightness brightness) {
  return tester.pumpWidget(
    MaterialApp(
      theme: appTheme(brightness),
      home: Scaffold(appBar: AppBar(title: const AppLogo())),
    ),
  );
}

FenceMarkPainter markPainter(WidgetTester tester) {
  final paint = tester.widget<CustomPaint>(
    find.descendant(
      of: find.byType(AppLogo),
      matching: find.byWidgetPredicate(
        (widget) => widget is CustomPaint && widget.painter is FenceMarkPainter,
      ),
    ),
  );
  return paint.painter! as FenceMarkPainter;
}

void main() {
  testWidgets('в заголовке знак и надпись, а не рамка «логотип»', (
    tester,
  ) async {
    await pumpLogo(tester, Brightness.light);

    expect(find.text('логотип'), findsNothing);
    expect(find.text('моя дача'), findsOneWidget);
    expect(find.bySemanticsLabel('МояДача'), findsOneWidget);
  });

  testWidgets('штакетник чернильный, мак томатный', (tester) async {
    await pumpLogo(tester, Brightness.light);

    final painter = markPainter(tester);
    expect(painter.fence, const Color(0xFF1D231B));
    expect(painter.poppy, const Color(0xFFD4442A));
  });

  testWidgets('в тёмной теме знак перекрашивается вместе с экраном', (
    tester,
  ) async {
    await pumpLogo(tester, Brightness.dark);

    final painter = markPainter(tester);
    final scheme = appTheme(Brightness.dark).colorScheme;
    expect(painter.fence, scheme.secondary);
    expect(painter.poppy, scheme.tertiary);
  });

  testWidgets('крупный системный шрифт не раздувает логотип', (tester) async {
    tester.platformDispatcher.textScaleFactorTestValue = 2;
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);

    await pumpLogo(tester, Brightness.light);

    expect(tester.takeException(), isNull);
    final height = tester.getSize(find.byType(AppLogo)).height;
    expect(height, lessThanOrEqualTo(kToolbarHeight));
  });
}
