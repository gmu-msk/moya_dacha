// Приложение МояДача.
//
// Точка входа решает единственный вопрос: человек уже входил или нет.
// Токен не истекает (specs/001-auth.md), поэтому тот, кто входил,
// попадает сразу внутрь.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';
import 'package:url_launcher/url_launcher.dart';

import 'api.dart';
import 'app_errors.dart';
import 'app_scope.dart';
import 'build_info.dart';
import 'screens/about_screen.dart';
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
/// `about` — «О приложении» с образцом сведений о сборке, его показывает
/// сценарий demo/stories/017-app-updates.
/// В релизной сборке без этого флага ветка с витриной выбрасывается
/// компилятором: условие константное.
const startScreen = String.fromEnvironment('START');

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  // Необработанные ошибки уходят на сервер (specs/021-app-errors.md).
  appErrors.install();
  runApp(const MoyaDachaApp());
}

class MoyaDachaApp extends StatefulWidget {
  const MoyaDachaApp({super.key});

  @override
  State<MoyaDachaApp> createState() => _MoyaDachaAppState();
}

class _MoyaDachaAppState extends State<MoyaDachaApp> {
  final SessionStore _session = SessionStore();
  final GlobalKey<NavigatorState> _navigator = GlobalKey<NavigatorState>();
  final GlobalKey<ScaffoldMessengerState> _messenger =
      GlobalKey<ScaffoldMessengerState>();

  /// Обновление ищется один раз за запуск, после входа
  /// (specs/017-app-updates.md).
  bool _updatesChecked = false;

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
    closePushedScreens(_navigator.currentState);
    await _restore();
  }

  Future<void> _signedOut() async {
    await _session.clear();
    debugPrint('$logMarker session=cleared');
    if (!mounted) {
      return;
    }
    setState(() => _token = null);
    // Выходят из аккаунта в профиле, а профиль открыт поверх ленты.
    closePushedScreens(_navigator.currentState);
  }

  void _checkUpdatesOnce() {
    if (_updatesChecked) {
      return;
    }
    _updatesChecked = true;
    WidgetsBinding.instance.addPostFrameCallback((_) => _afterSignedIn());
  }

  /// После входа: один раз «Что нового», затем полоса о новой версии.
  Future<void> _afterSignedIn() async {
    final own = await loadBuildInfo();
    debugPrint('$logMarker build=${own.build}');

    if (await WhatsNewStore().shouldShow(own)) {
      final context = _navigator.currentState?.overlay?.context;
      if (context != null && context.mounted) {
        await showDialog<void>(
          context: context,
          builder: (context) => AlertDialog(
            title: const Text('Что нового'),
            content: SingleChildScrollView(
              child: WhatsNewList(items: own.whatsNew),
            ),
            actions: [
              FilledButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('Понятно'),
              ),
            ],
          ),
        );
      }
    }

    final newer = await fetchNewerBuild(own);
    final messenger = _messenger.currentState;
    if (newer == null || messenger == null || !mounted) {
      return;
    }
    messenger.showMaterialBanner(
      MaterialBanner(
        content: const Text('Вышла новая версия'),
        actions: [
          TextButton(
            onPressed: messenger.hideCurrentMaterialBanner,
            child: const Text('Не сейчас'),
          ),
          FilledButton(
            onPressed: () {
              messenger.hideCurrentMaterialBanner();
              // APK скачивает браузер телефона: ставить его всё равно
              // системе, а не приложению.
              launchUrl(
                appFileUrl('app.apk'),
                mode: LaunchMode.externalApplication,
              );
            },
            child: const Text('Скачать'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final token = _token;
    // Отчёт об ошибке уходит от имени того, кто вошёл сейчас.
    appErrors.token = token;

    final Widget home;
    if (startScreen == 'gallery') {
      home = const GalleryScreen();
    } else if (startScreen == 'about') {
      home = AboutScreen(info: demoBuildInfo);
    } else if (!_restored) {
      home = const Scaffold(
        body: Center(child: LoadingView(label: 'Открываю приложение…')),
      );
    } else if (token == null) {
      home = LoginScreen(onSignedIn: _signedIn);
    } else {
      home = HomeScreen(token: token, onSignedOut: _signedOut);
      _checkUpdatesOnce();
    }

    return AppScope(
      restart: _restart,
      signOut: _signedOut,
      child: MaterialApp(
        title: 'МояДача',
        navigatorKey: _navigator,
        scaffoldMessengerKey: _messenger,
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

/// Образец сведений о сборке для сценария показа: у сборки для
/// эмулятора своих сведений нет.
final demoBuildInfo = BuildInfo(
  version: '1.0.0',
  build: 386520,
  date: DateTime.utc(2026, 9, 26, 12),
  commit: 'aa652ef',
  whatsNew: const [
    'Сборки обновляются поверх и показывают, что нового',
    'Бэклог в задачах GitHub',
  ],
);

/// Закрыть экраны, открытые поверх главного.
///
/// Смена `home` меняет только нижний экран стопки: открытые поверх него
/// остаются на месте и видны. Выйдя из аккаунта в профиле, человек так и
/// смотрел бы на свой профиль, хотя приложение уже забыло токен. Экраны
/// прошлого входа закрываются принудительно: профиль перехватывает
/// «назад» (`PopScope`), и `maybePop` его бы не закрыл.
void closePushedScreens(NavigatorState? navigator) =>
    navigator?.popUntil((route) => route.isFirst);
