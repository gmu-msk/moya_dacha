// Приложение МояДача.
//
// Точка входа решает единственный вопрос: человек уже входил или нет.
// Токен не истекает (specs/001-auth.md), поэтому тот, кто входил,
// попадает сразу внутрь.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import 'api.dart';
import 'app_scope.dart';
import 'screens/gallery_screen.dart';
import 'screens/home_screen.dart';
import 'screens/login_screen.dart';
import 'server.dart';
import 'session.dart';
import 'theme.dart';
import 'widgets/loading_view.dart';

/// Каким экраном открыть приложение. Пусто — обычный путь. `gallery` —
/// витрина общих виджетов, её показывает сценарий demo/stories/000-ui;
/// задаётся при сборке: `--dart-define=START=gallery` (ADR-0012).
/// В релизной сборке без этого флага ветка с витриной выбрасывается
/// компилятором: условие константное.
const startScreen = String.fromEnvironment('START');

void main() {
  runApp(const MoyaDachaApp());
}

class MoyaDachaApp extends StatefulWidget {
  const MoyaDachaApp({super.key});

  @override
  State<MoyaDachaApp> createState() => _MoyaDachaAppState();
}

class _MoyaDachaAppState extends State<MoyaDachaApp> {
  final SessionStore _session = SessionStore();

  String? _token;
  bool _restored = false;

  @override
  void initState() {
    super.initState();
    _restore();
  }

  Future<void> _restore() async {
    // Сначала адрес сервера: всё остальное в приложении ходит по нему.
    await restoreApiBaseUrl();
    final token = await _session.read();
    debugPrint('$logMarker session=${token == null ? 'none' : 'restored'}');
    if (!mounted) {
      return;
    }
    setState(() {
      _token = token;
      _restored = true;
    });
  }

  Future<void> _signedIn(SessionCreated session) async {
    await _session.write(session.token);
    if (!mounted) {
      return;
    }
    setState(() => _token = session.token);
  }

  /// Начать заново — после смены адреса сервера на экране «Сервер».
  /// Адрес перечитывается, сессия прошлого сервера уже стёрта.
  Future<void> _restart() async {
    setState(() {
      _token = null;
      _restored = false;
    });
    await _restore();
  }

  Future<void> _signedOut() async {
    await _session.clear();
    debugPrint('$logMarker session=cleared');
    if (!mounted) {
      return;
    }
    setState(() => _token = null);
  }

  @override
  Widget build(BuildContext context) {
    final token = _token;

    final Widget home;
    if (startScreen == 'gallery') {
      home = const GalleryScreen();
    } else if (!_restored) {
      home = const Scaffold(
        body: Center(child: LoadingView(label: 'Открываю приложение…')),
      );
    } else if (token == null) {
      home = LoginScreen(onSignedIn: _signedIn);
    } else {
      home = HomeScreen(token: token, onSignedOut: _signedOut);
    }

    return AppScope(
      restart: _restart,
      child: MaterialApp(
        title: 'МояДача',
        // Светлая и тёмная тема лежат рядом, показанную выбирает система
        // (ADR-0012). Своего переключателя в приложении нет.
        theme: appTheme(Brightness.light),
        darkTheme: appTheme(Brightness.dark),
        themeMode: ThemeMode.system,
        // Край списка показывается свечением, а не растяжением
        // содержимого (specs/000-ui.md, правило 1).
        scrollBehavior: const AppScrollBehavior(),
        home: home,
      ),
    );
  }
}
