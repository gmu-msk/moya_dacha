#!/usr/bin/env bash
# Посредник до Telegram Bot API на втором VPS (specs/018-telegram-bot.md,
# требование 3б). С прода api.telegram.org не открывается, со второго VPS
# открывается. Caddy на втором VPS пересылает пути /bot… в Telegram как
# есть; токен идёт в пути запроса, посредник его не хранит и не пишет.
#
# Запускает деплой под root: DOMAIN=<ip-через-дефисы>.sslip.io bash telegram-relay.sh
# Идемпотентен. Чужой Caddyfile не перезаписывает.
set -euo pipefail
: "${DOMAIN:?DOMAIN не задан}"
marker="# moya-dacha: посредник Telegram"

step() { echo "==> $*"; }

fresh=0
if ! command -v caddy >/dev/null; then
	step "пакет caddy"
	fresh=1
	apt-get update -q
	apt-get install -yq ca-certificates curl gnupg
	install -d /etc/apt/keyrings
	curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key \
		| gpg --dearmor --yes -o /etc/apt/keyrings/caddy-stable.gpg
	echo "deb [signed-by=/etc/apt/keyrings/caddy-stable.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main" \
		> /etc/apt/sources.list.d/caddy-stable.list
	apt-get update -q
	apt-get install -yq caddy
fi

# Caddy уже стоял и настроен под что-то своё — не трогаем.
if [ "$fresh" = 0 ] && [ -f /etc/caddy/Caddyfile ] && ! grep -qF "$marker" /etc/caddy/Caddyfile; then
	echo "На этой машине уже работает свой Caddy (/etc/caddy/Caddyfile), посредник не поставлен." >&2
	exit 1
fi

step "Caddyfile посредника"
cat > /etc/caddy/Caddyfile <<CADDY
$marker
$DOMAIN {
	@bot path /bot*
	handle @bot {
		reverse_proxy https://api.telegram.org {
			header_up Host api.telegram.org
		}
	}
	handle {
		respond 404
	}
}
CADDY

if command -v ufw >/dev/null && ufw status | grep -q "Status: active"; then
	step "ufw: 80, 443"
	ufw allow 80/tcp >/dev/null
	ufw allow 443/tcp >/dev/null
fi

systemctl enable -q caddy
systemctl reload-or-restart caddy

step "Telegram отсюда"
curl -fsS -m 15 -o /dev/null https://api.telegram.org && echo "api.telegram.org открывается"
