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

COPY backend ./
RUN CGO_ENABLED=0 go build -o /out/moya-dacha-server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM debian:trixie-slim

# curl нужен только healthcheck-у контейнера.
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=build /out/moya-dacha-server /usr/local/bin/moya-dacha-server
COPY --from=build /out/goose /usr/local/bin/goose
COPY demo/migrate.sh /usr/local/bin/migrate.sh

USER 10001:10001

ENTRYPOINT ["moya-dacha-server"]
