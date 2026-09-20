# Цикл разработки описан в README. Коротко:
#   спека -> контракт -> make generate -> тесты -> реализация -> make test -> PR

BACKEND        := backend
MOBILE         := mobile
GENERATED      := $(BACKEND)/api/gen $(MOBILE)/packages/moya_dacha_api
OPENAPI_GENERATOR_VERSION := 7.25.0
OPENAPI_GENERATOR_JAR     := .cache/openapi-generator-cli-$(OPENAPI_GENERATOR_VERSION).jar
TEST_DATABASE_URL ?= postgres://moya_dacha:moya_dacha@127.0.0.1:55432/moya_dacha_test?sslmode=disable
DEMO_COMPOSE   := docker compose -f docker-compose.demo.yml

.PHONY: help generate generate-server generate-client check-generated build test test-up test-down \
	fmt vet migrate-up migrate-status app-get app-analyze app-apk apk-phone apk-phone-install \
	demo demo-lan demo-down demo-reset demo-logs demo-psql demo-e2e \
	stories story check-stories ci

help:
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

generate: generate-server generate-client ## Сгенерировать код из specs/openapi.yaml (результат коммитится)

generate-server: ## Серверные типы и роутинг (Go)
	cd $(BACKEND) && go tool oapi-codegen -config oapi-codegen.yaml ../specs/openapi.yaml

generate-client: $(OPENAPI_GENERATOR_JAR) ## Клиент приложения (Dart)
	cd $(MOBILE) && java -jar ../$(OPENAPI_GENERATOR_JAR) generate -c openapi-generator.yaml

# Генератор Dart-клиента — jar фиксированной версии; лежит вне git (ADR-0008).
$(OPENAPI_GENERATOR_JAR):
	@mkdir -p $(dir $@)
	curl -fsSL -o $@ https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/$(OPENAPI_GENERATOR_VERSION)/openapi-generator-cli-$(OPENAPI_GENERATOR_VERSION).jar

check-generated: generate ## Упасть, если сгенерированное разошлось с контрактом
	@test -z "$$(git status --porcelain -- $(GENERATED))" \
		|| (echo ""; echo "Сгенерированный код разошёлся с specs/openapi.yaml:"; \
		    git status --short -- $(GENERATED); \
		    echo "Выполните 'make generate' и закоммитьте результат."; exit 1)

build: ## Собрать бинарник
	cd $(BACKEND) && go build -o ../bin/server ./cmd/server

fmt: ## Форматирование
	cd $(BACKEND) && gofmt -w .

vet: ## Статические проверки
	cd $(BACKEND) && go vet ./...

test-up: ## Поднять Postgres для тестов
	docker compose -f docker-compose.test.yml up -d --wait

test-down: ## Остановить Postgres для тестов
	docker compose -f docker-compose.test.yml down -v

test: test-up ## Интеграционные тесты против настоящей Postgres
	cd $(BACKEND) && DATABASE_URL="$(TEST_DATABASE_URL)" go test ./... -count=1

migrate-up: ## Накатить миграции (шаг деплоя; DATABASE_URL обязателен)
	cd $(BACKEND) && go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-status: ## Показать состояние миграций
	cd $(BACKEND) && go tool goose -dir migrations postgres "$$DATABASE_URL" status

app-get: ## Зависимости приложения
	cd $(MOBILE) && flutter pub get

app-analyze: ## Статический анализ приложения
	cd $(MOBILE) && flutter analyze

app-apk: ## Собрать debug-APK приложения (адрес стенда — как у эмулятора)
	cd $(MOBILE) && flutter build apk --debug

apk-phone: ## Собрать APK для телефона: адрес стенда в локальной сети
	./demo/apk-phone.sh

apk-phone-install: ## То же и сразу поставить на телефон по USB
	./demo/apk-phone.sh --install

demo: ## Поднять демо-стенд одной командой (Postgres + миграции + сервис)
	$(DEMO_COMPOSE) up -d --build
	./demo/smoke.sh

demo-lan: ## Поднять стенд видимым в локальной сети (чтобы дошёл телефон)
	DEMO_BIND_ADDR=0.0.0.0 $(DEMO_COMPOSE) up -d --build
	DEMO_BIND_ADDR=0.0.0.0 ./demo/smoke.sh

demo-down: ## Остановить демо-стенд (данные в базе остаются)
	$(DEMO_COMPOSE) down

demo-reset: ## Остановить демо-стенд и стереть его базу
	$(DEMO_COMPOSE) down -v

demo-logs: ## Логи сервиса на демо-стенде
	$(DEMO_COMPOSE) logs -f api

demo-psql: ## Консоль psql в базе демо-стенда
	$(DEMO_COMPOSE) exec postgres psql -U moya_dacha -d moya_dacha

demo-e2e: ## Прогнать приложение на подключённом эмуляторе против стенда
	./demo/e2e.sh

stories: ## Список сценариев показа user-story
	@./demo/story.sh

story: ## Показать один сценарий в эмуляторе: make story STORY=000-status
	@test -n "$(STORY)" || (./demo/story.sh; echo; echo "Укажите сценарий: make story STORY=<сценарий>"; exit 2)
	./demo/story.sh "$(STORY)"

check-stories: ## Проверить форму сценариев показа
	./demo/check-stories.sh

ci: check-generated vet test build check-stories ## То же, что гоняет CI
