// Жалоба на чужое: specs/008-reports.md.
//
// Проверяется то, чего гейт проекта не видит: «Пожаловаться» есть
// только у чужого, перед отправкой спрашивают, а когда сервис не
// ответил, написанное не пропадает (требования 11 и 12).
//
// Сети в виджет-тесте нет: любой запрос отвечает ошибкой. Это и нужно
// для последней проверки.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/screens/post_screen.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha_api/api.dart';

const _mine = '00000000-0000-0000-0000-000000000001';
const _someoneElse = '00000000-0000-0000-0000-000000000002';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('у чужого поста есть «Пожаловаться»', (tester) async {
    await _pumpPost(tester, authorId: _someoneElse, viewerId: _mine);

    expect(find.byIcon(Icons.flag_outlined), findsOneWidget);
  });

  testWidgets('у своего поста «Пожаловаться» нет', (tester) async {
    await _pumpPost(tester, authorId: _mine, viewerId: _mine);

    // У своего — «Удалить»: своё убирают сами (specs/007-deletion.md).
    expect(find.byIcon(Icons.flag_outlined), findsNothing);
    expect(find.byIcon(Icons.delete_outline), findsOneWidget);
  });

  testWidgets('перед жалобой спрашивают', (tester) async {
    await _pumpPost(tester, authorId: _someoneElse, viewerId: _mine);

    await tester.tap(find.byIcon(Icons.flag_outlined));
    await tester.pumpAndSettle();

    expect(find.text('Пожаловаться на пост?'), findsOneWidget);
    expect(find.text('Что не так?'), findsOneWidget);
  });

  testWidgets('отказ ничего не отправляет', (tester) async {
    await _pumpPost(tester, authorId: _someoneElse, viewerId: _mine);

    await tester.tap(find.byIcon(Icons.flag_outlined));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Отмена'));
    await tester.pumpAndSettle();

    // Вопрос закрылся, а «жалоба отправлена» не появилось: на сервис
    // никто не ходил.
    expect(find.text('Пожаловаться на пост?'), findsNothing);
    expect(find.byType(SnackBar), findsNothing);
  });

  testWidgets('написанное не теряется, если сервис не ответил', (tester) async {
    await _pumpPost(tester, authorId: _someoneElse, viewerId: _mine);

    await tester.tap(find.byIcon(Icons.flag_outlined));
    await tester.pumpAndSettle();
    // Поле берётся из самого вопроса: под ним, на экране поста, есть
    // ещё одно — для комментария.
    final reason = find.descendant(
      of: find.byType(AlertDialog),
      matching: find.byType(TextField),
    );
    await tester.enterText(reason, 'Это не про дачу');
    await tester.tap(find.text('Пожаловаться'));
    await tester.pumpAndSettle();

    // Вопрос остался открытым вместе с написанным, и видно, что не
    // получилось: отправлять заново, набирая заново, человек не должен.
    expect(find.text('Пожаловаться на пост?'), findsOneWidget);
    expect(find.text('Это не про дачу'), findsOneWidget);
    expect(find.text('Жалоба отправлена. Мы её посмотрим'), findsNothing);
  });

  testWidgets('жалоба ничего не меняет на экране поста', (tester) async {
    await _pumpPost(tester, authorId: _someoneElse, viewerId: _mine);

    await tester.tap(find.byIcon(Icons.flag_outlined));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Отмена'));
    await tester.pumpAndSettle();

    // Пост на месте и такой же: жалоба — сигнал владельцу сервиса, а не
    // действие над постом (требование 12).
    expect(find.byType(PostScreen), findsOneWidget);
    expect(find.text('Первая клубника в этом году'), findsOneWidget);
    expect(find.byIcon(Icons.flag_outlined), findsOneWidget);
  });
}

Future<void> _pumpPost(
  WidgetTester tester, {
  required String authorId,
  required String viewerId,
}) async {
  final post = Post(
    id: '00000000-0000-0000-0000-00000000000a',
    createdAt: DateTime(2026, 6, 1, 9, 30),
    caption: 'Первая клубника в этом году',
    author: Author(id: authorId, nickname: 'kolya_kartofel', name: 'Николай'),
    // Фотографий нет нарочно: Image.network в тесте ходить некуда
    // (сами фотографии — на показе, ADR-0009).
    media: [],
    likes: 0,
    liked: false,
    comments: 0,
    visibility: PostVisibility.all,
  );

  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(Brightness.light),
      home: PostScreen(post: post, token: 'т', viewerId: viewerId),
    ),
  );
  await tester.pumpAndSettle();
}
