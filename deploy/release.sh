#!/usr/bin/env bash
# Шаг деплоя на VPS (ADR-0005): дамп базы → миграции → новый бинарник.
# Запускается от root из распакованного релиза после setup.sh. Если
# миграция упала, старый бинарник продолжает работать на старой схеме,
# а свежий дамп лежит в /var/backups/moya-dacha.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
set -a
# shellcheck source=/dev/null
. /etc/moya-dacha.env
set +a

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
echo "== дамп перед миграцией"
pg_dump --format=custom --file="/var/backups/moya-dacha/predeploy-$stamp.dump" "$DATABASE_URL"
chown moya-dacha:moya-dacha "/var/backups/moya-dacha/predeploy-$stamp.dump"
# Дампов перед деплоем хватит десяти последних.
ls -1t /var/backups/moya-dacha/predeploy-*.dump | tail -n +11 | xargs -r rm --

echo "== миграции"
"$here/goose" -dir "$here/migrations" postgres "$DATABASE_URL" up

echo "== новый бинарник"
install -m 755 "$here/moya-dacha-server" /usr/local/bin/moya-dacha-server
systemctl restart moya-dacha

for _ in $(seq 1 30); do
	if curl -fsS http://127.0.0.1:8080/api/health >/dev/null; then
		echo "== сервис отвечает"
		exit 0
	fi
	sleep 1
done
echo "сервис не ответил на /api/health за 30 секунд" >&2
journalctl -u moya-dacha -n 50 --no-pager >&2
exit 1
