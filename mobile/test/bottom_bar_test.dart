// Нижняя панель: specs/011-bottom-bar.md.
//
// Проверяется то, чего гейт проекта не видит: какая кнопка что делает,
// что повторное касание ленты возвращает её к самому верху и что панель
// переживает крупный системный шрифт на узком телефоне.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/bottom_bar.dart';
import 'package:moya_dacha_api/api.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  final user = CurrentUser(
    id: '00000000-0000-0000-0000-000000000001',
    phone: '+79000000001',
    createdAt: DateTime(2026, 4, 1),
    nickname: 'nikolay_ogorod',
    nicknameChosen: true,
    name: 'Николай',
    about: '',
  );

  Widget bar({
    HomeTab tab = HomeTab.feed,
    void Function(HomeTab tab)? onSelect,
    VoidCallback? onNewPost,
  }) => MaterialApp(
    theme: appTheme(Brightness.light),
    home: Scaffold(
      body: const SizedBox.expand(),
      bottomNavigationBar: AppBottomBar(
        tab: tab,
        user: user,
        onSelect: onSelect ?? (_) {},
        onNewPost: onNewPost ?? () {},
      ),
    ),
  );

  testWidgets('три кнопки: лента, новый пост, профиль', (tester) async {
    final selected = <HomeTab>[];
    var newPosts = 0;
    await tester.pumpWidget(
      bar(onSelect: selected.add, onNewPost: () => newPosts++),
    );

    await tester.tap(find.text('Профиль'));
    await tester.tap(find.text('Новый пост'));
    // Лента уже открыта: касание всё равно доходит — им лента
    // возвращается к верху (требование 4).
    await tester.tap(find.text('Лента'));

    expect(selected, [HomeTab.profile, HomeTab.feed]);
    expect(newPosts, 1);
  });

  testWidgets('открытая лента — штакетник залит, закрытая — контур', (
    tester,
  ) async {
    await tester.pumpWidget(bar());
    expect(tester.widget<FenceIcon>(find.byType(FenceIcon)).selected, isTrue);
    expect(
      tester.widget<ProfileIcon>(find.byType(ProfileIcon)).selected,
      isFalse,
    );

    await tester.pumpWidget(bar(tab: HomeTab.profile));
    await tester.pumpAndSettle();
    expect(tester.widget<FenceIcon>(find.byType(FenceIcon)).selected, isFalse);
    expect(
      tester.widget<ProfileIcon>(find.byType(ProfileIcon)).selected,
      isTrue,
    );
  });

  testWidgets('панель помещается на узком телефоне при крупном шрифте', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    tester.platformDispatcher.textScaleFactorTestValue = 2;
    addTearDown(tester.view.reset);
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);

    await tester.pumpWidget(bar());

    expect(tester.takeException(), isNull);
    expect(find.text('Новый пост'), findsOneWidget);
  });

  testWidgets('долистанный список возвращается к самому верху', (tester) async {
    final scroll = ScrollController();
    addTearDown(scroll.dispose);
    await tester.pumpWidget(
      MaterialApp(
        home: ListView.builder(
          controller: scroll,
          itemCount: 500,
          itemBuilder: (_, index) =>
              SizedBox(height: 100, child: Text('$index')),
        ),
      ),
    );
    scroll.jumpTo(30000);
    await tester.pump();
    expect(find.text('0'), findsNothing);

    final done = scrollBackToTop(scroll);
    await tester.pumpAndSettle();
    await done;

    expect(scroll.offset, 0);
    expect(find.text('0'), findsOneWidget);
  });
}
