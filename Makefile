# Цикл разработки описан в README. Коротко:
#   спека -> контракт -> make generate -> тесты -> реализация -> make test -> PR

BACKEND        := backend
TEST_DATABASE_URL ?= postgres://moya_dacha:moya_dacha@127.0.0.1:55432/moya_dacha_test?sslmode=disable

.PHONY: help generate check-generated build test test-up test-down fmt vet migrate-up migrate-status ci

help:
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

generate: ## Сгенерировать код из specs/openapi.yaml (результат коммитится)
	cd $(BACKEND) && go tool oapi-codegen -config oapi-codegen.yaml ../specs/openapi.yaml

check-generated: generate ## Упасть, если сгенерированное разошлось с контрактом
	@test -z "$$(git status --porcelain -- $(BACKEND)/api/gen)" \
		|| (echo ""; echo "Сгенерированный код разошёлся с specs/openapi.yaml:"; \
		    git status --short -- $(BACKEND)/api/gen; \
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

ci: check-generated vet test build ## То же, что гоняет CI
