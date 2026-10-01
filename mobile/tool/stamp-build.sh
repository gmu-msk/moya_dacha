#!/usr/bin/env bash
# Записать сведения о сборке в assets/build_info.json и напечатать номер
# сборки для `flutter build --build-number` (specs/017-app-updates.md).
# Версию для `--build-name` брать из того же файла: `jq -r .version`.
# Запускается на CI перед сборкой APK для телефона.
#
#   WHATS_NEW="строка"   «Что нового» одной строкой — сборка PR
#   SINCE=<коммит>       иначе заголовки PR, вошедших в main после него
#   COMMIT=<sha>         коммит сборки, если HEAD не он (merge-коммит PR)
#
# Номер — минуты от 2026-01-01 UTC. Он растёт между любыми сборками, из
# какого бы workflow они ни шли: номер прогона у каждого workflow свой,
# и сборка main оказалась бы «старше» сборки PR и не встала бы поверх.
set -euo pipefail
cd "$(dirname "$0")/.."

build=$(( ($(date -u +%s) - 1767225600) / 60 ))
# Версия — «старшая.младшая» из pubspec.yaml и номер сборки третьим
# числом: её Android показывает в настройках, и она должна расти вместе
# с номером, а не стоять на 1.0.0.
version=$(sed -n 's/^version: *\([0-9]*\.[0-9]*\).*/\1/p' pubspec.yaml).$build
# COMMIT берётся как есть: в неглубоком checkout PR самого коммита нет.
if [ -n "${COMMIT:-}" ]; then
	commit=${COMMIT:0:7}
else
	commit=$(git rev-parse --short=7 HEAD)
fi

if [ -n "${WHATS_NEW:-}" ]; then
	lines=$WHATS_NEW
elif [ -n "${SINCE:-}" ] && git cat-file -e "$SINCE^{commit}" 2>/dev/null; then
	# В main попадают только PR, слитые одним коммитом: заголовок
	# коммита — это и есть заголовок PR.
	lines=$(git log --first-parent --format=%s "$SINCE..HEAD" | head -10)
else
	lines=''
fi
# Прошлую сборку узнать нельзя или новых PR нет (перезапуск деплоя) —
# последний заголовок.
[ -n "$lines" ] || lines=$(git log -1 --format=%s HEAD)

printf '%s\n' "$lines" | jq -R . | jq -s \
	--arg version "$version" \
	--argjson build "$build" \
	--arg date "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
	--arg commit "$commit" \
	'{version: $version, build: $build, date: $date, commit: $commit, whatsNew: .}' \
	> assets/build_info.json

echo "$build"
