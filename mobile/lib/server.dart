// Адрес сервера, к которому ходит приложение.
//
// Раньше адрес зашивался в APK при сборке, поэтому под каждый стенд
// собирался свой APK. Теперь сборка одна, а адрес задаётся в самом
// приложении: тестовую сборку из PR можно направить на временный стенд,
// на машину в локальной сети или на прод, ничего не пересобирая
// (ADR-0013). Зашитый при сборке адрес остаётся значением по умолчанию —
// им живут эмулятор и прогон на CI.
import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';

/// Адрес сервера, выбранный человеком, — на устройстве.
class ServerStore {
  static const _urlKey = 'api_base_url';

  Future<String?> read() async {
    final prefs = await _prefs();
    return prefs?.getString(_urlKey);
  }

  Future<void> write(String url) async {
    final prefs = await _prefs();
    await prefs?.setString(_urlKey, url);
  }

  /// Забыть выбранный адрес: приложение вернётся к зашитому при сборке.
  Future<void> clear() async {
    final prefs = await _prefs();
    await prefs?.remove(_urlKey);
  }

  /// Хранилище недоступно только там, где нет платформы (например, в
  /// `flutter test`). Это не повод не открыться: адрес тогда зашитый.
  Future<SharedPreferences?> _prefs() async {
    try {
      return await SharedPreferences.getInstance();
    } on Exception catch (error) {
      debugPrint('$logMarker server=storage_failed error=$error');
      return null;
    }
  }
}

/// Поднять сохранённый адрес в [apiBaseUrl]. Вызывается один раз при старте
/// приложения, до первого обращения к сервису.
Future<void> restoreApiBaseUrl() async {
  apiBaseUrl = await ServerStore().read() ?? apiBaseUrlDefault;
  debugPrint('$logMarker server=$apiBaseUrl');
}

/// Привести введённый человеком адрес к тому, что ждёт клиент API:
///
///     example.com                 -> https://example.com/api
///     https://example.com/        -> https://example.com/api
///     http://192.168.1.10:8080    -> http://192.168.1.10:8080/api
///
/// Адрес стенда приходит человеку ссылкой вида `https://…trycloudflare.com`,
/// и дописывать к ней `/api` руками с телефона — лишняя возможность
/// ошибиться. Возвращает null, если адреса из введённого не выходит.
String? normalizeServerUrl(String input) {
  var text = input.trim();
  if (text.isEmpty) {
    return null;
  }
  if (!text.contains('://')) {
    text = 'https://$text';
  }

  final uri = Uri.tryParse(text);
  if (uri == null || uri.host.isEmpty) {
    return null;
  }
  if (uri.scheme != 'http' && uri.scheme != 'https') {
    return null;
  }

  var path = uri.path.replaceAll(RegExp(r'/+$'), '');
  if (path.isEmpty) {
    path = '/api';
  }

  return Uri(
    scheme: uri.scheme,
    host: uri.host,
    port: uri.hasPort ? uri.port : null,
    path: path,
  ).toString();
}
