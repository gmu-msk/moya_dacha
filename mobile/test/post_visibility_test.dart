// Видимость постов: specs/013-post-visibility.md и 031-group-visibility.md.
//
// Проверяется то, чего гейт проекта не видит: слова выбора «Кто увидит»,
// в том числе у закрытого профиля и у групп
// (specs/031-group-visibility.md), и отметка на карточке.
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

  const all = Audience(PostVisibility.all);
  const friends = Audience(PostVisibility.friends);
  const me = Audience(PostVisibility.me);
  final romashka = Audience.group(GroupBrief(id: 'g1', name: 'снт Ромашка'));

  testWidgets('выбор «Кто увидит» у открытого профиля', (tester) async {
    Audience? picked;
    await pump(
      tester,
      VisibilityPicker(
        value: all,
        closed: false,
        onChanged: (value) => picked = value,
      ),
    );
    expect(find.text('Кто увидит'), findsOneWidget);
    expect(find.text('Все'), findsOneWidget);
    expect(find.text('Друзья'), findsOneWidget);
    expect(find.text('Только я'), findsOneWidget);
    // Пояснение — у каждой строки (specs/031-group-visibility.md, 15).
    expect(find.text('Все дачники в «Моей даче»'), findsOneWidget);
    expect(find.text('Пост виден только вам'), findsOneWidget);

    await tester.tap(find.text('Друзья'));
    expect(picked, friends);
  });

  testWidgets('группы — строками «Только участники» под тремя', (tester) async {
    Audience? picked;
    await pump(
      tester,
      VisibilityPicker(
        value: me,
        closed: false,
        groups: [romashka.group!],
        onChanged: (value) => picked = value,
      ),
    );
    final title = find.text('Только участники снт Ромашка');
    expect(title, findsOneWidget);
    expect(find.text('Пост увидят только участники группы'), findsOneWidget);
    expect(
      tester.getTopLeft(title).dy,
      greaterThan(tester.getTopLeft(find.text('Только я')).dy),
    );

    await tester.tap(title);
    expect(picked, romashka);
    expect(picked?.visibility, PostVisibility.me);
  });

  testWidgets('у закрытого профиля «Все» — это «Подписчики»', (tester) async {
    await pump(
      tester,
      VisibilityPicker(value: all, closed: true, onChanged: (_) {}),
    );
    expect(find.text('Все'), findsNothing);
    expect(find.text('Подписчики'), findsOneWidget);
    expect(find.text('Только ваши подписчики'), findsOneWidget);
  });

  test('сообщение после смены видимости', () {
    expect(
      visibilityChanged(friends, closed: false),
      'Теперь пост видят: друзья',
    );
    expect(
      visibilityChanged(all, closed: true),
      'Теперь пост видят: подписчики',
    );
    expect(
      visibilityChanged(me, closed: false),
      'Теперь пост видят: только вы',
    );
  });

  final now = DateTime.now();
  for (final (audience, mark) in [
    (friends, 'друзьям'),
    (me, 'только мне'),
    (romashka, 'участникам группы'),
  ]) {
    testWidgets('на карточке отметка «$mark»', (tester) async {
      await pump(tester, PostedLine(when: now, audience: audience));
      expect(find.textContaining(mark), findsOneWidget);
    });
  }

  testWidgets('у поста для всех отметки нет', (tester) async {
    await pump(tester, PostedLine(when: now, audience: all));
    expect(find.textContaining('друзьям'), findsNothing);
    expect(find.textContaining('только мне'), findsNothing);
  });
}
