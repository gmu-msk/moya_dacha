// Приложение МояДача.
//
// Точка входа решает единственный вопрос: человек уже входил или нет.
// Токен не истекает (specs/001-auth.md), поэтому тот, кто входил,
// попадает сразу внутрь.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import 'api.dart';
import 'screens/home_screen.dart';
import 'screens/login_screen.dart';
import 'session.dart';

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
  bool _isNewUser = false;
  bool _restored = false;

  @override
  void initState() {
    super.initState();
    _restore();
  }

  Future<void> _restore() async {
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
    setState(() {
      _token = session.token;
      _isNewUser = session.isNewUser;
    });
  }

  Future<void> _signedOut() async {
    await _session.clear();
    debugPrint('$logMarker session=cleared');
    if (!mounted) {
      return;
    }
    setState(() {
      _token = null;
      _isNewUser = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    final token = _token;

    final Widget home;
    if (!_restored) {
      home = const Scaffold(body: Center(child: CircularProgressIndicator()));
    } else if (token == null) {
      home = LoginScreen(onSignedIn: _signedIn);
    } else {
      home = HomeScreen(
        token: token,
        isNewUser: _isNewUser,
        onSignedOut: _signedOut,
      );
    }

    return MaterialApp(
      title: 'МояДача',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF3F7D3F)),
      ),
      home: home,
    );
  }
}
