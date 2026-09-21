// Удаление своего: specs/007-deletion.md.
//
// Проверяется то, чего гейт проекта не видит: «Удалить» показывается
// только у своего, и перед удалением обязательно спрашивают. Удаление
// жёсткое (ADR-0007), поэтому кнопка не у того человека — это потеря
// чужого поста, а не опечатка.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/screens/post_screen.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha_api/api.dart';

const _mine = '00000000-0000-0000-0000-000000000001';
const _someoneElse = '00000000-0000-0000-0000-000000000002';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('у своего поста есть «Удалить»', (tester) async {
    await _pumpPost(tester, authorId: _mine, viewerId: _mine);

    expect(find.byIcon(Icons.delete_outline), findsOneWidget);
  });

  testWidgets('у чужого поста «Удалить» нет', (tester) async {
    await _pumpPost(tester, authorId: _someoneElse, viewerId: _mine);

    expect(find.byIcon(Icons.delete_outline), findsNothing);
  });

  testWidgets('перед удалением спрашивают', (tester) async {
    await _pumpPost(tester, authorId: _mine, viewerId: _mine);

    await tester.tap(find.byIcon(Icons.delete_outline));
    await tester.pumpAndSettle();

    expect(find.text('Удалить пост?'), findsOneWidget);
  });

  testWidgets('отказ ничего не удаляет', (tester) async {
    await _pumpPost(tester, authorId: _mine, viewerId: _mine);

    await tester.tap(find.byIcon(Icons.delete_outline));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Отмена'));
    await tester.pumpAndSettle();

    // Вопрос закрылся, экран поста на месте, на сервис никто не ходил:
    // сети в тесте нет, и ошибки бы не миновать.
    expect(find.text('Удалить пост?'), findsNothing);
    expect(find.byType(PostScreen), findsOneWidget);
    expect(find.byType(SnackBar), findsNothing);
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
    author: Author(id: authorId, name: 'Николай'),
    // Фотографий нет нарочно: Image.network в тесте ходить некуда
    // (сами фотографии — на показе, ADR-0009).
    media: [],
    likes: 0,
    liked: false,
    comments: 0,
  );

  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(Brightness.light),
      home: PostScreen(post: post, token: 'т', viewerId: viewerId),
    ),
  );
  await tester.pumpAndSettle();
}
