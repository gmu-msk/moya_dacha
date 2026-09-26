// Сведения о сборке и поиск новой версии (specs/017-app-updates.md).
//
// Ошибка здесь видна только на телефоне у тестировщика и только после
// следующей сборки: полоса о новой версии не появится, или появится
// у того, у кого версия и так свежая.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:moya_dacha/build_info.dart';
import 'package:moya_dacha/main.dart';
import 'package:moya_dacha/screens/about_screen.dart';
import 'package:moya_dacha/theme.dart';

const _own = BuildInfo(version: '1.0.0', build: 100);

http.Client _server(Object body, {int status = 200}) => MockClient(
  (_) async => http.Response.bytes(
    utf8.encode(body is String ? body : jsonEncode(body)),
    status,
  ),
);

void main() {
  test('разбор сведений о сборке', () {
    final info = BuildInfo.fromJson({
      'version': '1.0.0',
      'build': 390120,
      'date': '2026-09-26T12:00:00Z',
      'commit': 'aa652ef',
      'whatsNew': ['Первое', '', 7, 'Второе'],
    });
    expect(info.build, 390120);
    expect(info.date, DateTime.utc(2026, 9, 26, 12));
    expect(info.whatsNew, ['Первое', 'Второе']);
    expect(info.isDevelopment, isFalse);
  });

  test('заглушка из репозитория — сборка для разработки', () {
    final info = BuildInfo.fromJson({
      'version': '',
      'build': 0,
      'date': '',
      'commit': '',
      'whatsNew': <String>[],
    });
    expect(info.isDevelopment, isTrue);
    expect(BuildInfo.fromJson('мусор').isDevelopment, isTrue);
  });

  test('на сервере сборка новее — она и возвращается', () async {
    final newer = await fetchNewerBuild(
      _own,
      client: _server({'version': '1.0.0', 'build': 101}),
    );
    expect(newer?.build, 101);
  });

  test('на сервере та же или старее — обновления нет', () async {
    expect(
      await fetchNewerBuild(_own, client: _server({'build': 100})),
      isNull,
    );
    expect(await fetchNewerBuild(_own, client: _server({'build': 99})), isNull);
  });

  test('сервер ответил не так — обновления нет, без ошибки', () async {
    expect(
      await fetchNewerBuild(_own, client: _server('not found', status: 404)),
      isNull,
    );
    expect(await fetchNewerBuild(_own, client: _server('<html>')), isNull);
  });

  test('сборка для разработки обновлений не ищет', () async {
    expect(
      await fetchNewerBuild(
        BuildInfo.development,
        client: _server({'build': 999999}),
      ),
      isNull,
    );
  });

  testWidgets('«О приложении» показывает сборку и что нового', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: appTheme(Brightness.light),
        home: AboutScreen(info: demoBuildInfo),
      ),
    );
    await tester.pump();

    expect(find.text('386520'), findsOneWidget);
    expect(find.text('aa652ef'), findsOneWidget);
    expect(find.text('Что нового'), findsOneWidget);
    expect(find.text('Бэклог в задачах GitHub'), findsOneWidget);
  });

  testWidgets('у сборки для разработки так и написано', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: appTheme(Brightness.light),
        home: const AboutScreen(info: BuildInfo.development),
      ),
    );
    await tester.pump();

    expect(find.text('Сборка для разработки'), findsOneWidget);
    expect(find.text('Что нового'), findsNothing);
  });
}
