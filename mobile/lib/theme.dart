// Тема приложения: цвета, шрифты, размеры текста, отступы, движение.
//
// Вид — «Сад» (раунд 2 холста «Моя дача — редизайн», вариант 2a): льняное
// полотно, мох и томат. Шрифты прежние — Rubik для заголовков и Golos Text
// для текста, крупнее материаловых (ADR-0003). Карточек в ленте нет: пост
// лежит прямо на полотне, фото 4:5 со скруглением 6.
//
// Экраны не задают ни цветов, ни размеров шрифта, ни голых чисел отступа
// и длительностей: всё это живёт здесь (ADR-0012).
import 'package:flutter/material.dart';

/// Основная краска — мох: кнопки, выделение, главное действие.
const _primary = Color(0xFF34502C);

/// Вторая краска — чернила полотна: имена авторов и комментаторов.
/// В «Саду» имя выделено начертанием, а не цветом.
const _secondary = Color(0xFF1D231B);

/// Акцент — томат: сердечко «нравится», точка непрочитанного, мак
/// в логотипе.
const _accent = Color(0xFFD4442A);

/// Те же краски на тёмном полотне: светлее, чтобы читались как текст.
const _primaryDark = Color(0xFFB5CE98);
const _onPrimaryDark = Color(0xFF16200F);
const _secondaryDark = Color(0xFFEDE7DA);
const _accentDark = Color(0xFFFF7657);

/// Во сколько раз шрифт крупнее материалового по умолчанию.
const _fontScale = 1.25;

/// Наименьшая высота кнопки.
const _tapTargetHeight = 56.0;

/// Скругление рамки у полей ввода. Кнопки — «таблетки».
const _inputRadius = 14.0;

/// Скругление карточек вне ленты (плашки, диалоги, листы).
const _cardRadius = 14.0;

const _headingFont = 'Rubik';
const _bodyFont = 'Golos Text';

/// Величины темы, которые можно покрутить в песочнице витрины
/// (`gallery_screen.dart`, ADR-0012), не пересобирая приложение.
@immutable
class ThemeTuning {
  const ThemeTuning({
    this.primary = _primary,
    this.secondary = _secondary,
    this.accent = _accent,
    this.fontScale = _fontScale,
    this.tapTargetHeight = _tapTargetHeight,
    this.inputRadius = _inputRadius,
    this.cardRadius = _cardRadius,
  });

  final Color primary;
  final Color secondary;
  final Color accent;
  final double fontScale;
  final double tapTargetHeight;
  final double inputRadius;
  final double cardRadius;

  String asThemeConstants() {
    return 'const _primary = Color(0x${_hex(primary)});\n'
        'const _secondary = Color(0x${_hex(secondary)});\n'
        'const _accent = Color(0x${_hex(accent)});\n'
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
    Color? accent,
    double? fontScale,
    double? tapTargetHeight,
    double? inputRadius,
    double? cardRadius,
  }) {
    return ThemeTuning(
      primary: primary ?? this.primary,
      secondary: secondary ?? this.secondary,
      accent: accent ?? this.accent,
      fontScale: fontScale ?? this.fontScale,
      tapTargetHeight: tapTargetHeight ?? this.tapTargetHeight,
      inputRadius: inputRadius ?? this.inputRadius,
      cardRadius: cardRadius ?? this.cardRadius,
    );
  }
}

/// Отступы. Других чисел отступа в экранах быть не должно.
abstract final class AppGap {
  static const tiny = 4.0;
  static const small = 8.0;
  static const snug = 12.0;
  static const medium = 16.0;
  static const large = 24.0;

  /// Между постами в ленте: без карточек пост отделяет воздух.
  static const loose = 36.0;
}

/// Скругления и толщина линий.
abstract final class AppShape {
  /// Фотография поста в ленте.
  static const photo = 6.0;

  /// Мелкие плашки и превью.
  static const small = 8.0;

  /// Превью выбранных фото в новом посте, ячейки сетки.
  static const medium = 14.0;

  /// «Таблетка»: кнопки, переключатели, точки карусели.
  static const pill = 999.0;

  /// Линии разделителей и верх нижней панели.
  static const hairline = 1.5;
}

/// Движение. Числа сняты с макета (раунд 1, «Предложенные анимации»).
abstract final class AppMotion {
  /// Смена цвета, подложки, мелкие отклики.
  static const quick = Duration(milliseconds: 200);

  /// Переключатели, карусель, раскрытие фото.
  static const standard = Duration(milliseconds: 450);

  /// Выезд нового поста снизу.
  static const sheet = Duration(milliseconds: 550);

  /// Появление поста в ленте и шаг между соседними.
  static const entrance = Duration(milliseconds: 650);
  static const stagger = Duration(milliseconds: 85);

  /// Сердечко: подскок, лепестки, большое сердце при двойном касании.
  static const heartPop = Duration(milliseconds: 350);
  static const petals = Duration(milliseconds: 600);
  static const bigHeart = Duration(milliseconds: 900);

  /// Плавное торможение — почти всё движение приложения.
  static const ease = Cubic(0.2, 0.8, 0.2, 1);

  /// С лёгким перелётом: ползунок переключателя, подскок значков.
  static const spring = Cubic(0.3, 1.3, 0.5, 1);
}

/// Зелень логотипа и точки «сервер на связи».
abstract final class AppBrand {
  static Color stem(Brightness brightness) => brightness == Brightness.light
      ? const Color(0xFF6E8F4F)
      : const Color(0xFF86B070);

  static Color online(Brightness brightness) => stem(brightness);
}

/// Нейтраль «Сада» — льняное полотно.
ColorScheme _neutral(ColorScheme scheme, Brightness brightness) {
  if (brightness == Brightness.light) {
    return scheme.copyWith(
      surface: const Color(0xFFF2EDE3),
      onSurface: const Color(0xFF1D231B),
      onSurfaceVariant: const Color(0xFF66675A),
      surfaceContainerLowest: const Color(0xFFFAF7F1),
      surfaceContainerLow: const Color(0xFFF2EDE3),
      surfaceContainer: const Color(0xFFEDE7DC),
      surfaceContainerHigh: const Color(0xFFE9E2D4),
      surfaceContainerHighest: const Color(0xFFE7E0D1),
      outline: const Color(0xFF8E8B7C),
      outlineVariant: const Color(0xFFD8CFBF),
      error: const Color(0xFF8F3A1B),
      onError: const Color(0xFFFFFFFF),
      errorContainer: const Color(0xFFFBE1D4),
      onErrorContainer: const Color(0xFF3F1607),
    );
  }
  return scheme.copyWith(
    surface: const Color(0xFF12150F),
    onSurface: const Color(0xFFEDE7DA),
    onSurfaceVariant: const Color(0xFFA9A695),
    surfaceContainerLowest: const Color(0xFF0D100B),
    surfaceContainerLow: const Color(0xFF161A13),
    surfaceContainer: const Color(0xFF1A1E17),
    surfaceContainerHigh: const Color(0xFF20251C),
    surfaceContainerHighest: const Color(0xFF262B22),
    outline: const Color(0xFF6F6C60),
    outlineVariant: const Color(0xFF30362C),
    error: const Color(0xFFFFB59B),
    onError: const Color(0xFF55200A),
    errorContainer: const Color(0xFF73341A),
    onErrorContainer: const Color(0xFFFFDBCD),
  );
}

ColorScheme _scheme(Brightness brightness, ThemeTuning tuning) {
  final fromPrimary = ColorScheme.fromSeed(
    seedColor: tuning.primary,
    brightness: brightness,
  );
  final fromSecondary = ColorScheme.fromSeed(
    seedColor: tuning.secondary,
    brightness: brightness,
  );
  final fromAccent = ColorScheme.fromSeed(
    seedColor: tuning.accent,
    brightness: brightness,
  );
  final light = brightness == Brightness.light;

  // В тёмной теме свои, подобранные на макете тона; если краску поменяли
  // в песочнице — светлый тон той же краски выводит Material.
  final darkPrimary =
      tuning.primary == _primary ? _primaryDark : fromPrimary.primary;
  final darkOnPrimary =
      tuning.primary == _primary ? _onPrimaryDark : fromPrimary.onPrimary;
  final darkSecondary =
      tuning.secondary == _secondary ? _secondaryDark : fromSecondary.primary;
  final darkAccent =
      tuning.accent == _accent ? _accentDark : fromAccent.primary;

  return _neutral(
    fromPrimary.copyWith(
      primary: light ? tuning.primary : darkPrimary,
      onPrimary: light ? const Color(0xFFF6F2E8) : darkOnPrimary,
      secondary: light ? tuning.secondary : darkSecondary,
      onSecondary: light ? const Color(0xFFF2EDE3) : const Color(0xFF12150F),
      secondaryContainer: fromSecondary.primaryContainer,
      onSecondaryContainer: fromSecondary.onPrimaryContainer,
      tertiary: light ? tuning.accent : darkAccent,
      onTertiary: light ? const Color(0xFFFFFFFF) : const Color(0xFF12150F),
      tertiaryContainer: fromAccent.primaryContainer,
      onTertiaryContainer: fromAccent.onPrimaryContainer,
    ),
    brightness,
  );
}

/// Тема светлая или тёмная; какая показана, решает система.
ThemeData appTheme(
  Brightness brightness, {
  ThemeTuning tuning = const ThemeTuning(),
}) {
  final colorScheme = _scheme(brightness, tuning);
  final base = ThemeData(colorScheme: colorScheme, fontFamily: _bodyFont);

  // Кнопка — «таблетка» не ниже цели касания.
  final buttonSize = ButtonStyle(
    minimumSize: WidgetStatePropertyAll(Size(64, tuning.tapTargetHeight)),
    shape: const WidgetStatePropertyAll(StadiumBorder()),
  );

  return base.copyWith(
    typography: Typography.material2021(
      platform: base.platform,
      colorScheme: colorScheme,
      englishLike: Typography.englishLike2021.apply(
        fontSizeFactor: tuning.fontScale,
      ),
      dense: Typography.dense2021.apply(fontSizeFactor: tuning.fontScale),
      tall: Typography.tall2021.apply(fontSizeFactor: tuning.fontScale),
    ),
    textTheme: _headings(base.textTheme),
    visualDensity: VisualDensity.standard,
    materialTapTargetSize: MaterialTapTargetSize.padded,
    appBarTheme: base.appBarTheme.copyWith(
      centerTitle: true,
      backgroundColor: colorScheme.surface,
      surfaceTintColor: Colors.transparent,
    ),
    filledButtonTheme: FilledButtonThemeData(style: buttonSize),
    outlinedButtonTheme: OutlinedButtonThemeData(style: buttonSize),
    textButtonTheme: TextButtonThemeData(style: buttonSize),
    // Карточка — только вне ленты: светлая плашка без канта и тени.
    cardTheme: CardThemeData(
      color: brightness == Brightness.light
          ? colorScheme.surfaceContainerLowest
          : colorScheme.surfaceContainer,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(tuning.cardRadius),
      ),
    ),
    inputDecorationTheme: base.inputDecorationTheme.copyWith(
      filled: true,
      fillColor: brightness == Brightness.light
          ? colorScheme.surfaceContainerLowest
          : colorScheme.surfaceContainer,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(tuning.inputRadius),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(tuning.inputRadius),
        borderSide: BorderSide(
          color: colorScheme.outlineVariant,
          width: AppShape.hairline,
        ),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(tuning.inputRadius),
        borderSide: BorderSide(color: colorScheme.primary, width: 2),
      ),
      contentPadding: const EdgeInsets.all(AppGap.medium),
    ),
    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: colorScheme.onSurface,
      contentTextStyle: TextStyle(color: colorScheme.surface),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(tuning.inputRadius),
      ),
    ),
    progressIndicatorTheme: ProgressIndicatorThemeData(
      color: colorScheme.primary,
      refreshBackgroundColor: colorScheme.surfaceContainerLowest,
    ),
  );
}

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

/// Край списка — свечением, а не растяжением (см. историю файла).
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
        return child;
      case TargetPlatform.android:
      case TargetPlatform.fuchsia:
      case TargetPlatform.linux:
      case TargetPlatform.windows:
        return GlowingOverscrollIndicator(
          axisDirection: details.direction,
          color: Theme.of(context).colorScheme.primary,
          child: child,
        );
    }
  }
}
