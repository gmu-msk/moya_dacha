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
MARKER='MOYA_DACHA_DEMO health='

if [ ! -f "$APK" ]; then
	echo "APK не найден: $APK — соберите его командой make app-apk" >&2
	exit 1
fi

adb wait-for-device
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
# а не только что приложение так считает. Если не получилось, не страшно.
if adb shell uiautomator dump /sdcard/ui.xml >/dev/null 2>&1; then
	echo "Текст на экране:"
	adb shell cat /sdcard/ui.xml 2>/dev/null \
		| grep -o 'text="[^"]\+"' | sed 's/^text="/  /; s/"$//' | sort -u
fi

case "$line" in
*health=ok*)
	echo "E2E пройден: эмулятор -> приложение -> API -> Postgres."
	;;
*)
	echo "E2E не пройден: приложение не получило ok от сервиса." >&2
	exit 1
	;;
esac
