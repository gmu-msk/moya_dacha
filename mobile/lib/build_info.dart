// Сведения о сборке: версия, номер, дата, коммит, «Что нового»
// (specs/017-app-updates.md).
//
// Их записывает в assets/build_info.json сборка на CI
// (mobile/tool/stamp-build.sh). В репозитории лежит заглушка с номером 0:
// такая сборка — для разработки, обновлений она не ищет и «Что нового»
// не показывает.
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';

class BuildInfo {
  const BuildInfo({
    required this.version,
    required this.build,
    this.date,
    this.commit = '',
    this.whatsNew = const [],
  });

  /// Сборка без сведений о себе: собрана не на CI.
  static const development = BuildInfo(version: '', build: 0);

  final String version;
  final int build;
  final DateTime? date;
  final String commit;
  final List<String> whatsNew;

  bool get isDevelopment => build <= 0;

  /// Разбор `build_info.json` и `/app.json`, у них один вид. Всё, чего
  /// нет или что не того типа, считается пустым: сведения о сборке — не
  /// повод приложению не открыться.
  factory BuildInfo.fromJson(Object? json) {
    if (json is! Map<String, dynamic>) {
      return development;
    }
    final build = json['build'];
    final date = json['date'];
    final whatsNew = json['whatsNew'];
    return BuildInfo(
      version: json['version'] is String ? json['version'] as String : '',
      build: build is int ? build : 0,
      date: date is String ? DateTime.tryParse(date) : null,
      commit: json['commit'] is String ? json['commit'] as String : '',
      whatsNew: whatsNew is List
          ? whatsNew.whereType<String>().where((s) => s.isNotEmpty).toList()
          : const [],
    );
  }
}

/// Сведения о своей сборке. Читаются один раз.
Future<BuildInfo> loadBuildInfo() async {
  try {
    final text = await rootBundle.loadString('assets/build_info.json');
    return BuildInfo.fromJson(jsonDecode(text));
  } on Exception catch (error) {
    debugPrint('$logMarker build=unknown error=$error');
    return BuildInfo.development;
  }
}

/// Адрес `/app.json` и `/app.apk` — у сервера, зашитого в сборку, а не
/// выбранного на экране «Сервер»: APK лежит на проде, а на стенде его нет.
Uri appFileUrl(String name) => Uri.parse(apiBaseUrlDefault).resolve('/$name');

/// Сборка на сервере, если она новее своей. Любая неудача — null: полоса
/// об обновлении не то, ради чего человек открыл приложение.
Future<BuildInfo?> fetchNewerBuild(BuildInfo own, {http.Client? client}) async {
  if (own.isDevelopment) {
    return null;
  }
  final httpClient = client ?? http.Client();
  try {
    final response = await httpClient
        .get(appFileUrl('app.json'))
        .timeout(const Duration(seconds: 10));
    if (response.statusCode != 200) {
      return null;
    }
    final latest = BuildInfo.fromJson(
      jsonDecode(utf8.decode(response.bodyBytes)),
    );
    debugPrint('$logMarker update=${latest.build > own.build}');
    return latest.build > own.build ? latest : null;
  } on Exception catch (error) {
    debugPrint('$logMarker update=unknown error=$error');
    return null;
  } finally {
    if (client == null) {
      httpClient.close();
    }
  }
}

/// Номер сборки, для которой уже показано «Что нового».
class WhatsNewStore {
  static const _key = 'whats_new_shown_build';

  /// Показать ли окно «Что нового» для этой сборки. Отвечает «да» один
  /// раз: сразу запоминает, что показано.
  Future<bool> shouldShow(BuildInfo own) async {
    if (own.isDevelopment || own.whatsNew.isEmpty) {
      return false;
    }
    try {
      final prefs = await SharedPreferences.getInstance();
      if (prefs.getInt(_key) == own.build) {
        return false;
      }
      await prefs.setInt(_key, own.build);
      return true;
    } on Exception catch (error) {
      debugPrint('$logMarker whats_new=storage_failed error=$error');
      return false;
    }
  }
}
