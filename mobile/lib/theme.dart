// Тема приложения: цвета, шрифты, размеры текста, отступы и цели касания.
//
// Вид — «Ситец»: белое тёплое полотно, две краски (мак и василёк),
// карточка с тонким кантом и скруглением 16. Выбран владельцем из
// вариантов на холсте макетов (specs/000-ui.md, раздел «Вид»).
//
// Аудитория МояДачи — дачники, среди них много людей старшего возраста
// (ADR-0003). Поэтому базовый шрифт крупнее материалового, а кнопки выше
// того, что Material даёт по умолчанию.
//
// Экраны не задают ни цветов, ни размеров шрифта, ни голых чисел отступа:
// всё это живёт здесь (ADR-0012).
import 'package:flutter/material.dart';

/// Мак — основная краска: кнопки, отметка «нравится», главное действие.
const _poppy = Color(0xFFC7323C);

/// Василёк — вторая краска: имена авторов, комментарии, аватары соседей.
/// Две краски вместо одного цвета-семени: ситец узнаётся именно парой.
const _cornflower = Color(0xFF2F5AA8);

/// Во сколько раз шрифт крупнее материалового по умолчанию: основной текст
/// становится 17–18sp вместо 14sp. Системное увеличение шрифта этим не
/// отменяется, а умножается на него, поэтому экраны должны выживать и при
/// двукратном размере — это пункт чек-листа каждого сценария показа.
const _fontScale = 1.25;

/// Наименьшая высота кнопки. Material 3 даёт 40dp; рекомендация по целям
/// касания — 48dp, а нашим пользователям и этого мало.
const _tapTargetHeight = 56.0;

/// Скругление рамки у полей ввода и кнопок. Столько же у карточки:
/// в «Ситце» все крупные углы одинаковые.
const _inputRadius = 16.0;

/// Скругление карточки поста.
const _cardRadius = 16.0;

/// Шрифт заголовков: имена, названия экранов, подписи постов.
const _headingFont = 'Rubik';

/// Шрифт остального текста.
const _bodyFont = 'Golos Text';

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
    this.primary = _poppy,
    this.secondary = _cornflower,
    this.fontScale = _fontScale,
    this.tapTargetHeight = _tapTargetHeight,
    this.inputRadius = _inputRadius,
    this.cardRadius = _cardRadius,
  });

  /// Основная краска: кнопки и отметка «нравится».
  final Color primary;

  /// Вторая краска: имена и комментарии.
  final Color secondary;

  /// Множитель размера текста поверх материалового.
  final double fontScale;

  /// Наименьшая высота кнопки.
  final double tapTargetHeight;

  /// Скругление рамки полей ввода и кнопок.
  final double inputRadius;

  /// Скругление карточки поста.
  final double cardRadius;

  /// То же самое в виде констант этого файла: песочница витрины
  /// показывает их и даёт скопировать, чтобы прислать в задачу.
  String asThemeConstants() {
    return 'const _poppy = Color(0x${_hex(primary)});\n'
        'const _cornflower = Color(0x${_hex(secondary)});\n'
        'const _fontScale = ${fontScale.toStringAsFixed(2)};\n'
        'const _tapTargetHeight = ${tapTargetHeight.toStringAsFixed(1)};\n'
        'const _inputRadius = ${inputRadius.toStringAsFixed(1)};\n'
        'const _cardRadius = ${cardRadius.toStringAsFixed(1)};';
  }

  static String _hex(Color color) =>
      color.toARGB32().toRadixString(16).padLeft(8, '0').toUpperCase();

  ThemeTuning copyWith({
    Color? primary,
    Color? secondary,
    double? fontScale,
    double? tapTargetHeight,
    double? inputRadius,
    double? cardRadius,
  }) {
    return ThemeTuning(
      primary: primary ?? this.primary,
      secondary: secondary ?? this.secondary,
      fontScale: fontScale ?? this.fontScale,
      tapTargetHeight: tapTargetHeight ?? this.tapTargetHeight,
      inputRadius: inputRadius ?? this.inputRadius,
      cardRadius: cardRadius ?? this.cardRadius,
    );
  }
}

/// Отступы. Других чисел отступа в экранах быть не должно.
abstract final class AppGap {
  /// Впритык: между картинкой и подписью к ней, между значками в ряду.
  static const tiny = 4.0;

  /// Между строками одного блока.
  static const small = 8.0;

  /// Между блоками внутри экрана.
  static const medium = 16.0;

  /// Поля экрана и расстояние между его крупными частями.
  static const large = 24.0;
}

/// Скругления и толщина канта. Крупные углы (карточка, кнопка, поле)
/// живут в [ThemeTuning] — их крутит песочница; здесь то, что стоит рядом
/// и в песочнице не нужно.
abstract final class AppShape {
  /// Фотография внутри карточки: скругление меньше, чем у самой карточки.
  static const photo = 12.0;

  /// Кант карточки и разделительных линий.
  static const hairline = 1.5;
}

/// Нейтраль «Ситца» — тёплое белое полотно и белая карточка на нём.
/// От выбранных красок не зависит: меняются краски, полотно остаётся.
ColorScheme _neutral(ColorScheme scheme, Brightness brightness) {
  if (brightness == Brightness.light) {
    return scheme.copyWith(
      surface: const Color(0xFFFFFCF5),
      onSurface: const Color(0xFF221E1A),
      onSurfaceVariant: const Color(0xFF6B6257),
      surfaceContainerLowest: const Color(0xFFFFFFFF),
      surfaceContainerLow: const Color(0xFFFFFCF5),
      surfaceContainer: const Color(0xFFF9F3EA),
      surfaceContainerHigh: const Color(0xFFF6EFE4),
      surfaceContainerHighest: const Color(0xFFF3ECE0),
      outline: const Color(0xFF8C8175),
      outlineVariant: const Color(0xFFEADFD0),
      // Ошибка теплее и темнее мака: рядом с кнопкой её не спутать.
      // Одним цветом состояние всё равно не различается (правило 7).
      error: const Color(0xFF8F3A1B),
      onError: const Color(0xFFFFFFFF),
      errorContainer: const Color(0xFFFBE1D4),
      onErrorContainer: const Color(0xFF3F1607),
    );
  }
  return scheme.copyWith(
    surface: const Color(0xFF16130F),
    onSurface: const Color(0xFFEDE5DA),
    onSurfaceVariant: const Color(0xFFCFC4B6),
    surfaceContainerLowest: const Color(0xFF100E0B),
    surfaceContainerLow: const Color(0xFF1C1915),
    surfaceContainer: const Color(0xFF221E19),
    surfaceContainerHigh: const Color(0xFF2C2822),
    surfaceContainerHighest: const Color(0xFF37322B),
    outline: const Color(0xFF988D7F),
    outlineVariant: const Color(0xFF4B443B),
    error: const Color(0xFFFFB59B),
    onError: const Color(0xFF55200A),
    errorContainer: const Color(0xFF73341A),
    onErrorContainer: const Color(0xFFFFDBCD),
  );
}

/// Палитра из двух красок: мак задаёт основной цвет, василёк — второй.
/// Оттенки под них (заливка аватара, подложка отметки) Material выводит
/// сам, поэтому краску в песочнице можно заменить любой.
ColorScheme _scheme(Brightness brightness, ThemeTuning tuning) {
  final fromPrimary = ColorScheme.fromSeed(
    seedColor: tuning.primary,
    brightness: brightness,
  );
  final fromSecondary = ColorScheme.fromSeed(
    seedColor: tuning.secondary,
    brightness: brightness,
  );
  final light = brightness == Brightness.light;

  return _neutral(
    fromPrimary.copyWith(
      // Краска берётся как есть, не «гармонизируется»: в светлой теме
      // это заливка кнопки, в тёмной — цвет текста на тёмном, поэтому
      // там нужен светлый тон той же краски.
      primary: light ? tuning.primary : fromPrimary.primary,
      onPrimary: light ? const Color(0xFFFFFFFF) : fromPrimary.onPrimary,
      secondary: light ? tuning.secondary : fromSecondary.primary,
      onSecondary: light ? const Color(0xFFFFFFFF) : fromSecondary.onPrimary,
      secondaryContainer: fromSecondary.primaryContainer,
      onSecondaryContainer: fromSecondary.onPrimaryContainer,
    ),
    brightness,
  );
}

/// Тема светлая или тёмная. Какая из них показана, решает система
/// (`themeMode: ThemeMode.system`), своего переключателя в приложении нет.
ThemeData appTheme(
  Brightness brightness, {
  ThemeTuning tuning = const ThemeTuning(),
}) {
  final colorScheme = _scheme(brightness, tuning);
  final base = ThemeData(colorScheme: colorScheme, fontFamily: _bodyFont);

  // Кнопка не ниже tuning.tapTargetHeight, какой бы короткой ни была
  // надпись, и со скруглением как у карточки — стадион Material 3
  // рядом с прямоугольной карточкой смотрится из другой темы.
  final buttonSize = ButtonStyle(
    minimumSize: WidgetStatePropertyAll(Size(64, tuning.tapTargetHeight)),
    shape: WidgetStatePropertyAll(
      RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(tuning.inputRadius),
      ),
    ),
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
    // Заголовки — Rubik, остальное — Golos Text из ThemeData выше.
    // Размер здесь не ставится: его домешает геометрия.
    textTheme: _headings(base.textTheme),
    // Стандартная плотность, а не компактная: цели касания не ужимаются.
    visualDensity: VisualDensity.standard,
    materialTapTargetSize: MaterialTapTargetSize.padded,
    appBarTheme: base.appBarTheme.copyWith(centerTitle: true),
    filledButtonTheme: FilledButtonThemeData(style: buttonSize),
    outlinedButtonTheme: OutlinedButtonThemeData(style: buttonSize),
    textButtonTheme: TextButtonThemeData(style: buttonSize),
    // Карточка поста: белая на полотне, тонкий кант вместо тени.
    cardTheme: CardThemeData(
      color: brightness == Brightness.light
          ? colorScheme.surfaceContainerLowest
          : colorScheme.surfaceContainer,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(tuning.cardRadius),
        side: BorderSide(
          color: colorScheme.outlineVariant,
          width: AppShape.hairline,
        ),
      ),
    ),
    // Рамка у полей ввода — общая: экран её не повторяет.
    inputDecorationTheme: base.inputDecorationTheme.copyWith(
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(tuning.inputRadius),
      ),
      contentPadding: const EdgeInsets.all(AppGap.medium),
    ),
  );
}

/// Заголовочные стили на [_headingFont]. Размеры не трогаются: они
/// приходят из геометрии, когда `MaterialApp` локализует тему.
TextTheme _headings(TextTheme base) {
  TextStyle? rubik(TextStyle? style) =>
      style?.copyWith(fontFamily: _headingFont, fontWeight: FontWeight.w600);

  return base.copyWith(
    displayLarge: rubik(base.displayLarge),
    displayMedium: rubik(base.displayMedium),
    displaySmall: rubik(base.displaySmall),
    headlineLarge: rubik(base.headlineLarge),
    headlineMedium: rubik(base.headlineMedium),
    headlineSmall: rubik(base.headlineSmall),
    titleLarge: rubik(base.titleLarge),
    titleMedium: rubik(base.titleMedium),
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
