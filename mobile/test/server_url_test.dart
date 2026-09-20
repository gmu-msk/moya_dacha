// Разбор адреса сервера, введённого руками.
//
// Гейт проекта — интеграционные тесты сервера (ADR-0002), и поведение
// клиента тестами не покрывается. Исключение здесь одно и сознательное:
// адрес стенда человек вставляет с телефона в поле ввода, а ошибка в
// разборе видна только немым экраном входа (ADR-0013).
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/server.dart';

void main() {
  test('к адресу без пути дописывается /api', () {
    expect(
      normalizeServerUrl('https://example.trycloudflare.com'),
      'https://example.trycloudflare.com/api',
    );
    expect(
      normalizeServerUrl('  https://example.trycloudflare.com/  '),
      'https://example.trycloudflare.com/api',
    );
  });

  test('адрес без схемы считается https', () {
    expect(
      normalizeServerUrl('example.trycloudflare.com'),
      'https://example.trycloudflare.com/api',
    );
  });

  test('адрес с портом и путём остаётся как есть', () {
    expect(
      normalizeServerUrl('http://192.168.1.10:8080/api'),
      'http://192.168.1.10:8080/api',
    );
  });

  test('из мусора адреса не выходит', () {
    expect(normalizeServerUrl(''), isNull);
    expect(normalizeServerUrl('   '), isNull);
    expect(normalizeServerUrl('ftp://example.com'), isNull);
  });
}
