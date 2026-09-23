// Профиль пользователя: specs/009-user-profile.md.
//
// Проверяется то, чего гейт проекта не видит: тексты под именем
// и карточка поста без строки автора в прокрутке постов одного человека.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/screens/user_screen.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/feed_view.dart';
import 'package:moya_dacha_api/api.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('число постов словами', () {
    expect(postsCount(0), 'постов пока нет');
    expect(postsCount(1), '1 пост');
    expect(postsCount(3), '3 поста');
    expect(postsCount(14), '14 постов');
    expect(postsCount(21), '21 пост');
  });

  test('с какого времени в МоейДаче: год — только прошлый', () {
    final now = DateTime(2026, 9, 22);
    expect(hereSince(DateTime(2026, 5, 14), now: now), 'в МоейДаче с мая');
    expect(
      hereSince(DateTime(2025, 12, 1), now: now),
      'в МоейДаче с декабря 2025',
    );
  });

  final post = Post(
    id: '00000000-0000-0000-0000-00000000000a',
    createdAt: DateTime(2026, 6, 1, 9, 30),
    caption: 'Кот Василий охраняет рассаду',
    author: Author(
      id: '00000000-0000-0000-0000-000000000002',
      nickname: 'valya_teplitsa',
      name: 'Валентина',
    ),
    // Фотографий нет нарочно: Image.network в тесте ходить некуда.
    media: [],
    likes: 0,
    liked: false,
    comments: 0,
  );

  testWidgets('в постах одного человека строки автора нет', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: appTheme(Brightness.light),
        home: Scaffold(
          body: FeedPostCard(
            post: post,
            token: 'т',
            showAuthor: false,
            onTap: () {},
            onChanged: (_) {},
          ),
        ),
      ),
    );

    expect(find.text('valya_teplitsa'), findsNothing);
    expect(find.text('Кот Василий охраняет рассаду'), findsOneWidget);
  });

  testWidgets('в ленте никнейм автора открывает его профиль', (tester) async {
    Author? opened;
    await tester.pumpWidget(
      MaterialApp(
        theme: appTheme(Brightness.light),
        home: Scaffold(
          body: FeedPostCard(
            post: post,
            token: 'т',
            onTap: () {},
            onChanged: (_) {},
            onOpenAuthor: (author) => opened = author,
          ),
        ),
      ),
    );

    await tester.tap(find.text('valya_teplitsa'));
    expect(opened?.id, post.author.id);
  });
}
