#!/bin/sh
set -eu
rm -rf dist
mkdir -p dist
VERSION="${VERSION:-dev}"
docker build --build-arg "VERSION=$VERSION" --target package -t mailpush-koreader-package .
cid="$(docker create mailpush-koreader-package)"
trap 'docker rm -f "$cid" >/dev/null 2>&1 || true' EXIT
docker cp "$cid:/dist/." ./dist/
printf 'Built: dist/mailpush.koplugin.zip (version %s)\n' "$VERSION"
