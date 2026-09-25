#!/usr/bin/env bash
# Настройка VPS под МоюДачу (ADR-0004): Postgres 17, Caddy, пользователь
# сервиса, юниты systemd, файрвол, ночной бэкап. Запускается от root шагом
# деплоя на каждом релизе и ничего не ломает при повторе: что уже сделано,
# пропускается.
#
#   DOMAIN=example.ru bash setup.sh
#
# Рядом со скриптом лежат файлы релиза: moya-dacha.service, backup.sh,
# moya-dacha-backup.service, moya-dacha-backup.timer.
set -euo pipefail

: "${DOMAIN:?DOMAIN не задан}"
here="$(cd "$(dirname "$0")" && pwd)"
export DEBIAN_FRONTEND=noninteractive

step() { echo "== $*"; }

# Памяти на маленьком VPS впритык: без подкачки при нехватке первой гибнет
# база. Файл подкачки заводится один раз.
if [ "$(swapon --noheadings | wc -l)" -eq 0 ]; then
	step "файл подкачки 1 ГБ"
	fallocate -l 1G /swapfile
	chmod 600 /swapfile
	mkswap /swapfile >/dev/null
	swapon /swapfile
	grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

# Postgres 17 из репозитория PGDG: в Ubuntu 24.04 своя шестнадцатая, а
# мажорная версия должна совпадать с тестами (ADR-0004).
if ! command -v caddy >/dev/null || [ ! -d /usr/lib/postgresql/17 ]; then
	step "пакеты: postgresql-17, caddy"
	apt-get update -q
	apt-get install -yq ca-certificates curl gnupg ufw
	install -d /usr/share/postgresql-common/pgdg /etc/apt/keyrings
	curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
		-o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc
	echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $(. /etc/os-release && echo "$VERSION_CODENAME")-pgdg main" \
		> /etc/apt/sources.list.d/pgdg.list
	curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key \
		| gpg --dearmor --yes -o /etc/apt/keyrings/caddy-stable.gpg
	echo "deb [signed-by=/etc/apt/keyrings/caddy-stable.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main" \
		> /etc/apt/sources.list.d/caddy-stable.list
	apt-get update -q
	apt-get install -yq postgresql-17 caddy
fi

if ! id moya-dacha >/dev/null 2>&1; then
	step "пользователь moya-dacha"
	useradd --system --home-dir /var/lib/moya-dacha --shell /usr/sbin/nologin moya-dacha
fi

# Пароль базы рождается здесь один раз и живёт только в /etc/moya-dacha.env.
if [ ! -f /etc/moya-dacha.env ]; then
	step "база и /etc/moya-dacha.env"
	password="$(head -c 24 /dev/urandom | base64 | tr -d '/+=')"
	sudo -u postgres psql -q -v ON_ERROR_STOP=1 \
		-c "CREATE ROLE moya_dacha LOGIN PASSWORD '$password'" \
		-c "CREATE DATABASE moya_dacha OWNER moya_dacha"
	umask 027
	cat > /etc/moya-dacha.env <<ENV
DATABASE_URL=postgres://moya_dacha:$password@127.0.0.1:5432/moya_dacha?sslmode=disable
ADDR=127.0.0.1:8080
MEDIA_DIR=/var/lib/moya-dacha/media
ENV
	umask 022
fi
# Пока нет SMS-провайдера, вход только по приглашениям (specs/015-invites.md).
grep -q '^AUTH_INVITES=' /etc/moya-dacha.env || echo 'AUTH_INVITES=1' >> /etc/moya-dacha.env
chown root:moya-dacha /etc/moya-dacha.env
chmod 640 /etc/moya-dacha.env

step "каталоги"
install -d -o moya-dacha -g moya-dacha -m 750 /var/backups/moya-dacha
install -d -m 755 /opt/moya-dacha /var/www/moya-dacha

step "юниты systemd"
install -m 644 "$here/moya-dacha.service" "$here/moya-dacha-backup.service" \
	"$here/moya-dacha-backup.timer" /etc/systemd/system/
install -m 755 "$here/backup.sh" /opt/moya-dacha/backup.sh
systemctl daemon-reload
systemctl enable moya-dacha.service >/dev/null
systemctl enable --now moya-dacha-backup.timer >/dev/null

# Caddy сам получает сертификат на домен. APK для тестировщиков лежит
# рядом и отдаётся по /app.apk.
step "Caddy для $DOMAIN"
sed "s/^example\.ru {/$DOMAIN {/" "$here/Caddyfile" > /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null
systemctl enable caddy >/dev/null
systemctl reload-or-restart caddy

step "файрвол: ssh, http, https"
ufw allow OpenSSH >/dev/null
ufw allow 80/tcp >/dev/null
ufw allow 443/tcp >/dev/null
ufw --force enable >/dev/null

echo "== настройка готова"
