// Тексты строк раздела «Уведомления»: specs/014-notifications.md,
// требование 1.
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/screens/notifications_screen.dart';
import 'package:moya_dacha_api/api.dart';

Notification _item(NotificationKindEnum kind, {int? others, String? comment}) =>
    Notification(
      id: 'n',
      kind: kind,
      createdAt: DateTime(2026, 9, 24),
      actor: FollowUser(id: 'u', nickname: 'Mikhalych', name: 'Михалыч'),
      others: others,
      comment: comment,
      unread: true,
    );

void main() {
  test('подписка и принятая заявка', () {
    expect(
      notificationText(_item(NotificationKindEnum.follow)),
      'подписался на вас',
    );
    expect(
      notificationText(_item(NotificationKindEnum.followAccepted)),
      'принял вашу заявку',
    );
  });

  test('лайк один и слитые лайки', () {
    expect(
      notificationText(_item(NotificationKindEnum.like, others: 0)),
      'отметил ваш пост',
    );
    expect(
      notificationText(_item(NotificationKindEnum.like, others: 3)),
      'и ещё 3 отметили ваш пост',
    );
  });

  test('комментарий с началом текста в кавычках', () {
    expect(
      notificationText(
        _item(NotificationKindEnum.comment, comment: 'Сорт какой?'),
      ),
      'ответил на ваш пост: «Сорт какой?»',
    );
  });
}
