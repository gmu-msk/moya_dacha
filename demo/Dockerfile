# Образ только для локального демо-стенда (demo/README.md).
# В прод он не едет: там один бинарник под systemd и Postgres из пакетов
# (ADR-0004). Никуда не публикуется, собирается на машине разработчика.
#
# В образе два бинарника: сам сервис и goose — чтобы шаг миграций гонял
# ровно ту версию goose, что зафиксирована в backend/go.mod.

FROM golang:1.26 AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

# goose собирается до копирования исходников: ему хватает go.mod и go.sum,
# поэтому слой переживает любую правку в backend/.
#
# Драйверы чужих СУБД выключены тегами — нужен только postgres. Они тянут
# ClickHouse, MySQL, MSSQL, SQLite, Vertica, YDB и libsql и компилируются
# дольше, чем всё остальное в этом образе вместе взятое.
RUN CGO_ENABLED=0 go build \
	-tags 'no_clickhouse no_libsql no_mssql no_azuresql no_mysql no_sqlite3 no_vertica no_ydb' \
	-o /out/goose github.com/pressly/goose/v3/cmd/goose

COPY backend ./
RUN CGO_ENABLED=0 go build -o /out/moya-dacha-server ./cmd/server

FROM debian:trixie-slim

# curl нужен только healthcheck-у контейнера.
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=build /out/moya-dacha-server /usr/local/bin/moya-dacha-server
COPY --from=build /out/goose /usr/local/bin/goose
COPY demo/migrate.sh /usr/local/bin/migrate.sh

# Папка для файлов пользователей (specs/002-profile.md, ADR-0011). Создаём
# её в образе и отдаём тому же пользователю, под которым работает сервис:
# том стенда подхватит владельца отсюда, иначе писать в него будет некому.
RUN mkdir -p /var/lib/moya-dacha/media && chown -R 10001:10001 /var/lib/moya-dacha

USER 10001:10001

ENTRYPOINT ["moya-dacha-server"]
