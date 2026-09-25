-- Данные сценария «Дашборд»: «Живая дача» плюс сутки измерений сервера
-- и несколько ошибок, чтобы графики сервера не стояли пустыми, пока
-- стенд не намерил своё (specs/016-dashboard.md).

\i /stories/000-dacha/seed.sql

-- Сутки измерений раз в минуту: днём дачники заходят чаще, ночью тихо.
-- Память и диск — как у VPS на 2 ГБ и 20 ГБ.
INSERT INTO server_samples (measured_at, cpu_percent, memory_used_bytes, memory_total_bytes,
                            swap_used_bytes, disk_used_bytes, disk_total_bytes, requests)
SELECT now() - make_interval(mins => m),
       greatest(1, 4 + 10 * day_load + 3 * sin(m / 7.0)),
       (620 + 140 * day_load + 20 * sin(m / 30.0))::bigint * 1024 * 1024,
       2048::bigint * 1024 * 1024,
       0,
       (5.2 * 1024 + (1440 - m) * 0.05)::bigint * 1024 * 1024,
       20::bigint * 1024 * 1024 * 1024,
       (2 + 40 * day_load + 5 * abs(sin(m / 3.0)))::bigint
FROM generate_series(1, 1440) AS m,
     LATERAL (SELECT greatest(0, sin(pi() * (extract(hour FROM (now() - make_interval(mins => m))
                 AT TIME ZONE 'Europe/Moscow') - 7) / 16))) AS l(day_load);

-- Две ошибки за сутки: обе давно, тревоги «ошибки» нет.
INSERT INTO server_errors (at, method, path, status, message) VALUES
    (now() - interval '5 hours 12 minutes', 'POST', '/api/posts', 500,
     'timeout: context deadline exceeded'),
    (now() - interval '19 hours 40 minutes', 'PUT', '/api/me/avatar', 500,
     'не удалось сохранить файл: no space left on device');
