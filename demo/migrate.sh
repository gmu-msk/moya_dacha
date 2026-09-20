#!/bin/sh
# Шаг миграций демо-стенда. Отдельный шаг, а не действие при старте
# приложения — как и в проде (ADR-0005). Только вперёд, down-миграций нет.
set -eu

if ! ls /migrations/*.sql >/dev/null 2>&1; then
	echo "Миграций пока нет: в backend/migrations нет ни одного .sql — пропускаю шаг."
	exit 0
fi

goose -dir /migrations postgres "$DATABASE_URL" up
goose -dir /migrations postgres "$DATABASE_URL" status
