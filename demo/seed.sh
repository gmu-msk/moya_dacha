#!/bin/sh
# Наполнение демо-стенда данными.
#
# Сначала общие данные — demo/seed/*.sql по порядку имён; потом, если
# стенд поднимают под конкретный сценарий показа (DEMO_STORY), данные
# этого сценария — demo/stories/<сценарий>/seed.sql.
set -eu

if ls /seed/*.sql >/dev/null 2>&1; then
	for file in /seed/*.sql; do
		echo "-> $file"
		psql -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -f "$file"
	done
else
	echo "Общих демо-данных нет: в demo/seed нет ни одного .sql — пропускаю."
fi

STORY="${DEMO_STORY:-}"
[ -n "$STORY" ] || exit 0

STORY_SEED="/stories/$STORY/seed.sql"
if [ ! -f "$STORY_SEED" ]; then
	echo "Сценария '$STORY' нет: не найден $STORY_SEED" >&2
	exit 1
fi

echo "-> сценарий $STORY: $STORY_SEED"
psql -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -f "$STORY_SEED"
