#!/usr/bin/env bash
# Прогон приложения на подключённом Android-устройстве (обычно эмуляторе)
# против демо-стенда: поставить APK, запустить, дождаться в логах ответа
# /api/health и снять экран.
#
# Сам стенд должен быть уже поднят (`make demo`), APK собран (`make app-apk`).
set -euo pipefail

APK="${APK:-mobile/build/app/outputs/flutter-apk/app-debug.apk}"
PACKAGE="${PACKAGE:-ru.moyadacha.app}"
SCREENSHOT="${SCREENSHOT:-demo/out/e2e-screenshot.png}"
# Чего ждём в логах приложения. Сценарий показа переопределяет это парой
# MARKER/EXPECT в своём story.env — см. demo/stories/README.md.
MARKER="${MARKER:-MOYA_DACHA_DEMO health=}"
EXPECT="${EXPECT:-*health=ok*}"

if [ ! -f "$APK" ]; then
	echo "APK не найден: $APK — соберите его командой make app-apk" >&2
	exit 1
fi

if ! command -v adb >/dev/null 2>&1; then
	echo "Не найден adb: поставьте Android SDK platform-tools и добавьте их в PATH." >&2
	exit 1
fi

# `adb wait-for-device` без эмулятора ждёт молча и вечно, поэтому ждём
# сами, с пределом и подсказкой: сначала устройство, потом конец загрузки
# системы — до него установка APK тоже висит.
printf 'Жду эмулятор'
deadline=$((SECONDS + 120))
until [ "$(adb get-state 2>/dev/null || true)" = "device" ] &&
	[ "$(adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; do
	if ((SECONDS > deadline)); then
		echo
		echo "Эмулятор не найден или не загрузился за 2 минуты." >&2
		echo "Запустите его (Android Studio → Device Manager → ▶ или" >&2
		echo "  emulator -avd <имя>; список имён — emulator -list-avds)" >&2
		echo "и повторите команду. Сейчас adb видит:" >&2
		adb devices >&2 || true
		exit 1
	fi
	printf '.'
	sleep 2
done
echo

adb install -r "$APK"
adb logcat -c
adb shell am start -n "$PACKAGE/.MainActivity"

printf 'Жду ответа приложения'
deadline=$((SECONDS + 90))
line=""
until [ -n "$line" ]; do
	line="$(adb logcat -d | grep -o "$MARKER.*" | tail -1 || true)"
	if [ -z "$line" ]; then
		if ((SECONDS > deadline)); then
			echo
			echo "Приложение не сообщило о результате за 90 секунд. Хвост логов:" >&2
			adb logcat -d | tail -50 >&2
			exit 1
		fi
		printf '.'
		sleep 2
	fi
done
echo

# Приложение пишет в лог сразу после ответа сервиса, экран перерисовывается
# чуть позже — иначе на снимке будет крутилка, а не результат.
sleep 2

mkdir -p "$(dirname "$SCREENSHOT")"
adb exec-out screencap -p > "$SCREENSHOT"

echo "Приложение сообщило: $line"
echo "Снимок экрана: $SCREENSHOT"

# Текст с экрана — чтобы по логу было видно, что именно показано человеку,
# а не только что приложение так считает. Вывод справочный: если дамп
# не получился или оказался пустым, прогон это не роняет.
print_screen_text() {
	adb shell uiautomator dump /sdcard/ui.xml >/dev/null 2>&1 || return 0

	local texts
	texts="$(adb shell cat /sdcard/ui.xml 2>/dev/null \
		| grep -o 'text="[^"]\+"' | sed 's/^text="/  /; s/"$//' | sort -u)" || return 0
	[ -n "$texts" ] || return 0

	echo "Текст на экране:"
	printf '%s\n' "$texts"
}

print_screen_text || true

# shellcheck disable=SC2254  # EXPECT — это шаблон, кавычки его сломают.
case "$line" in
$EXPECT)
	echo "E2E пройден: эмулятор -> приложение -> API -> Postgres."
	;;
*)
	echo "E2E не пройден: в логе '$line', ожидалось '$EXPECT'." >&2
	exit 1
	;;
esac
