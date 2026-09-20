#!/bin/sh
# Наполнение демо-стенда данными: применяет demo/seed/*.sql по порядку имён.
# Пока файлов нет, шаг ничего не делает и завершается успешно.
set -eu

if ! ls /seed/*.sql >/dev/null 2>&1; then
	echo "Демо-данных нет: в demo/seed нет ни одного .sql — пропускаю шаг."
	exit 0
fi

for file in /seed/*.sql; do
	echo "-> $file"
	psql -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -f "$file"
done
