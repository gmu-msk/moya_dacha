// Конец сессии закрывает открытые экраны.
//
// Баг с показа 2026-09-21: выход из аккаунта живёт в профиле
// (specs/004-feed.md), а профиль открыт поверх ленты. Приложение токен
// забывало, но человек продолжал видеть свой профиль: смена `home` меняет
// только нижний экран стопки. Проверяется на профиле не случайно — он
// перехватывает «назад» (`PopScope`), и закрыть его надо всё равно.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/app_scope.dart';
import 'package:moya_dacha/main.dart';
import 'package:moya_dacha/screens/login_screen.dart';
import 'package:moya_dacha/screens/profile_screen.dart';
import 'package:moya_dacha_api/api.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  // Хранилище на устройстве в тестах недоступно; подменяем его пустым,
  // как на чистом телефоне.
  TestWidgetsFlutterBinding.ensureInitialized();
  SharedPreferences.setMockInitialValues(const <String, Object>{});

  final user = CurrentUser(
    id: '00000000-0000-0000-0000-000000000001',
    phone: '+79000000001',
    createdAt: DateTime(2026, 4, 1),
    nickname: 'petya_kamaz',
    nicknameChosen: true,
    closed: false,
    name: 'Пётр',
    about: 'Три сотки под картошку',
  );

  testWidgets('профиль поверх ленты закрывается вместе с сессией', (
    tester,
  ) async {
    await tester.pumpWidget(const MoyaDachaApp());
    await tester.pumpAndSettle();

    final navigator = tester.state<NavigatorState>(find.byType(Navigator));
    unawaited(
      navigator.push(
        MaterialPageRoute<CurrentUser>(
          builder: (_) =>
              ProfileScreen(token: 'т', user: user, onSignedOut: () async {}),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(ProfileScreen), findsOneWidget);

    // Приложение начинает жизнь заново — этим кончается и выход
    // из аккаунта, и смена адреса сервера.
    final scope = AppScope.of(tester.element(find.byType(ProfileScreen)))!;
    unawaited(scope.restart());
    await tester.pumpAndSettle();

    expect(find.byType(ProfileScreen), findsNothing);
    expect(find.byType(LoginScreen), findsOneWidget);
  });

  testWidgets('выход с главного экрана ничего не ломает', (tester) async {
    final navigator = GlobalKey<NavigatorState>();
    await tester.pumpWidget(
      MaterialApp(
        navigatorKey: navigator,
        home: const Scaffold(body: Text('Экран входа')),
      ),
    );

    closePushedScreens(navigator.currentState);
    await tester.pumpAndSettle();

    expect(find.text('Экран входа'), findsOneWidget);
  });
}
