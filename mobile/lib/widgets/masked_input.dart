// Поле с записью известного вида: номер телефона, код из СМС.
//
// Подсказка показана целиком и никуда не девается: человек видит
// `+7(000)000-00-00` и вводит цифры на места нулей — введённое становится
// тёмным, незаполненный хвост остаётся светлым (specs/000-ui.md,
// правило 18). Недопустимый символ в поле не попадает: вместо него поле
// подсвечивается и качается (правило 17), а стирать потом нечего.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../theme.dart';

final _notDigit = RegExp(r'\D');
final _letters = RegExp(r'\p{L}', unicode: true);

/// Запись: что человек видит в пустом поле и куда встают цифры.
@immutable
class InputMask {
  const InputMask({required this.skeleton, required this.slots});

  /// Поле целиком, пока не введено ни одной цифры.
  final String skeleton;

  /// Места цифр в [skeleton], по порядку ввода.
  final List<int> slots;

  /// Сколько цифр принимает поле.
  int get length => slots.length;

  /// Поле с введёнными цифрами на своих местах.
  String apply(String digits) {
    final chars = skeleton.split('');
    for (var i = 0; i < digits.length && i < slots.length; i++) {
      chars[slots[i]] = digits[i];
    }
    return chars.join();
  }

  /// Где стоит курсор, когда введено [typed] цифр: сразу за последней.
  int caret(int typed) =>
      typed >= slots.length ? skeleton.length : slots[typed];
}

/// Номер телефона. `+7` стоит в поле сразу и не стирается: лишние цифры
/// человеку писать незачем (specs/001-auth.md, требование 3).
const phoneMask = InputMask(
  skeleton: '+7(000)000-00-00',
  slots: [3, 4, 5, 7, 8, 9, 11, 12, 14, 15],
);

/// Код из СМС: четыре знакоместа, и видно, что их четыре.
const codeMask = InputMask(skeleton: '____', slots: [0, 1, 2, 3]);

/// Цифры номера без кода страны: `89152345678`, `+7 915 234-56-78` и
/// `9152345678` — один и тот же номер (specs/001-auth.md, требование 2).
String phoneDigits(String raw) {
  var digits = raw.replaceAll(_notDigit, '');
  if (digits.length == 11 && (digits.startsWith('7') || digits.startsWith('8'))) {
    digits = digits.substring(1);
  }
  if (digits.length > phoneMask.length) {
    digits = digits.substring(0, phoneMask.length);
  }
  return digits;
}

/// Содержимое поля с маской.
///
/// Хранит введённые цифры, а не текст: текст — это всегда маска с ними
/// на своих местах, и собрать его можно в любой момент.
class MaskedController extends TextEditingController {
  MaskedController({required this.mask, this.boldPrefix = 0})
    : super.fromValue(
        TextEditingValue(
          text: mask.skeleton,
          // Курсор сразу за неизменной частью: человек начинает вводить
          // со следующей цифры.
          selection: TextSelection.collapsed(offset: mask.caret(0)),
        ),
      );

  final InputMask mask;

  /// Сколько знаков в начале показывать заметнее: `+7` у номера.
  final int boldPrefix;

  /// Цвет незаполненного хвоста. Ставится экраном из темы.
  Color? hintColor;

  String _digits = '';

  /// Введённые цифры без разделителей.
  String get digits => _digits;

  /// Все ли цифры введены.
  bool get complete => _digits.length == mask.length;

  set digits(String value) {
    super.value = applyDigits(value);
  }

  /// Содержимое поля с этими цифрами. Само поле не трогает: правку
  /// применяет тот, кто её разбирал, иначе она уедет дважды.
  TextEditingValue applyDigits(String value) {
    _digits = value;
    return TextEditingValue(
      text: mask.apply(value),
      selection: TextSelection.collapsed(offset: mask.caret(value.length)),
    );
  }

  @override
  void clear() {
    digits = '';
  }

  @override
  TextSpan buildTextSpan({
    required BuildContext context,
    TextStyle? style,
    required bool withComposing,
  }) {
    final boundary = mask.caret(_digits.length);
    final hint = style?.copyWith(color: hintColor);
    return TextSpan(
      style: style,
      children: [
        if (boldPrefix > 0)
          TextSpan(
            text: text.substring(0, boldPrefix),
            style: style?.copyWith(fontWeight: FontWeight.w500),
          ),
        TextSpan(text: text.substring(boldPrefix, boundary)),
        TextSpan(text: text.substring(boundary), style: hint),
      ],
    );
  }
}

/// Поле с маской: показывает [controller], сообщает о набранном и о том,
/// что символ не принят.
class MaskedField extends StatefulWidget {
  const MaskedField({
    super.key,
    required this.controller,
    required this.label,
    this.autofocus = false,
    this.centered = false,
    this.textStyle,
    this.onChanged,
  });

  final MaskedController controller;
  final String label;
  final bool autofocus;

  /// Код из СМС стоит по центру поля, номер телефона — по левому краю.
  final bool centered;

  final TextStyle? textStyle;

  /// Вызывается после каждой принятой правки.
  final ValueChanged<String>? onChanged;

  @override
  State<MaskedField> createState() => _MaskedFieldState();
}

class _MaskedFieldState extends State<MaskedField>
    with SingleTickerProviderStateMixin {
  late final AnimationController _shake = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 400),
  )..addStatusListener((status) {
    if (status == AnimationStatus.completed && mounted) {
      setState(() => _rejected = false);
    }
  });

  /// Последний символ не принят: форма подсвечена и качается.
  bool _rejected = false;

  @override
  void dispose() {
    _shake.dispose();
    super.dispose();
  }

  void _reject() {
    setState(() => _rejected = true);
    _shake.forward(from: 0);
  }

  /// Правка поля: из старого и нового текста видно, что человек сделал —
  /// набрал знак, стёр знак или вставил номер из буфера.
  ///
  /// Буква в поле для цифр не принимается вовсе: отказ виден подсветкой
  /// и покачиванием (specs/000-ui.md, правило 17).
  TextEditingValue _edit(TextEditingValue before, TextEditingValue after) {
    final controller = widget.controller;
    final mask = controller.mask;
    final was = controller.digits;
    final canonical = mask.apply(was);
    final fresh = after.text;

    if (fresh == canonical) {
      return before;
    }
    if (_letters.hasMatch(fresh)) {
      _reject();
      return before;
    }

    String digits;
    if (fresh.length > canonical.length) {
      // Набрано или вставлено: берём только что появившийся кусок.
      final added = fresh.length - canonical.length;
      final at = after.selection.baseOffset - added;
      final chunk = at >= 0 && at + added <= fresh.length
          ? fresh.substring(at, at + added)
          : fresh;
      final typed = chunk.replaceAll(_notDigit, '');
      if (typed.isEmpty) {
        _reject();
        return before;
      }
      digits = was + typed;
    } else if (_shorterByOne(fresh, canonical)) {
      // Стёрли знак: уходит последняя введённая цифра, а не знак маски.
      digits = was.isEmpty ? was : was.substring(0, was.length - 1);
    } else {
      // Поле переписали целиком: так приходит вставка из буфера и
      // подстановка клавиатурой.
      digits = fresh.replaceAll(_notDigit, '');
    }

    digits = mask == phoneMask
        ? phoneDigits(digits)
        : digits.substring(0, math.min(digits.length, mask.length));

    final value = controller.applyDigits(digits);
    if (digits != was) {
      widget.onChanged?.call(digits);
    }
    return value;
  }

  /// Стал ли текст прежним без одного знака.
  bool _shorterByOne(String fresh, String canonical) {
    if (fresh.length != canonical.length - 1) {
      return false;
    }
    for (var i = 0; i < canonical.length; i++) {
      if (canonical.substring(0, i) + canonical.substring(i + 1) == fresh) {
        return true;
      }
    }
    return false;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    widget.controller.hintColor = scheme.outline;

    // Рамка отказа — общая рамка полей из темы, перекрашенная: своего
    // скругления экран не придумывает (specs/000-ui.md, правило 9).
    final shape = theme.inputDecorationTheme.border;
    final border = (shape is OutlineInputBorder ? shape : const OutlineInputBorder())
        .copyWith(borderSide: BorderSide(color: scheme.error, width: 2));

    final field = TextField(
      key: const Key('masked-field'),
      controller: widget.controller,
      autofocus: widget.autofocus,
      keyboardType: TextInputType.number,
      style: widget.textStyle,
      textAlign: widget.centered ? TextAlign.center : TextAlign.start,
      inputFormatters: [TextInputFormatter.withFunction(_edit)],
      decoration: InputDecoration(
        labelText: widget.label,
        // Отказ виден и цветом, и движением: одного цвета мало
        // (specs/000-ui.md, правило 7).
        enabledBorder: _rejected ? border : null,
        focusedBorder: _rejected ? border : null,
        labelStyle: _rejected ? TextStyle(color: scheme.error) : null,
        filled: _rejected,
        fillColor: scheme.errorContainer.withValues(alpha: 0.35),
      ),
    );

    return AnimatedBuilder(
      animation: _shake,
      builder: (context, child) => Transform.translate(
        offset: Offset(
          math.sin(_shake.value * math.pi * 6) * (1 - _shake.value) * AppGap.small,
          0,
        ),
        child: child,
      ),
      child: field,
    );
  }
}
