// Видимость постов: specs/013-post-visibility.md.
//
// Проверяется то, чего гейт проекта не видит: слова выбора «Кто увидит»,
// в том числе у закрытого профиля, и отметка на карточке.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/author_line.dart';
import 'package:moya_dacha/widgets/visibility_picker.dart';
import 'package:moya_dacha_api/api.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pump(WidgetTester tester, Widget child) => tester.pumpWidget(
    MaterialApp(
      theme: appTheme(Brightness.light),
      home: Scaffold(body: SingleChildScrollView(child: child)),
    ),
  );

  testWidgets('выбор «Кто увидит» у открытого профиля', (tester) async {
    PostVisibility? picked;
    await pump(
      tester,
      VisibilityPicker(
        value: PostVisibility.all,
        closed: false,
        onChanged: (value) => picked = value,
      ),
    );
    expect(find.text('Кто увидит'), findsOneWidget);
    expect(find.text('Всем'), findsOneWidget);
    expect(find.text('Все дачники в МоейДаче'), findsOneWidget);
    expect(find.text('Те, с кем вы подписаны друг на друга'), findsOneWidget);
    expect(find.text('Пост виден только вам'), findsOneWidget);

    await tester.tap(find.text('Друзьям'));
    expect(picked, PostVisibility.friends);
  });

  testWidgets('у закрытого профиля «Всем» — это «Подписчикам»', (tester) async {
    await pump(
      tester,
      VisibilityPicker(
        value: PostVisibility.all,
        closed: true,
        onChanged: (_) {},
      ),
    );
    expect(find.text('Всем'), findsNothing);
    expect(find.text('Подписчикам'), findsOneWidget);
    expect(find.text('Только ваши подписчики'), findsOneWidget);
  });

  test('сообщение после смены видимости', () {
    expect(
      visibilityChanged(PostVisibility.friends, closed: false),
      'Теперь пост видят: друзья',
    );
    expect(
      visibilityChanged(PostVisibility.all, closed: true),
      'Теперь пост видят: подписчики',
    );
    expect(
      visibilityChanged(PostVisibility.me, closed: false),
      'Теперь пост видят: только вы',
    );
  });

  final now = DateTime.now();
  for (final (visibility, mark) in [
    (PostVisibility.friends, 'друзьям'),
    (PostVisibility.me, 'только мне'),
  ]) {
    testWidgets('на карточке отметка «$mark»', (tester) async {
      await pump(tester, PostedLine(when: now, visibility: visibility));
      expect(find.textContaining(mark), findsOneWidget);
    });
  }

  testWidgets('у поста для всех отметки нет', (tester) async {
    await pump(tester, PostedLine(when: now, visibility: PostVisibility.all));
    expect(find.textContaining('друзьям'), findsNothing);
    expect(find.textContaining('только мне'), findsNothing);
  });
}
