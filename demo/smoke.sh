#!/usr/bin/env bash
# Ждёт, пока демо-стенд ответит на /api/health, и печатает адреса,
# по которым к нему подключаться. Запускается из `make demo`.
set -euo pipefail

API_BASE="${API_BASE:-http://127.0.0.1:8080/api}"
DEADLINE=$((SECONDS + 120))

printf 'Жду ответа от %s/health' "$API_BASE"
until response="$(curl -fsS "$API_BASE/health" 2>/dev/null)"; do
	if ((SECONDS > DEADLINE)); then
		echo
		echo "Стенд не поднялся за две минуты. Логи: make demo-logs" >&2
		exit 1
	fi
	printf '.'
	sleep 2
done
echo

cat <<TEXT
Стенд поднят.

  API                 $API_BASE
  Проверка живости    $API_BASE/health -> $response
  Postgres            postgres://moya_dacha:moya_dacha@127.0.0.1:55433/moya_dacha
  Android-эмулятор    http://10.0.2.2:8080/api
TEXT

# Стенд, поднятый `make demo-lan`, слушает все интерфейсы — значит, до него
# дойдёт телефон из той же сети. Печатаем адрес, по которому идти.
if [ "${DEMO_BIND_ADDR:-127.0.0.1}" != "127.0.0.1" ]; then
	lan_ip="$(./demo/lan-ip.sh 2>/dev/null || true)"
	echo "  Телефон в той же сети  http://${lan_ip:-<IP этой машины>}:8080/api"
	echo
	echo "  Стенд открыт в локальную сеть: его видит не только эта машина."
	echo "  Приложение для телефона с этим адресом:  make apk-phone"
else
	echo "  Порты слушаются только на 127.0.0.1. Для телефона: make demo-lan"
fi

cat <<TEXT

  Показать сценарий   make story STORY=<сценарий>   (make stories — список)
  Логи                make demo-logs
  Консоль базы        make demo-psql
  Остановить          make demo-down     (данные остаются)
  Стереть и заново    make demo-reset && make demo
TEXT
