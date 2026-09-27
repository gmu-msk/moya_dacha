-- Ошибки приложения (specs/021-app-errors.md): «Живая дача» и три ошибки
-- за неделю, чтобы раздел дашборда не стоял пустым.
\i /stories/000-dacha/seed.sql

-- Отпечаток — как у сервиса (требование 5): текст и первые 5 непустых
-- строк стека без цифр, пробелы сжаты. Тогда отчёт той же ошибки из
-- приложения или curl ляжет в группу из сценария, а не в новую.
CREATE FUNCTION pg_temp.norm(s text) RETURNS text
	LANGUAGE sql IMMUTABLE
	RETURN btrim(regexp_replace(regexp_replace(s, '[0-9]', '', 'g'), '\s+', ' ', 'g'));

CREATE FUNCTION pg_temp.fingerprint(err text, stack text) RETURNS text
	LANGUAGE sql IMMUTABLE
	RETURN encode(sha256(convert_to(concat_ws(E'\n', pg_temp.norm(err), (
		SELECT string_agg(l, E'\n' ORDER BY i) FROM (
			SELECT pg_temp.norm(x) AS l, i
			FROM unnest(string_to_array(stack, E'\n')) WITH ORDINALITY AS t(x, i)
			WHERE pg_temp.norm(x) <> ''
			ORDER BY i LIMIT 5) AS top)), 'UTF8')), 'hex');

CREATE TEMP TABLE demo_errors (n int, err text, stack text) ;
INSERT INTO demo_errors VALUES
	(1, 'Null check operator used on a null value',
	 E'#0      _PostScreenState.build (package:moya_dacha/screens/post_screen.dart:212:31)\n'
	 '#1      StatefulElement.build (package:flutter/src/widgets/framework.dart:5842:27)\n'
	 '#2      ComponentElement.performRebuild (package:flutter/src/widgets/framework.dart:5730:15)\n'
	 '#3      StatefulElement.performRebuild (package:flutter/src/widgets/framework.dart:5893:11)\n'
	 '#4      Element.rebuild (package:flutter/src/widgets/framework.dart:5445:7)\n'
	 '#5      BuildOwner.buildScope (package:flutter/src/widgets/framework.dart:2915:19)'),
	(2, 'RangeError (index): Invalid value: Not in inclusive range 0..2: 3',
	 E'#0      List.[] (dart:core-patch/growable_array.dart:264:36)\n'
	 '#1      _PhotoCarouselState._page (package:moya_dacha/widgets/feed_view.dart:318:24)\n'
	 '#2      _PhotoCarouselState.build.<anonymous closure> (package:moya_dacha/widgets/feed_view.dart:290:18)\n'
	 '#3      SliverChildBuilderDelegate.build (package:flutter/src/widgets/scroll_delegate.dart:497:22)'),
	(3, 'FormatException: Invalid radix-10 number (at character 1)',
	 E'#0      int._handleFormatError (dart:core-patch/integers_patch.dart:195:7)\n'
	 '#1      int.parse (dart:core-patch/integers_patch.dart:126:16)\n'
	 '#2      _ServerScreenState._check (package:moya_dacha/screens/server_screen.dart:88:17)\n'
	 '#3      <asynchronous suspension>');

-- Отчёты: n ошибки, кто (номер из «Живой дачи», пусто — без входа),
-- сколько назад, экран, сборка.
CREATE TEMP TABLE demo_reports (n int, who int, ago interval, screen text, build bigint);
INSERT INTO demo_reports VALUES
	(1,  4, '2 hours 5 minutes',  'post', 386900),
	(1,  9, '6 hours',            'post', 386900),
	(1,  9, '6 hours 2 minutes',  'post', 386900),
	(1, 14, '1 day 3 hours',      'post', 386900),
	(1,  4, '2 days',             'post', 386870),
	(2,  2, '1 day 1 hour',       'feed', 386900),
	(2,  2, '4 days',             'feed', 386870),
	(3, NULL, '3 days 5 hours',   'server', 386870);

INSERT INTO app_error_groups (fingerprint, first_at, last_at, notified_at)
SELECT pg_temp.fingerprint(e.err, e.stack),
       now() - max(r.ago), now() - min(r.ago), now() - max(r.ago) + interval '1 minute'
FROM demo_errors e JOIN demo_reports r ON r.n = e.n
GROUP BY e.n, e.err, e.stack;

INSERT INTO app_errors (group_id, at, user_id, error, stack, version, build, screen, os)
SELECT g.id, now() - r.ago, u.id, e.err, e.stack, '1.0.0', r.build, r.screen,
       'android 14'
FROM demo_reports r
JOIN demo_errors e ON e.n = r.n
JOIN app_error_groups g ON g.fingerprint = pg_temp.fingerprint(e.err, e.stack)
LEFT JOIN dacha_users u ON u.n = r.who;
