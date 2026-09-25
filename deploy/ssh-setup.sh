#!/usr/bin/env bash
# Ключ и алиас `vps` для ssh/scp в джобах GitHub Actions. Секреты приходят
# переменными окружения и на диск раннера ложатся только в ~/.ssh.
set -euo pipefail
: "${VPS_HOST:?секрет VPS_HOST не задан}"
: "${VPS_SSH_KEY:?секрет VPS_SSH_KEY не задан}"
user="${VPS_USER:-root}"

install -d -m 700 ~/.ssh
printf '%s\n' "$VPS_SSH_KEY" > ~/.ssh/vps
chmod 600 ~/.ssh/vps
cat > ~/.ssh/config <<CFG
Host vps
	HostName $VPS_HOST
	User $user
	IdentityFile ~/.ssh/vps
	StrictHostKeyChecking accept-new
	ServerAliveInterval 30
CFG
