-- Обратная связь (specs/019-feedback.md): «Живая дача» и отзывы Николая
-- (+7 900 000-00-01) во всех статусах, чтобы список «Мои отзывы» было
-- видно целиком, а на дашборде был блок «Отзывы».
\i /stories/000-dacha/seed.sql

INSERT INTO feedback (source, user_id, author, text, app_version, device, status, issue, issue_url, build, created_at)
VALUES
    ('app', '11111111-1111-4111-8111-111111111111', 'Николай (@kolya_kartofel)',
     'Лента не обновляется после нового поста, приходится выходить и заходить',
     '1.0.0 (386900)', 'Google Pixel 7, Android 14', 'released', 71,
     'https://github.com/gmu-msk/moya_dacha/issues/71', 386950, now() - interval '6 days'),
    ('app', '11111111-1111-4111-8111-111111111111', 'Николай (@kolya_kartofel)',
     'Хочется сортировать свои посты по культурам: томаты отдельно, огурцы отдельно',
     '1.0.0 (386900)', 'Google Pixel 7, Android 14', 'approved', 74,
     'https://github.com/gmu-msk/moya_dacha/issues/74', NULL, now() - interval '3 days'),
    ('app', '11111111-1111-4111-8111-111111111111', 'Николай (@kolya_kartofel)',
     'Сделайте тёмную тему ещё темнее',
     '1.0.0 (386900)', 'Google Pixel 7, Android 14', 'declined', 75,
     'https://github.com/gmu-msk/moya_dacha/issues/75', NULL, now() - interval '2 days'),
    ('app', '11111111-1111-4111-8111-111111111111', 'Николай (@kolya_kartofel)',
     'Кнопка «Опубликовать» прячется под клавиатурой',
     '1.0.0 (386950)', 'Google Pixel 7, Android 14', 'accepted', 78,
     'https://github.com/gmu-msk/moya_dacha/issues/78', NULL, now() - interval '5 hours'),
    ('telegram', NULL, '@tester_olga',
     'Хочу напоминание полить рассаду', NULL, NULL, 'sent', NULL, NULL, NULL, now() - interval '1 hour');
