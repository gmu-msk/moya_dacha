#!/usr/bin/env bash
# Ночной дамп базы (ADR-0005). Хранятся четырнадцать последних.
#
# Дамп лежит на том же диске, что и база: от ошибки в данных он спасает,
# от потери самого VPS — нет. Копия наружу — отдельный шаг, когда появится
# хранилище для неё.
set -euo pipefail

set -a
# shellcheck source=/dev/null
. /etc/moya-dacha.env
set +a

dir=/var/backups/moya-dacha
pg_dump --format=custom --file="$dir/nightly-$(date -u +%Y%m%d).dump" "$DATABASE_URL"
ls -1t "$dir"/nightly-*.dump | tail -n +15 | xargs -r rm --
