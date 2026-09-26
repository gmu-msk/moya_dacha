// Заходы и время в приложении (specs/020-app-sessions.md).
//
// Сессия — время, пока приложение на экране у вошедшего человека. Начало
// и конец отмечает сервер по своим часам, приложение только сообщает
// о них и считает, какие экраны открывались. Всё молча: не вышло —
// человек ничего не видит и повтора нет (требование 7).
import 'package:flutter/widgets.dart';
import 'package:moya_dacha_api/api.dart';

import 'api.dart';

/// Учёт сессий одного приложения. Работает между [UsageTracker.begin]
/// (вошедший человек открыл главный экран) и [UsageTracker.finish]
/// (вышел из аккаунта); до и после отметки экранов ничего не значат.
class UsageTracker {
  /// Вернулся раньше — та же сессия: выбор фото из галереи уводит
  /// приложение с экрана, но это не новый заход (требование 2).
  static const resumeWithin = Duration(seconds: 30);

  String? _token;
  AppLifecycleListener? _lifecycle;

  /// Номер сессии от сервера; пока он в пути — будущее с ним.
  Future<String?>? _session;
  final Map<String, int> _screens = {};
  bool _onScreen = false;
  DateTime? _hiddenAt;

  /// Раздел нижней панели, открытый сейчас: при начале сессии он тоже
  /// считается открытым (требование 10).
  String _tab = 'feed';

  bool get _active => _token != null;

  void begin(String token, {String tab = 'feed'}) {
    if (_token == token) {
      return;
    }
    _token = token;
    _tab = tab;
    _lifecycle?.dispose();
    _lifecycle = AppLifecycleListener(onShow: _shown, onHide: _hidden);
    _start();
  }

  /// Конец сессии и конец учёта — до того, как токен забыт.
  Future<void> finish() async {
    if (!_active) {
      return;
    }
    _lifecycle?.dispose();
    _lifecycle = null;
    if (_onScreen) {
      await _end();
    }
    _onScreen = false;
    _session = null;
    _token = null;
  }

  /// Показан экран поверх раздела.
  void screen(String name) {
    if (!_active) {
      return;
    }
    _screens[name] = (_screens[name] ?? 0) + 1;
  }

  /// Открыт раздел нижней панели.
  void tab(String name) {
    _tab = name;
    screen(name);
  }

  void _shown() {
    if (!_active || _onScreen) {
      return;
    }
    final hiddenAt = _hiddenAt;
    if (_session != null &&
        hiddenAt != null &&
        DateTime.now().difference(hiddenAt) < resumeWithin) {
      _onScreen = true;
      return;
    }
    _start();
  }

  void _hidden() {
    if (!_active || !_onScreen) {
      return;
    }
    _onScreen = false;
    _hiddenAt = DateTime.now();
    _end();
  }

  void _start() {
    _onScreen = true;
    _screens
      ..clear()
      ..[_tab] = 1;
    final api = UsageApi(apiClient(token: _token));
    _session = api
        .startAppSession()
        .then<String?>((started) {
          debugPrint('$logMarker usage=started');
          return started?.id;
        })
        .catchError((Object error) {
          debugPrint('$logMarker usage=start_failed error=$error');
          return null;
        });
  }

  /// Отметка конца с накопленными экранами. Повторная отметка той же
  /// сессии сдвигает конец и заменяет экраны (требования 5 и 8).
  Future<void> _end() async {
    final token = _token;
    final id = await _session;
    if (id == null || token == null) {
      return;
    }
    try {
      await UsageApi(apiClient(token: token)).endAppSession(
        id,
        appSessionEnd: AppSessionEnd(screens: Map.of(_screens)),
      );
      debugPrint('$logMarker usage=ended screens=${_screens.length}');
    } on Exception catch (error) {
      debugPrint('$logMarker usage=end_failed error=$error');
    }
  }
}

/// Один учёт на приложение.
final usage = UsageTracker();
