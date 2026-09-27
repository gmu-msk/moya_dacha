// Подписки: specs/012-follows.md.
//
// Проверяется то, чего гейт проекта не видит: слова под числами,
// надпись на кнопке подписки при каждом отношении и то, что числа
// закрытого профиля без подписки не открывают списков.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/screens/follow_list_screen.dart';
import 'package:moya_dacha/screens/user_screen.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/follow_button.dart';
import 'package:moya_dacha_api/api.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pump(WidgetTester tester, Widget child) => tester.pumpWidget(
    MaterialApp(
      theme: appTheme(Brightness.light),
      home: Scaffold(body: child),
    ),
  );

  test('слова под числами', () {
    expect(postsWord(1), 'пост');
    expect(postsWord(24), 'поста');
    expect(followersWord(1), 'подписчик');
    expect(followersWord(3), 'подписчика');
    expect(followersWord(18), 'подписчиков');
    expect(followingWord(21), 'подписка');
    expect(followingWord(12), 'подписок');
  });

  Relation relation(RelationFollowingEnum following) =>
      Relation(following: following, followedBy: false);

  for (final (following, compact, label) in [
    (RelationFollowingEnum.none, false, 'Подписаться'),
    (RelationFollowingEnum.yes, false, 'Вы подписаны'),
    (RelationFollowingEnum.requested, false, 'Заявка отправлена'),
    (RelationFollowingEnum.requested, true, 'Запрошено'),
  ]) {
    testWidgets('кнопка при отношении $following: «$label»', (tester) async {
      await pump(
        tester,
        FollowButton(
          token: 'т',
          userId: '00000000-0000-0000-0000-000000000002',
          relation: relation(following),
          compact: compact,
          onChanged: (_) {},
        ),
      );
      expect(find.text(label), findsOneWidget);
    });
  }

  UserProfile profile({
    required bool closed,
    RelationFollowingEnum? following,
    bool blocked = false,
  }) => UserProfile(
    id: '00000000-0000-0000-0000-000000000002',
    nickname: 'nikolay_ogorod',
    name: 'Николай',
    about: '',
    createdAt: DateTime(2026, 4, 1),
    posts: 24,
    followers: 18,
    following: 12,
    closed: closed,
    blocked: blocked,
    relation: following == null ? null : relation(following),
  );

  test('внутрь закрытого профиля — только подписчику и хозяину', () {
    final closed = profile(closed: true, following: RelationFollowingEnum.none);
    expect(UserScreenState.canSeeInside(closed, mine: false), isFalse);
    expect(UserScreenState.canSeeInside(closed, mine: true), isTrue);
    expect(
      UserScreenState.canSeeInside(
        profile(closed: true, following: RelationFollowingEnum.requested),
        mine: false,
      ),
      isFalse,
    );
    expect(
      UserScreenState.canSeeInside(
        profile(closed: true, following: RelationFollowingEnum.yes),
        mine: false,
      ),
      isTrue,
    );
    expect(
      UserScreenState.canSeeInside(
        profile(closed: false, following: RelationFollowingEnum.none),
        mine: false,
      ),
      isTrue,
    );
  });

  test('внутрь заблокированного не заглянуть, пока не разблокируешь', () {
    // specs/022-edit-block-delete.md, требование 15.
    expect(
      UserScreenState.canSeeInside(
        profile(
          closed: false,
          following: RelationFollowingEnum.none,
          blocked: true,
        ),
        mine: false,
      ),
      isFalse,
    );
  });

  testWidgets('числа открывают списки, когда их можно открыть', (tester) async {
    FollowListTab? opened;
    await pump(
      tester,
      FollowCounts(
        profile: profile(closed: false),
        onOpen: (tab) => opened = tab,
      ),
    );
    expect(find.text('24'), findsOneWidget);
    expect(find.text('подписчиков'), findsOneWidget);

    await tester.tap(find.text('подписок'));
    expect(opened, FollowListTab.following);
    await tester.tap(find.text('подписчиков'));
    expect(opened, FollowListTab.followers);
  });

  testWidgets('закрытый профиль без подписки: вместо постов замок', (
    tester,
  ) async {
    await pump(tester, const ClosedProfileView(nickname: 'nikolay_ogorod'));
    expect(find.text('Закрытый профиль'), findsOneWidget);
    expect(
      find.text('Посты nikolay_ogorod видят только его подписчики'),
      findsOneWidget,
    );
  });
}
