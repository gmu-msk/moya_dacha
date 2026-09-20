#!/usr/bin/env bash
# Postgres для тестов без Docker: нативный кластер на порту 55432.
#
# Нужен там, где docker недоступен (облачная сессия агента). Обычный путь —
# `make test`, он поднимает Postgres в контейнере; этот скрипт — запасной.
#
#   scripts/pg-local.sh start|stop|status
set -euo pipefail

PORT=55432
DATADIR=${PGLOCAL_DATADIR:-/var/lib/postgresql/moya-dacha-test}
DBUSER=moya_dacha
DBNAME=moya_dacha_test

bindir() {
	local d
	d=$(ls -d /usr/lib/postgresql/*/bin 2>/dev/null | sort -V | tail -1 || true)
	if [ -z "$d" ]; then
		echo "Не найден Postgres в /usr/lib/postgresql. Поставьте postgresql или используйте 'make test' с Docker." >&2
		exit 1
	fi
	echo "$d"
}

# Всё, что трогает каталог кластера, Postgres отказывается делать от root.
as_pg() {
	if [ "$(id -u)" = 0 ]; then su postgres -c "$1"; else sh -c "$1"; fi
}

running() {
	as_pg "$(bindir)/pg_ctl -D $DATADIR status" >/dev/null 2>&1
}

start() {
	if running; then
		echo "Postgres для тестов уже поднят на порту $PORT"
		return 0
	fi
	if [ ! -s "$DATADIR/PG_VERSION" ]; then
		rm -rf "$DATADIR"
		mkdir -p "$DATADIR"
		[ "$(id -u)" = 0 ] && chown postgres:postgres "$DATADIR"
		as_pg "$(bindir)/initdb -D $DATADIR -U $DBUSER --auth=trust" >/dev/null
	fi
	as_pg "$(bindir)/pg_ctl -D $DATADIR -o '-p $PORT -c listen_addresses=127.0.0.1' -l $DATADIR/server.log start" >/dev/null
	as_pg "$(bindir)/createdb -h 127.0.0.1 -p $PORT -U $DBUSER $DBNAME" 2>/dev/null || true
	echo "Postgres для тестов поднят на порту $PORT, база $DBNAME"
}

stop() {
	running || { echo "Postgres для тестов не запущен"; return 0; }
	as_pg "$(bindir)/pg_ctl -D $DATADIR -m fast stop" >/dev/null
	echo "Postgres для тестов остановлен"
}

case "${1:-start}" in
	start) start ;;
	stop) stop ;;
	status) running && echo "поднят" || { echo "не запущен"; exit 1; } ;;
	*) echo "Использование: $0 start|stop|status" >&2; exit 2 ;;
esac
