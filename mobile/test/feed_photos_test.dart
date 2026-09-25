// Фотографии поста в ленте: specs/004-feed.md, требование 9.
//
// Гейт проекта видит только сервис; что фотографии листаются свайпом,
// а отметка «2/5» гаснет и появляется снова, проверяется здесь.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/feed_view.dart';
import 'package:moya_dacha_api/api.dart';

Media photo(int i) => Media(
  id: 'm$i',
  kind: MediaKindEnum.photo,
  url: '/media/$i.jpg',
  width: 400,
  height: 300,
);

double countOpacity(WidgetTester tester) => tester
    .widget<AnimatedOpacity>(
      find.ancestor(
        of: find.textContaining('/5'),
        matching: find.byType(AnimatedOpacity),
      ),
    )
    .opacity;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pump(WidgetTester tester, int count) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: appTheme(Brightness.light),
        home: Scaffold(
          body: FeedPhotos(media: [for (var i = 0; i < count; i++) photo(i)]),
        ),
      ),
    );
    // Картинок в тесте нет: сеть отвечает ошибкой, место остаётся пустым.
    tester.takeException();
  }

  testWidgets('отметка видна сразу и через полторы секунды гаснет', (
    tester,
  ) async {
    await pump(tester, 5);

    expect(find.text('1/5'), findsOneWidget);
    expect(countOpacity(tester), 1);

    await tester.pump(FeedPhotos.countShown);
    await tester.pump(FeedPhotos.countFade);
    expect(countOpacity(tester), 0);
  });

  testWidgets('свайп листает фотографии и снова показывает отметку', (
    tester,
  ) async {
    await pump(tester, 5);
    await tester.pump(FeedPhotos.countShown);
    await tester.pump(FeedPhotos.countFade);

    await tester.fling(find.byType(PageView), const Offset(-300, 0), 1000);
    await tester.pumpAndSettle();
    tester.takeException();

    expect(find.text('2/5'), findsOneWidget);
    expect(countOpacity(tester), 1);

    await tester.pump(FeedPhotos.countShown);
    await tester.pump(FeedPhotos.countFade);
    expect(countOpacity(tester), 0);
  });

  testWidgets('у одной фотографии ни отметки, ни листания', (tester) async {
    await pump(tester, 1);

    expect(find.byType(PageView), findsNothing);
    expect(find.textContaining('/1'), findsNothing);
  });
}
