// Приложение запускается и рисуется.
//
// Это не гейт и не проверка поведения: гейт в проекте один — интеграционные
// тесты сервера (ADR-0002), а фичи проверяются глазами по сценариям показа
// (ADR-0009). Здесь проверяется ровно одно: экран собирается без исключения,
// в обеих темах и при увеличенном системном шрифте. Такая ошибка иначе
// находится только прогоном в эмуляторе, а он идёт минуты.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/main.dart';
import 'package:moya_dacha/screens/gallery_screen.dart';
import 'package:moya_dacha/screens/intro_screen.dart';
import 'package:moya_dacha/screens/login_screen.dart';
import 'package:moya_dacha/screens/new_post_screen.dart';
import 'package:moya_dacha/screens/post_screen.dart';
import 'package:moya_dacha/screens/profile_screen.dart';
import 'package:moya_dacha/screens/server_screen.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/feed_view.dart';
import 'package:moya_dacha_api/api.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  // Хранилище на устройстве в тестах недоступно; подменяем его пустым,
  // чтобы адрес сервера и сессия читались, как на чистом телефоне.
  TestWidgetsFlutterBinding.ensureInitialized();
  SharedPreferences.setMockInitialValues(const <String, Object>{});

  testWidgets('приложение открывается', (tester) async {
    await tester.pumpWidget(const MoyaDachaApp());
    await tester.pump();
  });

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

  // Пост без единой фотографии невозможен (docs/adr/0006-post-is-media.md),
  // но сети в виджет-тесте нет, а Image.network в ней падает. Сами
  // фотографии проверяются глазами по сценарию показа (ADR-0009); здесь
  // проверяется, что экран поста собирается.
  final post = Post(
    id: '00000000-0000-0000-0000-000000000002',
    createdAt: DateTime(2026, 6, 1, 9, 30),
    caption: 'Первая клубника в этом году',
    author: Author(id: user.id, nickname: user.nickname, name: user.name),
    media: [],
    likes: 2,
    liked: true,
    comments: 1,
    visibility: PostVisibility.all,
  );

  for (final brightness in Brightness.values) {
    final theme = brightness == Brightness.light ? 'светлой' : 'тёмной';

    testWidgets('экран входа рисуется в $theme теме', (tester) async {
      await _pump(tester, brightness, LoginScreen(onSignedIn: (_) async {}));
    });

    testWidgets('витрина рисуется в $theme теме', (tester) async {
      await _pump(tester, brightness, const GalleryScreen());
    });

    testWidgets('знакомство рисуется в $theme теме', (tester) async {
      await _pump(
        tester,
        brightness,
        IntroScreen(token: 'т', user: user, onDone: (_) {}),
      );
    });

    testWidgets('профиль рисуется в $theme теме', (tester) async {
      await _pump(
        tester,
        brightness,
        ProfileScreen(token: 'т', user: user, onSignedOut: () async {}),
      );
    });

    testWidgets('новый пост рисуется в $theme теме', (tester) async {
      await _pump(tester, brightness, const NewPostScreen(token: 'т'));
    });

    testWidgets('пост рисуется в $theme теме', (tester) async {
      await _pump(
        tester,
        brightness,
        PostScreen(post: post, token: 'т', viewerId: user.id),
      );
    });

    testWidgets('пост в ленте рисуется в $theme теме', (tester) async {
      await _pump(
        tester,
        brightness,
        Scaffold(
          body: FeedPostCard(
            post: post,
            token: 'т',
            onTap: () {},
            onChanged: (_) {},
          ),
        ),
      );
    });

    testWidgets('пост в ленте выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(
        tester,
        brightness,
        Scaffold(
          body: FeedPostCard(
            post: post,
            token: 'т',
            onTap: () {},
            onChanged: (_) {},
          ),
        ),
        textScale: 2,
      );
    });

    testWidgets('новый пост выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(
        tester,
        brightness,
        const NewPostScreen(token: 'т'),
        textScale: 2,
      );
    });

    testWidgets('экран входа выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(
        tester,
        brightness,
        LoginScreen(onSignedIn: (_) async {}),
        textScale: 2,
      );
    });

    testWidgets('витрина выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(tester, brightness, const GalleryScreen(), textScale: 2);
    });

    testWidgets('песочница витрины раскрывается в $theme теме', (tester) async {
      await _pump(tester, brightness, const GalleryScreen());
      await tester.tap(find.text('Песочница'));
      await tester.pumpAndSettle();
      expect(find.text('Значения для темы'), findsOneWidget);
    });

    testWidgets('песочница витрины выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(tester, brightness, const GalleryScreen(), textScale: 2);
      await tester.tap(find.text('Песочница'));
      await tester.pumpAndSettle();
    });

    testWidgets('экран сервера рисуется в $theme теме', (tester) async {
      await _pump(tester, brightness, const ServerScreen());
    });

    testWidgets('экран сервера выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(tester, brightness, const ServerScreen(), textScale: 2);
    });

    testWidgets('профиль выживает при крупном шрифте в $theme теме', (
      tester,
    ) async {
      await _pump(
        tester,
        brightness,
        ProfileScreen(token: 'т', user: user, onSignedOut: () async {}),
        textScale: 2,
      );
    });
  }
}

/// Показать экран так, как его увидит человек: с нашей темой и, если надо,
/// с увеличенным системным шрифтом.
Future<void> _pump(
  WidgetTester tester,
  Brightness brightness,
  Widget screen, {
  double textScale = 1,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: appTheme(brightness),
      home: MediaQuery(
        data: MediaQueryData(textScaler: TextScaler.linear(textScale)),
        child: screen,
      ),
    ),
  );
  await tester.pump();
}
