# syntax=docker/dockerfile:1
FROM golang:1.23-bookworm AS build
WORKDIR /src
ARG VERSION=dev
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w -buildid= -X main.version=${VERSION}" -o /out/mailpush ./cmd/mailpush
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -buildid= -X main.version=${VERSION}" -o /out/mailpush-linux-amd64 ./cmd/mailpush

FROM debian:bookworm-slim AS package
RUN apt-get update && apt-get install -y --no-install-recommends zip ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /work
COPY --from=build /out/mailpush /work/mailpush.koplugin/bin/mailpush
COPY --from=build /out/mailpush-linux-amd64 /work/host/mailpush-linux-amd64
COPY mailpush.koplugin/_meta.lua mailpush.koplugin/main.lua mailpush.koplugin/config.default.json /work/mailpush.koplugin/
RUN cp /etc/ssl/certs/ca-certificates.crt /work/mailpush.koplugin/cacert.pem
RUN chmod 755 /work/mailpush.koplugin/bin/mailpush && \
    mkdir -p /dist && \
    cd /work && zip -9 -r /dist/mailpush.koplugin.zip mailpush.koplugin >/dev/null && \
    cp /work/host/mailpush-linux-amd64 /dist/

FROM scratch AS export
COPY --from=package /dist/ /
