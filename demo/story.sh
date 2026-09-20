#!/usr/bin/env bash
# Показ одной user-story в эмуляторе: стенд с данными этого сценария,
# приложение поверх него, снимок экрана и список того, что человек
# проверяет глазами.
#
#   demo/story.sh 000-status
#
# Флаги (нужны в основном CI, где стенд и APK уже готовы):
#   --keep-stand   не пересоздавать стенд (тогда данные сценария должны
#                  быть налиты при его подъёме: DEMO_STORY=<id> make demo)
#   --no-apk       не пересобирать APK
set -euo pipefail

STORIES_DIR="demo/stories"
KEEP_STAND=""
NO_APK=""
STORY=""

for arg in "$@"; do
	case "$arg" in
	--keep-stand) KEEP_STAND=1 ;;
	--no-apk) NO_APK=1 ;;
	-*)
		echo "Неизвестный флаг: $arg" >&2
		exit 2
		;;
	*) STORY="$arg" ;;
	esac
done

list_stories() {
	echo "Сценарии показа:"
	local dir
	for dir in "$STORIES_DIR"/[0-9][0-9][0-9]-*/; do
		[ -d "$dir" ] || continue
		local id title
		id="$(basename "$dir")"
		title="$(sed -n 's/^# //p' "$dir/story.md" 2>/dev/null | head -1)"
		printf '  %-20s %s\n' "$id" "${title:-—}"
	done
	echo
	echo "Показать:  make story STORY=<сценарий>"
}

if [ -z "$STORY" ]; then
	list_stories
	exit 0
fi

STORY_DIR="$STORIES_DIR/$STORY"
if [ ! -d "$STORY_DIR" ]; then
	echo "Сценария '$STORY' нет." >&2
	echo >&2
	list_stories >&2
	exit 1
fi

TITLE="$(sed -n 's/^# //p' "$STORY_DIR/story.md" | head -1)"

# Настройки запуска сценария: чем отличается его прогон от прогона соседа.
MARKER=""
EXPECT=""
# shellcheck source=/dev/null
[ -f "$STORY_DIR/story.env" ] && . "$STORY_DIR/story.env"

echo "=============================================================="
echo " $STORY — ${TITLE:-без названия}"
echo "=============================================================="
echo

if [ -z "$KEEP_STAND" ]; then
	echo "-> Стенд заново, с данными сценария"
	make demo-reset
	DEMO_STORY="$STORY" make demo
else
	echo "-> Стенд уже поднят, оставляю как есть"
fi
echo

if [ -z "$NO_APK" ]; then
	echo "-> Сборка приложения"
	make app-apk
	echo
fi

echo "-> Приложение в эмуляторе"
export SCREENSHOT="demo/out/$STORY.png"
[ -n "$MARKER" ] && export MARKER || true
[ -n "$EXPECT" ] && export EXPECT || true
./demo/e2e.sh
echo

echo "=============================================================="
echo " Проверьте глазами"
echo "=============================================================="
# Раздел «Что проверить руками» из story.md — до следующего заголовка.
sed -n '/^## Что проверить руками$/,/^## /p' "$STORY_DIR/story.md" \
	| sed '1d; /^## /d'
echo "Снимок экрана: demo/out/$STORY.png"
echo "Целиком сценарий: $STORY_DIR/story.md"
