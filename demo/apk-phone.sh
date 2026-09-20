#!/usr/bin/env bash
# Сборка APK для телефона: тот же debug-APK, но с адресом демо-стенда
# в локальной сети вместо адреса эмулятора.
#
#   make apk-phone            собрать
#   make apk-phone-install    собрать и поставить на телефон по USB
#
# Стенд при этом должен быть поднят так, чтобы его было видно из сети:
# `make demo-lan` (обычный `make demo` слушает только 127.0.0.1).
set -euo pipefail

INSTALL=""
[ "${1:-}" = "--install" ] && INSTALL=1

IP="$(./demo/lan-ip.sh)"
API_BASE_URL="http://$IP:8080/api"
OUT="demo/out/moya-dacha-$IP.apk"

echo "Адрес стенда для телефона: $API_BASE_URL"

if curl -fsS --max-time 3 "$API_BASE_URL/health" >/dev/null 2>&1; then
	echo "Стенд по этому адресу отвечает."
else
	echo
	echo "Внимание: по $API_BASE_URL/health сейчас никто не отвечает."
	echo "APK всё равно соберётся, но телефон к стенду не подключится."
	echo "Поднимите стенд видимым в сети: make demo-lan"
	echo
fi

(cd mobile && flutter build apk --debug --dart-define=API_BASE_URL="$API_BASE_URL")

mkdir -p "$(dirname "$OUT")"
cp mobile/build/app/outputs/flutter-apk/app-debug.apk "$OUT"

if [ -n "$INSTALL" ]; then
	echo
	echo "Ставлю на телефон по USB (adb -d)."
	adb -d install -r "$OUT"
	echo "Готово: приложение на телефоне, адрес стенда $API_BASE_URL."
	exit 0
fi

cat <<TEXT

APK: $OUT
Адрес стенда внутри него: $API_BASE_URL

Как поставить на телефон:

  по USB       подключить телефон, включить отладку по USB и
               make apk-phone-install
  без провода  перекинуть файл на телефон (AirDrop, облако, почта) и
               открыть его; Android спросит разрешение на установку
               из этого источника

Телефон и этот компьютер должны быть в одной сети Wi-Fi, а стенд поднят
командой make demo-lan. Если адрес машины в сети сменился, APK надо
пересобрать: адрес зашит в него при сборке.
TEXT
