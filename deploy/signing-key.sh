#!/usr/bin/env bash
# Постоянный ключ подписи APK (ADR-0021). Живёт на VPS, на раннер приходит
# на время сборки. Сборка, не нашедшая ключа, создаёт его сама — так ключ
# появляется без ручных шагов. Нужен алиас `vps` (deploy/ssh-setup.sh).
#
# Пишет mobile/android/key.properties: по нему Gradle подписывает и
# release-, и debug-сборку. Сам ключ ложится вне репозитория.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
remote=/var/lib/moya-dacha/signing/key.env
sudo_='sudo=$( [ "$(id -u)" = 0 ] || echo sudo );'
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

if ! ssh vps "$sudo_ \$sudo test -s $remote"; then
	echo "Ключа подписи на сервере нет — создаю"
	password=$(openssl rand -hex 24)
	keytool -genkeypair -noprompt \
		-keystore "$work/new.jks" -storetype PKCS12 \
		-alias moyadacha -keyalg RSA -keysize 4096 -validity 36500 \
		-storepass "$password" -keypass "$password" \
		-dname "CN=Moya Dacha, O=Moya Dacha, C=RU"
	{
		echo "STORE_PASSWORD=$password"
		echo "KEYSTORE_B64=$(base64 -w0 "$work/new.jks")"
	} > "$work/new.env"
	# set -C: файл создаётся, только если его ещё нет. Две сборки, разом не
	# нашедшие ключа, сохранят один, и обе возьмут его ниже.
	ssh vps "$sudo_ \$sudo install -d -m 700 $(dirname "$remote") &&
		\$sudo sh -c 'umask 077; set -C; cat > $remote' 2>/dev/null || true" \
		< "$work/new.env"
fi

ssh vps "$sudo_ \$sudo cat $remote" > "$work/key.env"
# shellcheck source=/dev/null
. "$work/key.env"
: "${STORE_PASSWORD:?в ключе на сервере нет пароля}"
: "${KEYSTORE_B64:?в ключе на сервере нет самого ключа}"

store="${RUNNER_TEMP:-$HOME}/moya-dacha-release.jks"
printf '%s' "$KEYSTORE_B64" | base64 -d > "$store"
chmod 600 "$store"
umask 077
cat > "$here/../mobile/android/key.properties" <<PROPS
storeFile=$store
storePassword=$STORE_PASSWORD
keyAlias=moyadacha
keyPassword=$STORE_PASSWORD
PROPS
echo "Ключ подписи на месте"
