// Когда это было, словами: specs/000-ui.md, правило 12.
//
// Проверки приложения — не гейт проекта (гейт один, ADR-0002), но здесь
// чистая функция со склонениями и границами суток: «11 минут», а не
// «11 минута», и пост в 23:30 — это «вчера», а не «час назад». Такое
// ловится только чтением кода или вот такими проверками за секунду.
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/widgets/author_line.dart';

void main() {
  // Точка отсчёта у всех проверок одна: иначе они зависели бы от того,
  // в какой день и час их запустили.
  final now = DateTime(2026, 9, 21, 12, 0);
  String when(DateTime moment) => whenPosted(moment, from: now);

  test('только что', () {
    expect(when(now.subtract(const Duration(seconds: 30))), 'только что');
    // Часы телефона отстали от сервера: пост «из будущего» — это не повод
    // писать «через минуту».
    expect(when(now.add(const Duration(minutes: 5))), 'только что');
  });

  test('минуты со склонением', () {
    expect(when(now.subtract(const Duration(minutes: 1))), '1 минуту назад');
    expect(when(now.subtract(const Duration(minutes: 5))), '5 минут назад');
    expect(when(now.subtract(const Duration(minutes: 11))), '11 минут назад');
    expect(when(now.subtract(const Duration(minutes: 21))), '21 минуту назад');
    expect(when(now.subtract(const Duration(minutes: 22))), '22 минуты назад');
  });

  test('часы, пока не наступила полночь', () {
    expect(when(now.subtract(const Duration(hours: 1))), '1 час назад');
    expect(when(now.subtract(const Duration(hours: 3))), '3 часа назад');
    expect(when(DateTime(2026, 9, 21, 0, 30)), '11 часов назад');
  });

  test('вчера — это календарный день, а не 24 часа', () {
    expect(when(DateTime(2026, 9, 20, 23, 30)), 'вчера');
    expect(when(DateTime(2026, 9, 20, 0, 5)), 'вчера');
    // Полночь наступила час назад — значит уже вчера, а не «час назад».
    expect(whenPosted(
      DateTime(2026, 9, 20, 23, 30),
      from: DateTime(2026, 9, 21, 0, 30),
    ), 'вчера');
  });

  test('дни и недели', () {
    expect(when(DateTime(2026, 9, 18, 12, 0)), '3 дня назад');
    expect(when(DateTime(2026, 9, 16, 12, 0)), '5 дней назад');
    expect(when(DateTime(2026, 9, 13, 12, 0)), 'на прошлой неделе');
    expect(when(DateTime(2026, 9, 5, 12, 0)), '2 недели назад');
  });

  test('месяцы', () {
    expect(when(DateTime(2026, 8, 21, 12, 0)), 'в прошлом месяце');
    expect(when(DateTime(2026, 8, 25, 12, 0)), '3 недели назад');
    expect(when(DateTime(2026, 4, 21, 12, 0)), '5 месяцев назад');
    expect(when(DateTime(2025, 11, 21, 12, 0)), '10 месяцев назад');
  });

  test('годы', () {
    expect(when(DateTime(2025, 9, 21, 12, 0)), 'в прошлом году');
    expect(when(DateTime(2021, 9, 21, 12, 0)), '5 лет назад');
    expect(when(DateTime(2024, 9, 21, 12, 0)), '2 года назад');
  });
}
