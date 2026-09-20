// Доступ к API: адрес стенда, клиент с токеном и разбор ошибок сервиса.
//
// Сам клиент генерируется из specs/openapi.yaml и руками не правится
// (ADR-0008) — здесь только то, чем приложение им пользуется.
import 'dart:convert';

import 'package:moya_dacha_api/api.dart';

/// Адрес API. По умолчанию — демо-стенд с точки зрения Android-эмулятора:
/// 10.0.2.2 это 127.0.0.1 машины-хоста. Переопределяется при сборке:
/// `flutter build apk --dart-define=API_BASE_URL=http://192.168.1.10:8080/api`.
const apiBaseUrl = String.fromEnvironment(
  'API_BASE_URL',
  defaultValue: 'http://10.0.2.2:8080/api',
);

/// Строка, по которой прогон в эмуляторе узнаёт в логах, до чего дошло
/// приложение (demo/stories/README.md).
const logMarker = 'MOYA_DACHA_DEMO';

/// Клиент API. С токеном — от имени вошедшего пользователя, без токена —
/// от имени гостя.
ApiClient apiClient({String? token}) {
  if (token == null) {
    return ApiClient(basePath: apiBaseUrl);
  }
  return ApiClient(
    basePath: apiBaseUrl,
    authentication: HttpBearerAuth()..accessToken = token,
  );
}

/// Полная ссылка на файл сервиса — аватар и дальше фотографии постов.
///
/// Сервис возвращает ссылку относительной (`/media/avatars/…`): он не
/// знает, по какому адресу до него достучались (ADR-0011). Достраивает
/// её приложение — от того же адреса, по которому ходит в API.
String mediaUrl(String link) => Uri.parse(apiBaseUrl).resolve(link).toString();

/// Машиночитаемый код ошибки сервиса (`invalid_code`, `code_expired`, …)
/// или null, если сервис вообще не ответил.
String? serviceErrorCode(Object error) => _errorField(error, 'code');

/// Сообщение, которое можно показать человеку. Тексты ошибок пишет сервис:
/// он один знает, что именно пошло не так (specs/001-auth.md).
String errorMessage(Object error) {
  final message = _errorField(error, 'message');
  if (message != null) {
    return message;
  }
  if (error is ApiException) {
    return 'Сервис ответил ошибкой ${error.code}';
  }
  return 'Сервер не отвечает. Проверьте, что стенд поднят';
}

String? _errorField(Object error, String field) {
  if (error is! ApiException) {
    return null;
  }
  final body = error.message;
  if (body == null) {
    return null;
  }
  try {
    final decoded = jsonDecode(body);
    if (decoded is Map && decoded[field] is String) {
      return decoded[field] as String;
    }
  } on FormatException {
    // Тело ответа не JSON: сказать о нём нечего.
  }
  return null;
}
