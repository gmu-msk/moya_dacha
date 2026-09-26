// Поле с записью известного вида: номер телефона, код из СМС.
//
// Подсказка показана целиком и никуда не девается: человек видит
// `+7(000)000-00-00` и вводит цифры на места нулей (specs/000-ui.md,
// правило 18). Недопустимый символ в поле не попадает: вместо него поле
// подсвечивается и качается (правило 17).
//
// Код из СМС показывается четырьмя клетками (макет «Сад», 2a): активная
// клетка обведена основной краской, введённая — тёмной; когда код набран
// и проверяется, клетки заливаются и по очереди приподнимаются.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../theme.dart';

final _notDigit = RegExp(r'\D');
final _letters = RegExp(r'\p{L}', unicode: true);

@immutable
class InputMask {
  const InputMask({required this.skeleton, required this.slots});

  final String skeleton;
  final List<int> slots;

  int get length => slots.length;

  String apply(String digits) {
    final chars = skeleton.split('');
    for (var i = 0; i < digits.length && i < slots.length; i++) {
      chars[slots[i]] = digits[i];
    }
    return chars.join();
  }

  int caret(int typed) =>
      typed >= slots.length ? skeleton.length : slots[typed];
}

const phoneMask = InputMask(
  skeleton: '+7(000)000-00-00',
  slots: [3, 4, 5, 7, 8, 9, 11, 12, 14, 15],
);

const codeMask = InputMask(skeleton: '____', slots: [0, 1, 2, 3]);

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

class MaskedController extends TextEditingController {
  MaskedController({required this.mask, this.boldPrefix = 0})
    : super.fromValue(
        TextEditingValue(
          text: mask.skeleton,
          selection: TextSelection.collapsed(offset: mask.caret(0)),
        ),
      );

  final InputMask mask;
  final int boldPrefix;
  Color? hintColor;

  String _digits = '';

  String get digits => _digits;

  bool get complete => _digits.length == mask.length;

  set digits(String value) {
    super.value = applyDigits(value);
  }

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

class MaskedField extends StatefulWidget {
  const MaskedField({
    super.key,
    required this.controller,
    required this.label,
    this.autofocus = false,
    this.centered = false,
    this.boxes = false,
    this.done = false,
    this.textStyle,
    this.onChanged,
  });

  final MaskedController controller;
  final String label;
  final bool autofocus;
  final bool centered;

  /// Показать клетками — по одной на цифру (код из СМС).
  final bool boxes;

  /// Код набран и проверяется: клетки залиты основной краской.
  final bool done;

  final TextStyle? textStyle;
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

  bool _rejected = false;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_keepCaretAtTheEnd);
  }

  @override
  void didUpdateWidget(MaskedField old) {
    super.didUpdateWidget(old);
    if (old.controller != widget.controller) {
      old.controller.removeListener(_keepCaretAtTheEnd);
      widget.controller.addListener(_keepCaretAtTheEnd);
    }
  }

  @override
  void dispose() {
    widget.controller.removeListener(_keepCaretAtTheEnd);
    _shake.dispose();
    super.dispose();
  }

  void _keepCaretAtTheEnd() {
    final controller = widget.controller;
    final selection = controller.selection;
    if (!selection.isValid || !selection.isCollapsed) {
      return;
    }
    final end = controller.mask.caret(controller.digits.length);
    if (selection.baseOffset != end) {
      controller.selection = TextSelection.collapsed(offset: end);
    }
  }

  void _reject() {
    setState(() => _rejected = true);
    _shake.forward(from: 0);
  }

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
      digits = was.isEmpty ? was : was.substring(0, was.length - 1);
    } else {
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

    final shape = theme.inputDecorationTheme.border;
    final border = (shape is OutlineInputBorder ? shape : const OutlineInputBorder())
        .copyWith(borderSide: BorderSide(color: scheme.error, width: 2));

    final Widget field;
    if (widget.boxes) {
      // Настоящее поле лежит поверх клеток невидимым: оно принимает ввод,
      // вставку и клавиатуру, а видно только клетки.
      field = Stack(
        children: [
          ListenableBuilder(
            listenable: widget.controller,
            builder: (context, _) => _CodeBoxes(
              digits: widget.controller.digits,
              length: widget.controller.mask.length,
              done: widget.done,
              rejected: _rejected,
              style: widget.textStyle ?? theme.textTheme.headlineMedium,
            ),
          ),
          Positioned.fill(
            child: TextField(
              key: const Key('masked-field'),
              controller: widget.controller,
              autofocus: widget.autofocus,
              keyboardType: TextInputType.number,
              showCursor: false,
              enableInteractiveSelection: false,
              style: const TextStyle(color: Colors.transparent),
              inputFormatters: [TextInputFormatter.withFunction(_edit)],
              decoration: InputDecoration(
                filled: false,
                border: InputBorder.none,
                enabledBorder: _rejected ? border : InputBorder.none,
                focusedBorder: _rejected ? border : InputBorder.none,
                contentPadding: EdgeInsets.zero,
                semanticCounterText: widget.label,
              ),
            ),
          ),
        ],
      );
    } else {
      field = TextField(
        key: const Key('masked-field'),
        controller: widget.controller,
        autofocus: widget.autofocus,
        keyboardType: TextInputType.number,
        style: widget.textStyle,
        textAlign: widget.centered ? TextAlign.center : TextAlign.start,
        inputFormatters: [TextInputFormatter.withFunction(_edit)],
        decoration: InputDecoration(
          labelText: widget.label,
          enabledBorder: _rejected ? border : null,
          focusedBorder: _rejected ? border : null,
          labelStyle: _rejected ? TextStyle(color: scheme.error) : null,
          fillColor: _rejected
              ? scheme.errorContainer.withValues(alpha: 0.35)
              : null,
        ),
      );
    }

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

/// Клетки кода: 62×70, скругление полей, между клетками 12.
class _CodeBoxes extends StatelessWidget {
  const _CodeBoxes({
    required this.digits,
    required this.length,
    required this.done,
    required this.rejected,
    required this.style,
  });

  final String digits;
  final int length;
  final bool done;
  final bool rejected;
  final TextStyle? style;

  static const _size = Size(62, 70);

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final shape = theme.inputDecorationTheme.border;
    final radius = shape is OutlineInputBorder
        ? shape.borderRadius
        : BorderRadius.circular(AppShape.medium);

    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        for (var i = 0; i < length; i++) ...[
          if (i > 0) const SizedBox(width: AppGap.snug),
          _box(scheme, radius, i),
        ],
      ],
    );
  }

  Widget _box(ColorScheme scheme, BorderRadius radius, int i) {
    final ch = i < digits.length ? digits[i] : '';
    final active = i == digits.length && !done;
    final line = rejected
        ? scheme.error
        : done || active
        ? scheme.primary
        : ch.isNotEmpty
        ? scheme.onSurfaceVariant
        : scheme.outlineVariant;

    return AnimatedSlide(
      offset: Offset(0, done ? -0.09 : 0),
      duration: AppMotion.quick + AppMotion.stagger * (done ? i : 0),
      curve: AppMotion.spring,
      child: AnimatedScale(
        scale: active ? 1.06 : 1,
        duration: AppMotion.quick,
        curve: AppMotion.spring,
        child: AnimatedContainer(
          duration: AppMotion.quick,
          width: _size.width,
          height: _size.height,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: done ? scheme.primary : scheme.surfaceContainerLowest,
            borderRadius: radius,
            border: Border.all(color: line, width: 2),
          ),
          child: Text(
            ch,
            style: style?.copyWith(
              color: done ? scheme.onPrimary : scheme.onSurface,
            ),
          ),
        ),
      ),
    );
  }
}
