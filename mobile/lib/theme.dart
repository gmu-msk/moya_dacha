// Тема приложения: цвета, размеры текста, отступы и цели касания.
//
// Аудитория МояДачи — дачники, среди них много людей старшего возраста
// (ADR-0003). Поэтому базовый шрифт крупнее материалового, а кнопки выше
// того, что Material даёт по умолчанию.
//
// Экраны не задают ни цветов, ни размеров шрифта, ни голых чисел отступа:
// всё это живёт здесь (ADR-0011).
import 'package:flutter/material.dart';

/// Цвет, из которого Material 3 выводит обе палитры — светлую и тёмную.
const _seed = Color(0xFF3F7D3F);

/// Во сколько раз шрифт крупнее материалового по умолчанию: основной текст
/// становится 17–18sp вместо 14sp. Системное увеличение шрифта этим не
/// отменяется, а умножается на него, поэтому экраны должны выживать и при
/// двукратном размере — это пункт чек-листа каждого сценария показа.
const _fontScale = 1.25;

/// Наименьшая высота кнопки. Material 3 даёт 40dp; рекомендация по целям
/// касания — 48dp, а нашим пользователям и этого мало.
const _tapTargetHeight = 56.0;

/// Отступы. Других чисел отступа в экранах быть не должно.
abstract final class AppGap {
  /// Между строками одного блока.
  static const small = 8.0;

  /// Между блоками внутри экрана.
  static const medium = 16.0;

  /// Поля экрана и расстояние между его крупными частями.
  static const large = 24.0;
}

/// Тема светлая или тёмная. Какая из них показана, решает система
/// (`themeMode: ThemeMode.system`), своего переключателя в приложении нет.
ThemeData appTheme(Brightness brightness) {
  final colorScheme = ColorScheme.fromSeed(
    seedColor: _seed,
    brightness: brightness,
  );
  final base = ThemeData(colorScheme: colorScheme);

  return base.copyWith(
    // Размеры текста живут в «геометрии» темы, а не в textTheme: в самой
    // теме размер у стиля не проставлен, он появляется только когда
    // MaterialApp домешивает геометрию под язык. Поэтому увеличиваем
    // геометрию — тогда крупнее становится весь текст сразу, включая
    // заголовки, кнопки и подписи полей.
    typography: Typography.material2021(
      platform: base.platform,
      colorScheme: colorScheme,
      englishLike: Typography.englishLike2021.apply(fontSizeFactor: _fontScale),
      dense: Typography.dense2021.apply(fontSizeFactor: _fontScale),
      tall: Typography.tall2021.apply(fontSizeFactor: _fontScale),
    ),
    // Стандартная плотность, а не компактная: цели касания не ужимаются.
    visualDensity: VisualDensity.standard,
    materialTapTargetSize: MaterialTapTargetSize.padded,
    appBarTheme: base.appBarTheme.copyWith(centerTitle: true),
    filledButtonTheme: const FilledButtonThemeData(style: _buttonSize),
    outlinedButtonTheme: const OutlinedButtonThemeData(style: _buttonSize),
    textButtonTheme: const TextButtonThemeData(style: _buttonSize),
    // Рамка у полей ввода — общая: экран её не повторяет.
    inputDecorationTheme: base.inputDecorationTheme.copyWith(
      border: const OutlineInputBorder(),
      contentPadding: const EdgeInsets.all(AppGap.medium),
    ),
  );
}

/// Кнопка не ниже [_tapTargetHeight], какой бы короткой ни была надпись.
const _buttonSize = ButtonStyle(
  minimumSize: WidgetStatePropertyAll(Size(64, _tapTargetHeight)),
);
