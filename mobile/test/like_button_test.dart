// Сердечко под постом: specs/005-likes.md.
//
// Проверки приложения — не гейт проекта (гейт один, ADR-0002), но у лайка
// та часть, из-за которой он ломается, целиком на стороне приложения:
// одно и то же число должно сходиться в ленте и на экране поста, а
// собственное касание — отзываться сразу и откатываться при ошибке.
// Глазами это ловится только на показе, а здесь — за секунды.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/feed_view.dart';
import 'package:moya_dacha/widgets/like_button.dart';
import 'package:moya_dacha_api/api.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('лайк, поставленный на экране поста, виден в ленте', (
    tester,
  ) async {
    await _pumpCard(tester, _post(likes: 0, liked: false));
    expect(_likesShown(tester), isFalse);

    // Пост тот же самый, но пришёл заново: так лента получает его после
    // лайка на экране поста (FeedViewState.replace).
    await _pumpCard(tester, _post(likes: 1, liked: true));

    expect(find.text('1'), findsOneWidget);
    expect(_heartIsFilled(tester), isTrue);
  });

  testWidgets('снятый на экране поста лайк тоже виден в ленте', (tester) async {
    await _pumpCard(tester, _post(likes: 3, liked: true));
    expect(find.text('3'), findsOneWidget);

    await _pumpCard(tester, _post(likes: 2, liked: false));

    expect(find.text('2'), findsOneWidget);
    expect(_heartIsFilled(tester), isFalse);
  });

  testWidgets('чужие лайки не переезжают на другой пост', (tester) async {
    await _pumpCard(tester, _post(likes: 3, liked: true));

    // На том же месте в списке — другой пост: после обновления ленты так
    // бывает у каждого, кто сдвинулся.
    await _pumpCard(
      tester,
      _post(id: '00000000-0000-0000-0000-000000000009', likes: 0, liked: false),
    );

    expect(_likesShown(tester), isFalse);
    expect(find.text('3'), findsNothing);
    expect(_heartIsFilled(tester), isFalse);
  });

  testWidgets('число не сбивается, пока сверху ничего не менялось', (
    tester,
  ) async {
    await _pumpCard(tester, _post(likes: 2, liked: false));
    // Перерисовка тем же постом — обычное дело: лента перестраивается
    // при каждой подгруженной странице.
    await _pumpCard(tester, _post(likes: 2, liked: false));

    expect(find.text('2'), findsOneWidget);
  });

  testWidgets('сердечко возвращается, если сервис не ответил', (tester) async {
    // Сети в виджет-тесте нет: любой запрос отвечает ошибкой, и это
    // ровно тот случай, ради которого лайк откатывается
    // (specs/005-likes.md, требование 9).
    await _pumpCard(tester, _post(likes: 0, liked: false));

    await tester.tap(find.byType(LikeButton));
    await tester.pumpAndSettle();

    expect(_likesShown(tester), isFalse);
    expect(_heartIsFilled(tester), isFalse);
    expect(find.byType(SnackBar), findsOneWidget);
  });
}

/// Видно ли число рядом с сердечком. Подписи словом у него нет, а ноль
/// не показывается вовсе (specs/000-ui.md, правило 15), поэтому «отметок
/// нет» — это отсутствие текста в кнопке.
bool _likesShown(WidgetTester tester) => tester
    .widgetList(
      find.descendant(of: find.byType(LikeButton), matching: find.byType(Text)),
    )
    .isNotEmpty;

Post _post({
  String id = '00000000-0000-0000-0000-000000000002',
  required int likes,
  required bool liked,
}) => Post(
  id: id,
  createdAt: DateTime(2026, 6, 1, 9, 30),
  caption: 'Первая клубника в этом году',
  author: Author(
    id: '00000000-0000-0000-0000-000000000001',
    name: 'Николай',
  ),
  // Фотографий нет нарочно: Image.network в тесте ходить некуда, а
  // сердечко от них не зависит (сами фотографии — на показе, ADR-0009).
  media: [],
  likes: likes,
  liked: liked,
  comments: 0,
);

/// Показать пост в ленте. Повторный вызов с тем же деревом обновляет уже
/// показанную карточку — так же, как это делает сама лента.
Future<void> _pumpCard(WidgetTester tester, Post post) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(Brightness.light),
      home: Scaffold(
        body: FeedPostCard(
          post: post,
          token: 'т',
          onTap: () {},
          onChanged: (_) {},
        ),
      ),
    ),
  );
  await tester.pump();
}

/// Закрашено ли сердечко: пустое и закрашенное — разные значки. Рядом
/// в карточке есть и другие значки, поэтому ищется именно сердечко.
bool _heartIsFilled(WidgetTester tester) =>
    find.byIcon(Icons.favorite).evaluate().isNotEmpty;
