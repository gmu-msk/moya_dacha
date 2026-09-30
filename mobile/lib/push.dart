// Пуши о новых уведомлениях (specs/024-push.md, ADR-0027).
//
// Firebase берёт настройки из ресурсов Android, которые кладёт в сборку CI
// (mobile/tool/firebase-config.sh). В сборке без них, на телефоне без
// Google Play или при любой ошибке Firebase пушей просто нет: приложение
// работает как раньше и человеку ничего не говорит (требование 3).
import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:moya_dacha_api/api.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';

/// Куда ведёт касание пуша (требование 4): вид события, пост и тот, кто
/// это сделал, — поля `data` сообщения FCM.
class PushTarget {
  const PushTarget({required this.kind, this.postId, this.userId});

  factory PushTarget.fromMessage(RemoteMessage message) => PushTarget(
    kind: message.data['kind'] as String? ?? '',
    postId: message.data['post_id'] as String?,
    userId: message.data['user_id'] as String?,
  );

  final String kind;
  final String? postId;
  final String? userId;
}

/// Пуши одного вошедшего человека: между [start] и [stop].
class PushClient {
  /// Разрешение спрашивается один раз (требование 1): Firebase на
  /// Android не отличает «ещё не спрашивали» от «отказали», поэтому
  /// приложение помнит само.
  static const _askedKey = 'push_permission_asked';

  final List<StreamSubscription<Object?>> _subscriptions = [];

  /// Спросить разрешение, отдать токен сервису и слушать пуши.
  /// [onOpen] — человек коснулся пуша, [onForeground] — пуш пришёл,
  /// пока приложение открыто (требование 5).
  Future<void> start({
    required String token,
    required void Function(PushTarget target) onOpen,
    required VoidCallback onForeground,
  }) async {
    await stop();
    try {
      if (Firebase.apps.isEmpty) {
        await Firebase.initializeApp();
      }
    } on Object catch (error) {
      // Настроек Firebase в сборке нет — пушей нет (требование 3).
      debugPrint('$logMarker push=off reason=$error');
      return;
    }
    final messaging = FirebaseMessaging.instance;
    try {
      if (!await _allowed(messaging)) {
        debugPrint('$logMarker push=denied');
        return;
      }

      _subscriptions
        ..add(
          FirebaseMessaging.onMessageOpenedApp.listen(
            (message) => onOpen(PushTarget.fromMessage(message)),
          ),
        )
        ..add(FirebaseMessaging.onMessage.listen((_) => onForeground()))
        ..add(messaging.onTokenRefresh.listen((fcm) => _send(token, fcm)));

      // Приложение запущено касанием пуша, пока оно было закрыто.
      final initial = await messaging.getInitialMessage();
      if (initial != null) {
        onOpen(PushTarget.fromMessage(initial));
      }

      final fcm = await messaging.getToken();
      if (fcm != null) {
        await _send(token, fcm);
      }
    } on Object catch (error) {
      debugPrint('$logMarker push=failed error=$error');
    }
  }

  /// Перестать слушать: человек вышел. Токен сервис забудет сам вместе
  /// с сессией (требование 7).
  Future<void> stop() async {
    for (final subscription in _subscriptions) {
      await subscription.cancel();
    }
    _subscriptions.clear();
  }

  Future<bool> _allowed(FirebaseMessaging messaging) async {
    final prefs = await SharedPreferences.getInstance();
    final NotificationSettings settings;
    if (prefs.getBool(_askedKey) ?? false) {
      settings = await messaging.getNotificationSettings();
    } else {
      await prefs.setBool(_askedKey, true);
      settings = await messaging.requestPermission();
    }
    return settings.authorizationStatus == AuthorizationStatus.authorized ||
        settings.authorizationStatus == AuthorizationStatus.provisional;
  }

  /// Токен телефона — сервису (требование 2). Не вышло — молча:
  /// следующий запуск отдаст его снова.
  Future<void> _send(String session, String fcm) async {
    try {
      await NotificationsApi(apiClient(token: session))
          .setPushToken(PushToken(token: fcm));
      debugPrint('$logMarker push=registered');
    } on Exception catch (error) {
      debugPrint('$logMarker push=register_failed error=$error');
    }
  }
}
