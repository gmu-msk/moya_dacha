// Нижняя панель: specs/011-bottom-bar.md.
//
// Проверяется то, чего гейт проекта не видит: какая кнопка что делает,
// как выделен открытый раздел и что повторное касание ленты возвращает
// её к самому верху.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/bottom_bar.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

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
        onSelect: onSelect ?? (_) {},
        onNewPost: onNewPost ?? () {},
      ),
    ),
  );

  /// Залит ли значок этого рода.
  bool filled<T extends CustomPainter>(WidgetTester tester) {
    final paint = tester.widget<CustomPaint>(
      find.byWidgetPredicate((w) => w is CustomPaint && w.painter is T),
    );
    return switch (paint.painter) {
      final FenceIconPainter p => p.filled,
      final PersonIconPainter p => p.filled,
      _ => false,
    };
  }

  testWidgets('три кнопки без подписей: лента, новый пост, профиль', (
    tester,
  ) async {
    final selected = <HomeTab>[];
    var newPosts = 0;
    await tester.pumpWidget(
      bar(onSelect: selected.add, onNewPost: () => newPosts++),
    );

    // Подписей на экране нет (требование 2), названия — у TalkBack.
    expect(find.text('Лента'), findsNothing);
    expect(find.bySemanticsLabel('Новый пост'), findsOneWidget);

    await tester.tap(find.byTooltip('Профиль'));
    await tester.tap(find.byTooltip('Новый пост'));
    // Лента уже открыта: касание всё равно доходит — им лента
    // возвращается к верху (требование 4).
    await tester.tap(find.byTooltip('Лента'));

    expect(selected, [HomeTab.profile, HomeTab.feed]);
    expect(newPosts, 1);
  });

  testWidgets('панель ниже материаловой: 60 вместо 80', (tester) async {
    await tester.pumpWidget(bar());
    expect(tester.getSize(find.byType(AppBottomBar)).height, bottomBarHeight);
  });

  testWidgets('открытый раздел залит, закрытый — контуром', (tester) async {
    await tester.pumpWidget(bar());
    expect(filled<FenceIconPainter>(tester), isTrue);
    expect(filled<PersonIconPainter>(tester), isFalse);
    expect(
      tester.getSemantics(find.bySemanticsLabel('Лента')),
      isSemantics(
        label: 'Лента',
        isButton: true,
        isSelected: true,
        hasTapAction: true,
      ),
    );

    await tester.pumpWidget(bar(tab: HomeTab.profile));
    await tester.pumpAndSettle();
    expect(filled<FenceIconPainter>(tester), isFalse);
    expect(filled<PersonIconPainter>(tester), isTrue);
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
