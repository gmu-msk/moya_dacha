#!/usr/bin/env bash
# Печатает адрес этой машины в локальной сети — тот, по которому до
# демо-стенда достучится телефон из той же сети Wi-Fi.
#
# Если угадать не выходит (несколько интерфейсов, VPN, непривычная сеть),
# адрес задаётся руками: LAN_IP=192.168.1.10 make apk-phone
set -euo pipefail

if [ -n "${LAN_IP:-}" ]; then
	echo "$LAN_IP"
	exit 0
fi

ip=""

case "$(uname -s)" in
Darwin)
	# Интерфейс маршрута по умолчанию: на Маке Wi-Fi это не всегда en0.
	iface="$(route -n get default 2>/dev/null | awk '/interface:/ {print $2; exit}' || true)"
	for candidate in "$iface" en0 en1; do
		[ -n "$candidate" ] || continue
		ip="$(ipconfig getifaddr "$candidate" 2>/dev/null || true)"
		[ -n "$ip" ] && break
	done
	;;
Linux)
	ip="$(ip -4 route get 1.1.1.1 2>/dev/null |
		awk '{for (i = 1; i <= NF; i++) if ($i == "src") {print $(i + 1); exit}}' || true)"
	[ -n "$ip" ] || ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
	;;
esac

if [ -z "$ip" ]; then
	cat >&2 <<'TEXT'
Не смог определить адрес машины в локальной сети.

Посмотрите его сами (на Маке: Системные настройки -> Сеть -> Wi-Fi ->
Подробности, или `ipconfig getifaddr en0`) и передайте руками:

    LAN_IP=192.168.1.10 make apk-phone
TEXT
	exit 1
fi

echo "$ip"
