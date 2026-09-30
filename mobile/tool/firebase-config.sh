#!/usr/bin/env bash
# Положить настройки Firebase в сборку APK (specs/024-push.md,
# требование 17): google-services.json из секрета GOOGLE_SERVICES_JSON.
# Gradle берёт из него строки для ресурсов (android/app/build.gradle.kts).
# Секрета нет — файла нет, и приложение собирается без пушей.
set -euo pipefail
cd "$(dirname "$0")/.."

target=android/app/google-services.json
if [ -z "${GOOGLE_SERVICES_JSON:-}" ]; then
	rm -f "$target"
	echo "Пуши: секрета GOOGLE_SERVICES_JSON нет, APK без пушей" >&2
	exit 0
fi
printf '%s' "$GOOGLE_SERVICES_JSON" > "$target"
# Сломанный JSON лучше увидеть здесь, а не посреди сборки Gradle.
jq -e '.project_info.project_id' "$target" >/dev/null ||
	{ echo "GOOGLE_SERVICES_JSON не похож на google-services.json" >&2; exit 1; }
echo "Пуши: настройки Firebase в сборке" >&2
