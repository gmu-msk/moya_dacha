// Отчёты об ошибках приложения (specs/021-app-errors.md, требования 16–18).
//
// Проверка без эмулятора: отчёты уходят на маленький HTTP-сервер здесь же.
// Гейт от этого не зависит (ADR-0012).
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/api.dart';
import 'package:moya_dacha/app_errors.dart';
import 'package:moya_dacha/usage.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  // Тесты Flutter подменяют HTTP заглушкой; здесь нужен настоящий.
  HttpOverrides.global = null;

  late HttpServer server;
  late List<Map<String, dynamic>> bodies;
  late List<String?> auth;

  setUp(() async {
    bodies = [];
    auth = [];
    server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((request) async {
      if (request.uri.path == '/api/app-errors') {
        bodies.add(
          jsonDecode(await utf8.decodeStream(request)) as Map<String, dynamic>,
        );
        auth.add(request.headers.value('authorization'));
      }
      request.response.statusCode = 204;
      await request.response.close();
    });
    apiBaseUrl = 'http://127.0.0.1:${server.port}/api';
  });

  tearDown(() => server.close(force: true));

  test('отчёт уходит с текстом, стеком, экраном и токеном', () async {
    final reporter = AppErrorReporter()..token = 'secret';
    usage.screen('post');
    await reporter.report(StateError('No element'), StackTrace.current);

    expect(bodies, hasLength(1));
    expect(bodies.single['error'], 'Bad state: No element');
    expect(bodies.single['stack'], contains('app_errors_test.dart'));
    expect(bodies.single['screen'], 'post');
    expect(bodies.single['build'], isA<int>());
    expect(auth.single, 'Bearer secret');
  });

  test('без входа — без токена', () async {
    await AppErrorReporter().report(Exception('до входа'), null);
    expect(bodies, hasLength(1));
    expect(auth.single, isNull);
  });

  test('та же ошибка за запуск — один раз, всего — не больше 20', () async {
    final reporter = AppErrorReporter();
    for (var i = 0; i < 3; i++) {
      await reporter.report(StateError('одна и та же'), null);
    }
    expect(bodies, hasLength(1));
    for (var i = 0; i < 30; i++) {
      await reporter.report(StateError('ошибка $i'), null);
    }
    expect(bodies, hasLength(AppErrorReporter.perLaunch));
  });

  test('сервер недоступен — молча, без исключения', () async {
    await server.close(force: true);
    await AppErrorReporter().report(StateError('нет сети'), null);
  });
}
