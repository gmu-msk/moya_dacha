// Тема «Ситец»: две краски, свои шрифты и общие формы.
//
// Проверка не про вкус, а про то, что выбранное владельцем доехало до
// экранов: краски на местах, шрифты подставлены, карточка и кнопка
// одной формы. Гейт от этого не зависит (ADR-0012), но молча потерять
// шрифт при следующей правке темы не хочется.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';

/// Тема, локализованная как в приложении: размеры текста появляются
/// только после того, как `MaterialApp` домешает геометрию.
Future<ThemeData> builtTheme(WidgetTester tester, Brightness brightness) async {
  late ThemeData theme;
  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(brightness),
      home: Builder(
        builder: (context) {
          theme = Theme.of(context);
          return const SizedBox.shrink();
        },
      ),
    ),
  );
  return theme;
}

void main() {
  testWidgets('кнопки васильковые, мак — только акцент', (tester) async {
    final theme = await builtTheme(tester, Brightness.light);

    expect(theme.colorScheme.primary, const Color(0xFF2F5AA8));
    expect(theme.colorScheme.secondary, const Color(0xFF2F5AA8));
    expect(theme.colorScheme.tertiary, const Color(0xFFC7323C));
  });

  testWidgets('заголовки набраны Rubik, остальной текст — Golos Text', (
    tester,
  ) async {
    final theme = await builtTheme(tester, Brightness.light);

    expect(theme.textTheme.titleMedium?.fontFamily, 'Rubik');
    expect(theme.textTheme.titleLarge?.fontFamily, 'Rubik');
    expect(theme.textTheme.bodyLarge?.fontFamily, 'Golos Text');
    expect(theme.textTheme.labelLarge?.fontFamily, 'Golos Text');
  });

  testWidgets('карточка поста: белая, с кантом и скруглением 8', (
    tester,
  ) async {
    final theme = await builtTheme(tester, Brightness.light);
    final shape = theme.cardTheme.shape! as RoundedRectangleBorder;

    expect(theme.cardTheme.color, theme.colorScheme.surfaceContainerLowest);
    expect(theme.cardTheme.elevation, 0);
    expect(shape.borderRadius, BorderRadius.circular(8));
    expect(shape.side.width, AppShape.hairline);
  });

  testWidgets('тёмная тема остаётся тёмной, краски в ней светлее', (
    tester,
  ) async {
    final dark = await builtTheme(tester, Brightness.dark);

    expect(dark.colorScheme.brightness, Brightness.dark);
    // Краска на тёмном — это цвет текста и значка, поэтому она должна
    // быть светлее фона, а не той же заливкой, что в светлой теме.
    expect(
      dark.colorScheme.primary.computeLuminance(),
      greaterThan(dark.colorScheme.surface.computeLuminance()),
    );
    expect(
      dark.colorScheme.secondary.computeLuminance(),
      greaterThan(dark.colorScheme.surface.computeLuminance()),
    );
  });

  testWidgets('кнопка не ниже цели касания из темы', (tester) async {
    final theme = await builtTheme(tester, Brightness.light);
    final size = theme.filledButtonTheme.style?.minimumSize?.resolve({});

    expect(size?.height, 56);
  });
}
