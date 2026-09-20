#!/usr/bin/env bash
# Проверка формы сценариев показа (demo/stories/README.md): у каждого
# сценария есть название, разделы и свои данные.
#
# Отдельно, уже не как проверка, печатает спецификации фич, у которых
# сценария показа ещё нет: гейт в проекте один — тесты (ADR-0002), а это
# подсказка ревьюеру.
set -euo pipefail

STORIES_DIR="demo/stories"
SPECS_DIR="specs"
SECTIONS=("## Что показываем" "## Данные" "## Что проверить руками" "## Спецификация")
KNOWN_KEYS="MARKER EXPECT"

failed=0

fail() {
	echo "  ошибка: $1" >&2
	failed=1
}

for dir in "$STORIES_DIR"/*/; do
	id="$(basename "$dir")"
	echo "$id"

	case "$id" in
	[0-9][0-9][0-9]-*) ;;
	*)
		fail "имя папки должно быть <NNN>-<slug>, по номеру фичи из specs/000-overview.md"
		continue
		;;
	esac

	if [ ! -f "$dir/story.md" ]; then
		fail "нет story.md"
		continue
	fi
	[ -f "$dir/seed.sql" ] || fail "нет seed.sql (пустой файл — тоже ответ)"

	grep -q '^# .' "$dir/story.md" || fail "в story.md нет заголовка '# <название>'"

	for section in "${SECTIONS[@]}"; do
		grep -qF -x "$section" "$dir/story.md" || fail "в story.md нет раздела '$section'"
	done

	if [ -f "$dir/story.env" ]; then
		while IFS= read -r line; do
			case "$line" in
			"" | \#*) continue ;;
			esac
			key="${line%%=*}"
			case " $KNOWN_KEYS " in
			*" $key "*) ;;
			*) fail "в story.env неизвестный ключ '$key' (знаем: $KNOWN_KEYS)" ;;
			esac
		done < "$dir/story.env"
	fi
done

if [ "$failed" -ne 0 ]; then
	echo >&2
	echo "Форма сценария описана в $STORIES_DIR/README.md" >&2
	exit 1
fi

echo
echo "Сценарии показа в порядке."

# Спецификации фич без сценария. 000-overview.md — не фича, её пропускаем.
missing=""
for spec in "$SPECS_DIR"/[0-9][0-9][0-9]-*.md; do
	[ -f "$spec" ] || continue
	name="$(basename "$spec" .md)"
	case "$name" in
	000-*) continue ;;
	esac
	[ -d "$STORIES_DIR/$name" ] || missing="$missing $name"
done

if [ -n "$missing" ]; then
	echo
	echo "Спецификации фич без сценария показа:"
	for name in $missing; do
		echo "  $name — нет $STORIES_DIR/$name/"
	done
	echo
	echo "Фича без сценария показа — красный флаг на ревью (ADR-0009)."
fi
