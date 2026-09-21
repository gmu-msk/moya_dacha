// Экран входа: номер, код, таймеры (specs/001-auth.md).
//
// Почти всё, что человек замечает на этом экране, живёт в приложении:
// маска номера, неактивная кнопка, обратный счётчик, место ошибки. Гейт
// проекта (ADR-0002) про это ничего не знает, а на телефоне такое
// проверяется минутами — поэтому проверки здесь.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/screens/login_screen.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha_api/api.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('цифры встают по маске, подсказка остаётся видной', (
    tester,
  ) async {
    await _pump(tester, _Auth());

    await _typePhone(tester, '9152');
    expect(_fieldText(tester), '+7(915)200-00-00');

    await _typePhone(tester, '9152345678');
    expect(_fieldText(tester), '+7(915)234-56-78');
  });

  testWidgets('буква в поле не появляется, форма подсвечивается', (
    tester,
  ) async {
    await _pump(tester, _Auth());

    await _typePhone(tester, '915ф');

    expect(_fieldText(tester), '+7(000)000-00-00');
    expect(_borderIsAlarming(tester), isTrue);
  });

  testWidgets('«Получить код» ждёт полного номера', (tester) async {
    final auth = _Auth();
    await _pump(tester, auth);
    expect(_requestEnabled(tester), isFalse);

    await _typePhone(tester, '915234567');
    expect(_requestEnabled(tester), isFalse);

    await _typePhone(tester, '9152345678');
    expect(_requestEnabled(tester), isTrue);

    await tester.tap(find.text('Получить код'));
    await tester.pump();
    expect(auth.requested, ['+7(915)234-56-78']);
  });

  testWidgets('код уходит сам, как только набраны четыре цифры', (
    tester,
  ) async {
    final auth = _Auth();
    var signedIn = false;
    await _pump(tester, auth, onSignedIn: () => signedIn = true);
    await _reachCode(tester);

    await tester.enterText(find.byType(TextField), '12');
    await tester.pump();
    expect(auth.entered, isEmpty, reason: 'кода ещё нет');
    expect(_fieldText(tester), '12__');

    await tester.enterText(find.byType(TextField), '1234');
    await tester.pumpAndSettle();

    expect(auth.entered, ['1234']);
    expect(signedIn, isTrue);
  });

  testWidgets('счётчик повторной отправки идёт от ответа сервиса', (
    tester,
  ) async {
    await _pump(tester, _Auth(resendAfter: 3));
    await _reachCode(tester);

    expect(find.text('Отправить код ещё раз через 3 сек'), findsOneWidget);
    expect(_resendEnabled(tester), isFalse);

    await _wait(tester, 2);
    expect(find.text('Отправить код ещё раз через 1 сек'), findsOneWidget);

    await _wait(tester, 1);
    expect(find.text('Отправить код ещё раз'), findsOneWidget);
    expect(_resendEnabled(tester), isTrue);
  });

  testWidgets('«код уже отправлен» показан с убывающим счётчиком', (
    tester,
  ) async {
    final auth = _Auth()
      ..codeError = _refusal(429, 'too_many_requests', 'На этот номер код уже отправлен', retryAfter: 3);
    await _pump(tester, auth);

    await _typePhone(tester, '9152345678');
    await tester.tap(find.text('Получить код'));
    await tester.pump();

    expect(
      find.textContaining('На номер +7(915)234-56-78 код уже был отправлен'),
      findsOneWidget,
    );
    expect(find.textContaining('через 3 сек'), findsOneWidget);
    expect(_requestEnabled(tester), isFalse);

    await _wait(tester, 1);
    expect(find.textContaining('через 2 сек'), findsOneWidget);

    await _wait(tester, 2);
    expect(find.textContaining('код уже был отправлен'), findsNothing);
    expect(_requestEnabled(tester), isTrue);
  });

  testWidgets('правка номера убирает отказ, не дожидаясь счётчика', (
    tester,
  ) async {
    final auth = _Auth()
      ..codeError = _refusal(429, 'too_many_requests', 'На этот номер код уже отправлен', retryAfter: 30);
    await _pump(tester, auth);

    await _typePhone(tester, '9152345678');
    await tester.tap(find.text('Получить код'));
    await tester.pump();
    expect(_requestEnabled(tester), isFalse);

    await _typePhone(tester, '9152345679');

    expect(find.textContaining('код уже был отправлен'), findsNothing);
    expect(_requestEnabled(tester), isTrue);
  });

  testWidgets('неверный код сказан под полем, и поле чистое', (tester) async {
    final auth = _Auth()
      ..signInError = _refusal(401, 'invalid_code', 'Неверный код');
    await _pump(tester, auth);
    await _reachCode(tester);

    await tester.enterText(find.byType(TextField), '1239');
    await tester.pumpAndSettle();

    expect(find.text('Неверный код'), findsOneWidget);
    expect(_fieldText(tester), '____', reason: 'следующий код вводится сразу');
  });

  testWidgets('«Другой номер» возвращает пустое поле номера', (tester) async {
    await _pump(tester, _Auth());
    await _reachCode(tester);

    await tester.tap(find.text('Другой номер'));
    await tester.pumpAndSettle();

    expect(find.text('Вход в аккаунт'), findsOneWidget);
    expect(_fieldText(tester), '+7(000)000-00-00');
  });
}

/// Сервис, который отвечает так, как нужно проверке.
class _Auth implements AuthGateway {
  _Auth({this.resendAfter = 60});

  final int resendAfter;

  /// Чем сервис отвечает на запрос кода и на вход, если не успехом.
  Exception? codeError;
  Exception? signInError;

  final List<String> requested = [];
  final List<String> entered = [];

  @override
  Future<AuthCodeAccepted> requestCode(String phone) async {
    requested.add(phone);
    final error = codeError;
    if (error != null) {
      throw error;
    }
    return AuthCodeAccepted(resendAfter: resendAfter, codeTtl: 300);
  }

  @override
  Future<SessionCreated> signIn(String phone, String code) async {
    entered.add(code);
    final error = signInError;
    if (error != null) {
      throw error;
    }
    return SessionCreated(
      token: 'т',
      isNewUser: false,
      user: CurrentUser(
        id: '00000000-0000-0000-0000-000000000001',
        phone: '+79152345678',
        createdAt: DateTime(2026, 4, 1),
        name: 'Пётр',
        about: '',
      ),
    );
  }
}

/// Отказ сервиса — ровно в том виде, в каком его видит приложение.
ApiException _refusal(
  int status,
  String code,
  String message, {
  int? retryAfter,
}) => ApiException(
  status,
  jsonEncode({
    'code': code,
    'message': message,
    'retry_after': ?retryAfter,
  }),
);

Future<void> _pump(
  WidgetTester tester,
  AuthGateway auth, {
  void Function()? onSignedIn,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(Brightness.light),
      home: LoginScreen(
        auth: auth,
        onSignedIn: (_) async => onSignedIn?.call(),
      ),
    ),
  );
  await tester.pump();
}

/// Ввести номер так, как его вводит человек: одними цифрами.
Future<void> _typePhone(WidgetTester tester, String digits) async {
  await tester.enterText(find.byType(TextField), digits);
  await tester.pump();
}

/// Дойти до экрана кода.
Future<void> _reachCode(WidgetTester tester) async {
  await _typePhone(tester, '9152345678');
  await tester.tap(find.text('Получить код'));
  await tester.pumpAndSettle();
  expect(find.text('Введите код'), findsOneWidget);
}

/// Прождать [seconds] секунд так, как их отсчитывает экран.
Future<void> _wait(WidgetTester tester, int seconds) async {
  for (var i = 0; i < seconds; i++) {
    await tester.pump(const Duration(seconds: 1));
  }
}

String _fieldText(WidgetTester tester) =>
    tester.widget<TextField>(find.byType(TextField)).controller!.text;

bool _requestEnabled(WidgetTester tester) =>
    tester.widget<FilledButton>(find.byType(FilledButton)).onPressed != null;

bool _resendEnabled(WidgetTester tester) => tester
        .widget<TextButton>(
          find.ancestor(
            of: find.textContaining('Отправить код ещё раз'),
            matching: find.byType(TextButton),
          ),
        )
        .onPressed !=
    null;

/// Подсвечена ли форма ввода: отказ виден не одним цветом, но цвет
/// проверяется проще всего (specs/000-ui.md, правило 17).
bool _borderIsAlarming(WidgetTester tester) {
  final field = tester.widget<TextField>(find.byType(TextField));
  final border = field.decoration?.focusedBorder;
  return border != null &&
      border.borderSide.color ==
          appTheme(Brightness.light).colorScheme.error;
}
