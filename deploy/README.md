# Деплой

Прод — один бинарник под systemd, Postgres и Caddy из пакетов (ADR-0004).
Деплой выполняется только из GitHub Actions и только с `main`.

Порядок шага деплоя (ADR-0005):

1. `pg_dump` текущей базы, копия — в объектное хранилище, не на этот VPS
2. `make migrate-up` — миграции, только вперёд
3. замена бинарника и `systemctl restart moya-dacha`

Если миграция упала, старый бинарник продолжает работать на старой схеме,
а на руках свежий дамп.

## Файлы

- `moya-dacha.service` — юнит systemd, в `/etc/systemd/system/`
- `Caddyfile` — в `/etc/caddy/`, домен подставить свой
- `/etc/moya-dacha.env` — секреты (`DATABASE_URL`, `ADDR`), в git не хранятся
