-- Данные сценария «Правка, блокировка и удаление аккаунта».
--
-- Основа — «Живая дача» ([000-dacha]). Сверху три вещи, чтобы было на
-- что посмотреть сразу, ещё до правок руками:
-- * у самого свежего поста Николая (номер 01) подпись уже поправлена —
--   рядом со временем «изменено»;
-- * один комментарий Николая тоже поправлен;
-- * Николай уже заблокировал Сергея (09): список «Заблокированные» не
--   пустой, а постов и комментариев Сергея у Николая нет нигде.
-- Денис (11) — тот, кто удаляет аккаунт: у него есть посты, лайки,
-- комментарии и подписки, и видно, как они пропадают у всех.
\i /stories/000-dacha/seed.sql

UPDATE posts SET
    caption = caption || ' (поправил: сорт «Невский»)',
    edited_at = created_at + interval '10 minutes'
WHERE id = (
    SELECT p.id FROM posts p
    WHERE p.author_id = (SELECT id FROM dacha_users WHERE n = 1)
    ORDER BY p.created_at DESC LIMIT 1
);

UPDATE comments SET
    text = text || ' (дополнил)',
    edited_at = created_at + interval '5 minutes'
WHERE id = (
    SELECT c.id FROM comments c
    WHERE c.author_id = (SELECT id FROM dacha_users WHERE n = 1)
    ORDER BY c.created_at DESC LIMIT 1
);

-- Блокировка разрывает подписки в обе стороны: так же и здесь.
DELETE FROM follows
WHERE (follower_id, followee_id) IN (
    SELECT a.id, b.id FROM dacha_users a, dacha_users b
    WHERE (a.n, b.n) IN ((1, 9), (9, 1))
);

INSERT INTO blocks (blocker_id, blocked_id, created_at)
SELECT a.id, b.id, now() - interval '2 days'
FROM dacha_users a, dacha_users b
WHERE a.n = 1 AND b.n = 9;
