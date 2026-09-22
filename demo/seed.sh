#!/bin/sh
# Наполнение демо-стенда данными.
#
# Сначала фотографии, потом общие данные — demo/seed/*.sql по порядку
# имён; потом данные сценария показа (DEMO_STORY) —
# demo/stories/<сценарий>/seed.sql.
set -eu

# Демо-данные с фотографиями и аватарами ссылаются на файлы в хранилище
# (specs/003-posts.md). Сами файлы в базу не положишь, поэтому они лежат
# в demo/seed/media и раскладываются сюда — иначе лента была бы из пустых
# рамок. Ключи в SQL начинаются с demo/.
if [ -d /seed/media ] && [ -d /media ]; then
	mkdir -p /media/demo
	cp /seed/media/*.jpg /media/demo/
	chmod -R a+rX /media/demo
	echo "-> фотографии сценариев: /media/demo"
fi

if ls /seed/*.sql >/dev/null 2>&1; then
	for file in /seed/*.sql; do
		echo "-> $file"
		psql -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -f "$file"
	done
else
	echo "Общих демо-данных нет: в demo/seed нет ни одного .sql — пропускаю."
fi

# Без сценария стенд поднимается «Живой дачей» (demo/stories/000-dacha):
# `/стенд` и `make demo` сразу показывают ленту, где идёт жизнь. Пустая
# лента — сценарий без своих данных, например DEMO_STORY=003-posts.
STORY="${DEMO_STORY:-000-dacha}"

STORY_SEED="/stories/$STORY/seed.sql"
if [ ! -f "$STORY_SEED" ]; then
	echo "Сценария '$STORY' нет: не найден $STORY_SEED" >&2
	exit 1
fi

echo "-> сценарий $STORY: $STORY_SEED"
psql -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -f "$STORY_SEED"
