#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
mkdir -p "$DIST"

cd "$ROOT/go-cli"

# Version: prefer env var, then `git describe`, else "dev".
VERSION="${SHUTTLE_VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
fi

# License public key (base64). Empty = local dev build, license gates are no-ops.
LICENSE_PUBKEY_B64="${LICENSE_PUBKEY_B64:-}"

# License server URL. Empty = use compile-time default.
LICENSE_SERVER="${LICENSE_SERVER:-}"

LDPKG="github.com/MakFly/deploy-shuttle/go-cli/internal/version"
LDFLAGS="-s -w -X ${LDPKG}.Version=${VERSION}"
if [ -n "$LICENSE_PUBKEY_B64" ]; then
  LDFLAGS="$LDFLAGS -X ${LDPKG}.LicensePubKeyB64=${LICENSE_PUBKEY_B64}"
fi
if [ -n "$LICENSE_SERVER" ]; then
  LDFLAGS="$LDFLAGS -X ${LDPKG}.LicenseServer=${LICENSE_SERVER}"
fi

build() {
  goos="$1"
  goarch="$2"
  out="$3"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" -o "$DIST/$out" ./cmd/shuttle
}

build linux  amd64 shuttle-linux-x64
build linux  arm64 shuttle-linux-arm64
build darwin arm64 shuttle-darwin-arm64
build darwin amd64 shuttle-darwin-x64

cd "$DIST"
sha256sum shuttle-* > checksums.txt
