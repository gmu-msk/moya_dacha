// Числа темы для макета: цвета обеих палитр и размеры текста.
//
// Макет экрана рисуется не на глаз, а из этих значений (specs/000-ui.md,
// раздел «Макеты»). Считает их сам Flutter: палитру выводит
// `ColorScheme.fromSeed` из цвета-семени, а размеры текста появляются
// только после того, как `MaterialApp` домешает в тему геометрию, —
// поэтому это прогон в тестовом окружении, а не обычный скрипт.
//
// Запускать через `make theme-tokens`: он прогоняет этот файл и печатает
// то, что между маркерами.
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';

/// Цвет как `#RRGGBB` — в таком виде он нужен в макете.
String hex(Color color) =>
    '#${color.toARGB32().toRadixString(16).padLeft(8, '0').substring(2).toUpperCase()}';

Map<String, String> colors(ColorScheme s) => {
  'primary': hex(s.primary),
  'onPrimary': hex(s.onPrimary),
  'primaryContainer': hex(s.primaryContainer),
  'onPrimaryContainer': hex(s.onPrimaryContainer),
  'secondary': hex(s.secondary),
  'secondaryContainer': hex(s.secondaryContainer),
  'onSecondaryContainer': hex(s.onSecondaryContainer),
  'surface': hex(s.surface),
  'onSurface': hex(s.onSurface),
  'surfaceContainerLow': hex(s.surfaceContainerLow),
  'surfaceContainerHighest': hex(s.surfaceContainerHighest),
  'onSurfaceVariant': hex(s.onSurfaceVariant),
  'outline': hex(s.outline),
  'outlineVariant': hex(s.outlineVariant),
  'error': hex(s.error),
  'onError': hex(s.onError),
  'errorContainer': hex(s.errorContainer),
};

Map<String, double?> textSizes(TextTheme t) => {
  'titleLarge': t.titleLarge?.fontSize,
  'titleMedium': t.titleMedium?.fontSize,
  'bodyLarge': t.bodyLarge?.fontSize,
  'bodyMedium': t.bodyMedium?.fontSize,
  'bodySmall': t.bodySmall?.fontSize,
  'labelLarge': t.labelLarge?.fontSize,
};

void main() {
  testWidgets('числа темы', (tester) async {
    final tuning = const ThemeTuning();
    final out = <String, dynamic>{
      'отступы': {
        'small': AppGap.small,
        'medium': AppGap.medium,
        'large': AppGap.large,
      },
      'кнопка': {'наименьшаяВысота': tuning.tapTargetHeight},
      'поляВвода': {'скругление': tuning.inputRadius},
      'шрифт': {'множитель': tuning.fontScale},
    };

    for (final brightness in Brightness.values) {
      late TextTheme textTheme;
      await tester.pumpWidget(
        MaterialApp(
          theme: appTheme(brightness),
          home: Builder(
            builder: (context) {
              textTheme = Theme.of(context).textTheme;
              return const SizedBox.shrink();
            },
          ),
        ),
      );

      final name = brightness == Brightness.light ? 'светлая' : 'тёмная';
      out[name] = {
        'цвета': colors(
          ColorScheme.fromSeed(seedColor: tuning.seed, brightness: brightness),
        ),
        'размерыТекста': textSizes(textTheme),
      };
    }

    // Маркеры: по ним `make theme-tokens` достаёт JSON из вывода прогона.
    stdout.writeln('--- ТОКЕНЫ ТЕМЫ ---');
    stdout.writeln(const JsonEncoder.withIndent('  ').convert(out));
    stdout.writeln('--- КОНЕЦ ---');
  });
}
