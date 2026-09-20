// Токен сессии на устройстве.
//
// Токен не истекает (specs/001-auth.md), поэтому переживает перезапуск
// приложения: человек входит один раз. Лежит он в приватном хранилище
// приложения — см. docs/adr/0010-opaque-session-tokens.md о том, почему
// этого хватает на время закрытого теста и что придётся сделать до
// публичного релиза.
import 'package:shared_preferences/shared_preferences.dart';

class SessionStore {
  static const _tokenKey = 'auth_token';

  Future<String?> read() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_tokenKey);
  }

  Future<void> write(String token) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenKey, token);
  }

  Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
  }
}
