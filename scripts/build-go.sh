#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
mkdir -p "$DIST"

cd "$ROOT/go-cli"

GOOS=linux GOARCH=amd64 go build -o "$DIST/deploy-shuttle-linux-x64" ./cmd/deploy-shuttle
GOOS=darwin GOARCH=arm64 go build -o "$DIST/deploy-shuttle-darwin-arm64" ./cmd/deploy-shuttle
GOOS=darwin GOARCH=amd64 go build -o "$DIST/deploy-shuttle-darwin-x64" ./cmd/deploy-shuttle

cd "$DIST"
sha256sum deploy-shuttle-* > checksums.txt
