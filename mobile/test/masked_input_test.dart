// Запись номера и кода: specs/000-ui.md, правило 18.
//
// Маска — чистый счёт по знакоместам, и ошибиться в ней легко: проверки
// дешевле, чем искать съехавшую скобку на телефоне.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/masked_input.dart';

void main() {
  test('введённые цифры встают на места нулей, хвост остаётся', () {
    expect(phoneMask.apply(''), '+7(000)000-00-00');
    expect(phoneMask.apply('915'), '+7(915)000-00-00');
    expect(phoneMask.apply('9152'), '+7(915)200-00-00');
    expect(phoneMask.apply('9152345678'), '+7(915)234-56-78');
  });

  test('курсор стоит сразу за введённым, а пустое поле — сразу за +7', () {
    expect(phoneMask.caret(0), 3);
    expect(phoneMask.caret(3), 7);
    expect(phoneMask.caret(10), '+7(915)234-56-78'.length);
  });

  test('код из СМС виден четырьмя знакоместами', () {
    expect(codeMask.apply(''), '____');
    expect(codeMask.apply('12'), '12__');
    expect(codeMask.apply('1234'), '1234');
  });

  test('номер в любом привычном виде — один и тот же номер', () {
    expect(phoneDigits('9152345678'), '9152345678');
    expect(phoneDigits('89152345678'), '9152345678');
    expect(phoneDigits('+7 915 234-56-78'), '9152345678');
    expect(phoneDigits('+7(915)234-56-78'), '9152345678');
    // Лишнее в конце отбрасывается: больше десяти цифр номер не вмещает.
    expect(phoneDigits('915234567890'), '9152345678');
  });

  testWidgets('у клеток кода не видно чёрточек маски', (tester) async {
    final controller = MaskedController(mask: codeMask);
    await tester.pumpWidget(
      MaterialApp(
        theme: appTheme(Brightness.light),
        home: Scaffold(
          body: MaskedField(controller: controller, label: 'Код', boxes: true),
        ),
      ),
    );
    controller.digits = '12';
    await tester.pump();

    // Невидимое поле лежит поверх клеток: незаполненный хвост `__` в нём
    // тоже должен быть прозрачным, иначе посреди клеток видны чёрточки.
    final span = controller.buildTextSpan(
      context: tester.element(find.byType(TextField)),
      style: const TextStyle(color: Colors.transparent),
      withComposing: false,
    );
    span.visitChildren((child) {
      final text = (child as TextSpan).text ?? '';
      if (text.contains('_')) {
        final color = child.style?.color ?? span.style?.color;
        expect(color?.a, 0, reason: 'хвост «$text» виден');
      }
      return true;
    });
  });
}
