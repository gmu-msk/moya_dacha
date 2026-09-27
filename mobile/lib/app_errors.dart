// Отчёты об ошибках приложения (specs/021-app-errors.md).
//
// Приложение ловит ошибки, которые никто не обработал, и молча
// отправляет их на сервер. Ошибки, которые экран поймал и показал
// человеку (нет сети, сервер ответил ошибкой), сюда не попадают.
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:moya_dacha_api/api.dart';

import 'api.dart';
import 'build_info.dart';
import 'usage.dart';

/// Отправка отчётов одного запуска приложения.
class AppErrorReporter {
  /// Больше за запуск не отправляется: ошибка в каждом кадре не должна
  /// заваливать сервер (требование 17).
  static const perLaunch = 20;

  /// Токен вошедшего человека; нет — отчёт уходит без входа
  /// (требование 3).
  String? token;

  final Set<String> _sent = {};

  /// Подключиться к Flutter: ошибки при построении и отрисовке экрана и
  /// ошибки в асинхронном коде, которые никто не поймал (требование 15).
  /// Обычное поведение Flutter остаётся как было (требование 16).
  void install() {
    final previous = FlutterError.onError;
    FlutterError.onError = (details) {
      previous?.call(details);
      report(details.exception, details.stack);
    };
    PlatformDispatcher.instance.onError = (error, stack) {
      FlutterError.presentError(
        FlutterErrorDetails(exception: error, stack: stack),
      );
      report(error, stack);
      return true;
    };
  }

  /// Отправить отчёт об ошибке. Молча: не вышло — повтора нет
  /// (требование 16). Та же ошибка за запуск — один раз (требование 17).
  Future<void> report(Object error, StackTrace? stack) async {
    final text = _cut(error.toString(), 2000);
    if (text.isEmpty || _sent.length >= perLaunch || !_sent.add(text)) {
      return;
    }
    try {
      final build = await loadBuildInfo();
      await UsageApi(apiClient(token: token)).reportAppError(
        AppErrorReport(
          error: text,
          stack: _cut(stack?.toString() ?? '', 20000),
          version: build.version,
          build: build.build,
          screen: usage.lastScreen,
          os: _cut(
            '${Platform.operatingSystem} ${Platform.operatingSystemVersion}',
            100,
          ),
        ),
      );
      debugPrint('$logMarker app_error=sent');
    } on Object catch (sendError) {
      // Ошибка самой отправки в отчёт не уходит: иначе цикл.
      debugPrint('$logMarker app_error=send_failed error=$sendError');
    }
  }

  static String _cut(String s, int max) {
    final runes = s.runes;
    return runes.length <= max ? s : String.fromCharCodes(runes.take(max));
  }
}

/// Один на приложение.
final appErrors = AppErrorReporter();
