// Тема приложения: цвета, размеры текста, отступы и цели касания.
//
// Аудитория МояДачи — дачники, среди них много людей старшего возраста
// (ADR-0003). Поэтому базовый шрифт крупнее материалового, а кнопки выше
// того, что Material даёт по умолчанию.
//
// Экраны не задают ни цветов, ни размеров шрифта, ни голых чисел отступа:
// всё это живёт здесь (ADR-0012).
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

/// Скругление рамки у полей ввода. Столько же даёт `OutlineInputBorder`
/// по умолчанию — величина названа, чтобы её можно было покрутить.
const _inputRadius = 4.0;

/// Величины темы, которые можно покрутить, не пересобирая приложение.
///
/// Значения по умолчанию — те, с которыми приложение живёт: продуктовые
/// экраны вызывают [appTheme] без настройки и получают ровно прежнюю тему.
/// Другие значения подставляет только песочница витрины
/// (`mobile/lib/screens/gallery_screen.dart`, ADR-0012): владелец крутит
/// их на своём телефоне и присылает то, что понравилось, а сюда они
/// попадают правкой констант выше.
///
/// Отступов [AppGap] здесь нет и быть не может: они — константы времени
/// компиляции, экраны подставляют их прямо в свои `EdgeInsets`, и без
/// пересборки они не меняются.
@immutable
class ThemeTuning {
  const ThemeTuning({
    this.seed = _seed,
    this.fontScale = _fontScale,
    this.tapTargetHeight = _tapTargetHeight,
    this.inputRadius = _inputRadius,
  });

  /// Цвет-семя, из которого выводится вся палитра.
  final Color seed;

  /// Множитель размера текста поверх материалового.
  final double fontScale;

  /// Наименьшая высота кнопки.
  final double tapTargetHeight;

  /// Скругление рамки полей ввода.
  final double inputRadius;

  /// То же самое в виде констант этого файла: песочница витрины
  /// показывает их и даёт скопировать, чтобы прислать в задачу.
  String asThemeConstants() {
    final hex = seed.toARGB32().toRadixString(16).padLeft(8, '0').toUpperCase();
    return 'const _seed = Color(0x$hex);\n'
        'const _fontScale = ${fontScale.toStringAsFixed(2)};\n'
        'const _tapTargetHeight = ${tapTargetHeight.toStringAsFixed(1)};\n'
        'const _inputRadius = ${inputRadius.toStringAsFixed(1)};';
  }

  ThemeTuning copyWith({
    Color? seed,
    double? fontScale,
    double? tapTargetHeight,
    double? inputRadius,
  }) {
    return ThemeTuning(
      seed: seed ?? this.seed,
      fontScale: fontScale ?? this.fontScale,
      tapTargetHeight: tapTargetHeight ?? this.tapTargetHeight,
      inputRadius: inputRadius ?? this.inputRadius,
    );
  }
}

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
ThemeData appTheme(
  Brightness brightness, {
  ThemeTuning tuning = const ThemeTuning(),
}) {
  final colorScheme = ColorScheme.fromSeed(
    seedColor: tuning.seed,
    brightness: brightness,
  );
  final base = ThemeData(colorScheme: colorScheme);

  // Кнопка не ниже tuning.tapTargetHeight, какой бы короткой ни была
  // надпись.
  final buttonSize = ButtonStyle(
    minimumSize: WidgetStatePropertyAll(Size(64, tuning.tapTargetHeight)),
  );

  return base.copyWith(
    // Размеры текста живут в «геометрии» темы, а не в textTheme: в самой
    // теме размер у стиля не проставлен, он появляется только когда
    // MaterialApp домешивает геометрию под язык. Поэтому увеличиваем
    // геометрию — тогда крупнее становится весь текст сразу, включая
    // заголовки, кнопки и подписи полей.
    typography: Typography.material2021(
      platform: base.platform,
      colorScheme: colorScheme,
      englishLike: Typography.englishLike2021.apply(
        fontSizeFactor: tuning.fontScale,
      ),
      dense: Typography.dense2021.apply(fontSizeFactor: tuning.fontScale),
      tall: Typography.tall2021.apply(fontSizeFactor: tuning.fontScale),
    ),
    // Стандартная плотность, а не компактная: цели касания не ужимаются.
    visualDensity: VisualDensity.standard,
    materialTapTargetSize: MaterialTapTargetSize.padded,
    appBarTheme: base.appBarTheme.copyWith(centerTitle: true),
    filledButtonTheme: FilledButtonThemeData(style: buttonSize),
    outlinedButtonTheme: OutlinedButtonThemeData(style: buttonSize),
    textButtonTheme: TextButtonThemeData(style: buttonSize),
    // Рамка у полей ввода — общая: экран её не повторяет.
    inputDecorationTheme: base.inputDecorationTheme.copyWith(
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(tuning.inputRadius),
      ),
      contentPadding: const EdgeInsets.all(AppGap.medium),
    ),
  );
}

/// Поведение прокрутки, общее для всего приложения.
///
/// Android с 12-й версии показывает край списка растяжением содержимого:
/// при прокрутке до упора картинки и текст плывут. На крупном шрифте и
/// на фотографиях это читается как поломка, поэтому край показываем
/// по-старому — свечением у границы (`GlowingOverscrollIndicator`).
/// Свечение ничего не двигает: оно появляется на краю, гаснет само и
/// говорит ровно одно — дальше прокручивать нечего.
///
/// Физика прокрутки при этом материаловая, то есть список упирается
/// в край, а не отпружинивает.
class AppScrollBehavior extends MaterialScrollBehavior {
  const AppScrollBehavior();

  @override
  Widget buildOverscrollIndicator(
    BuildContext context,
    Widget child,
    ScrollableDetails details,
  ) {
    switch (getPlatform(context)) {
      case TargetPlatform.iOS:
      case TargetPlatform.macOS:
        // Там прокрутка упругая, край виден и без указателя.
        return child;
      case TargetPlatform.android:
      case TargetPlatform.fuchsia:
      case TargetPlatform.linux:
      case TargetPlatform.windows:
        return GlowingOverscrollIndicator(
          axisDirection: details.direction,
          color: Theme.of(context).colorScheme.secondary,
          child: child,
        );
    }
  }
}
