// Населённый пункт: specs/025-places.md, specs/026-places-nearby.md.
//
// Проверяется то, чего гейт проекта не видит: строка пункта в профиле,
// подсказки под полем и то, что набранный текст пунктом не становится.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:moya_dacha/theme.dart';
import 'package:moya_dacha/widgets/place_field.dart';
import 'package:moya_dacha_api/api.dart';

void main() {
  final snt = Place(
    id: 'snt',
    name: 'снт Андрейково',
    area: 'Дмитровский р-н, Московская обл',
  );
  final village = Place(
    id: 'village',
    name: 'д Андрейково',
    area: 'Вологодский р-н, Вологодская обл',
  );
  final moscow = Place(id: 'msk', name: 'г Москва', area: '');

  Widget app(Widget child) => MaterialApp(
    theme: appTheme(Brightness.light),
    home: Scaffold(body: ListView(children: [child])),
  );

  testWidgets('в профиле — название и под ним район и область', (tester) async {
    await tester.pumpWidget(app(PlaceLine(place: snt)));
    expect(find.text('снт Андрейково'), findsOneWidget);
    expect(find.text('Дмитровский р-н, Московская обл'), findsOneWidget);
  });

  testWidgets('пустая подпись — одна строка', (tester) async {
    await tester.pumpWidget(app(PlaceLine(place: moscow)));
    expect(find.text('г Москва'), findsOneWidget);
    expect(find.byType(Text), findsOneWidget);
  });

  testWidgets('подсказки выбираются касанием, набранный текст — нет', (
    tester,
  ) async {
    Place? chosen;
    var changes = 0;
    final asked = <String>[];
    await tester.pumpWidget(
      app(
        StatefulBuilder(
          builder: (context, setState) => PlaceField(
            token: 'т',
            place: chosen,
            suggest: (q) async {
              asked.add(q);
              return [snt, village];
            },
            onChanged: (place) => setState(() {
              chosen = place;
              changes++;
            }),
          ),
        ),
      ),
    );

    await tester.enterText(find.byType(TextField), 'А');
    await tester.pump(const Duration(milliseconds: 400));
    expect(asked, isEmpty, reason: 'одна буква — справочник не спрашиваем');

    await tester.enterText(find.byType(TextField), ' Андрейково ');
    await tester.pump(const Duration(milliseconds: 400));
    expect(asked, ['Андрейково']);
    expect(find.text('Вологодский р-н, Вологодская обл'), findsOneWidget);
    expect(changes, 0, reason: 'набранный текст пунктом не становится');

    await tester.tap(find.text('снт Андрейково'));
    await tester.pump();
    expect(chosen, snt);
    expect(find.text('Убрать'), findsOneWidget);
    expect(find.text('Дмитровский р-н, Московская обл'), findsOneWidget);

    await tester.tap(find.text('Убрать'));
    await tester.pump();
    expect(chosen, isNull);
    expect(find.byType(TextField), findsOneWidget);
  });

  testWidgets('ничего не нашлось и справочник недоступен', (tester) async {
    var fail = false;
    await tester.pumpWidget(
      app(
        PlaceField(
          token: 'т',
          place: null,
          suggest: (q) async {
            if (fail) {
              throw Exception('503');
            }
            return const [];
          },
          onChanged: (_) {},
        ),
      ),
    );

    await tester.enterText(find.byType(TextField), 'Ыыыыы');
    await tester.pump(const Duration(milliseconds: 400));
    expect(find.text('Ничего не нашлось'), findsOneWidget);

    fail = true;
    await tester.enterText(find.byType(TextField), 'Ыыыыыы');
    await tester.pump(const Duration(milliseconds: 400));
    expect(find.text('Подсказки сейчас недоступны'), findsOneWidget);
  });

  testWidgets('пункты рядом по кнопке, набор их убирает', (tester) async {
    Place? chosen;
    await tester.pumpWidget(
      app(
        StatefulBuilder(
          builder: (context, setState) => PlaceField(
            token: 'т',
            place: chosen,
            nearby: () async => [snt, village],
            suggest: (q) async => [moscow],
            onChanged: (place) => setState(() => chosen = place),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Определить по месту'));
    await tester.pump();
    expect(find.text('снт Андрейково'), findsOneWidget);
    expect(find.text('д Андрейково'), findsOneWidget);

    await tester.enterText(find.byType(TextField), 'Моск');
    await tester.pump(const Duration(milliseconds: 400));
    expect(find.text('снт Андрейково'), findsNothing);
    expect(find.text('г Москва'), findsOneWidget);

    await tester.tap(find.text('Определить по месту'));
    await tester.pump();
    await tester.tap(find.text('снт Андрейково'));
    await tester.pump();
    expect(chosen, snt);
  });

  testWidgets('пусто рядом и нет доступа к месту', (tester) async {
    Object? answer = const <Place>[];
    await tester.pumpWidget(
      app(
        PlaceField(
          token: 'т',
          place: null,
          nearby: () async {
            final a = answer;
            if (a is Exception) {
              throw a;
            }
            return a as List<Place>;
          },
          onChanged: (_) {},
        ),
      ),
    );

    await tester.tap(find.text('Определить по месту'));
    await tester.pump();
    expect(
      find.text('Рядом ничего не нашлось. Найдите пункт по названию'),
      findsOneWidget,
    );

    answer = const NearbyProblem(
      'Нет доступа к месту. Найдите пункт по названию',
    );
    await tester.tap(find.text('Определить по месту'));
    await tester.pump();
    expect(
      find.text('Нет доступа к месту. Найдите пункт по названию'),
      findsOneWidget,
    );

    answer = Exception('503');
    await tester.tap(find.text('Определить по месту'));
    await tester.pump();
    expect(find.text('Подсказки сейчас недоступны'), findsOneWidget);
  });
}
