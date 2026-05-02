#!/usr/bin/env sh
# DeployShuttle installer
#
# Detects the current OS / architecture, downloads the latest release binary
# from GitHub, verifies the checksum, and installs it into ~/.local/bin
# (or $DEPLOY_SHUTTLE_INSTALL_DIR if set).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
#
# Optional environment variables:
#   DEPLOY_SHUTTLE_VERSION       Specific release tag (default: latest)
#   DEPLOY_SHUTTLE_INSTALL_DIR   Install directory (default: $HOME/.local/bin)

set -eu

REPO="MakFly/deploy-shuttle"
INSTALL_DIR="${DEPLOY_SHUTTLE_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${DEPLOY_SHUTTLE_VERSION:-latest}"

err() {
  printf 'deploy-shuttle install: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

need uname
need mkdir
need chmod
if command -v curl >/dev/null 2>&1; then
  fetcher="curl -fsSL"
elif command -v wget >/dev/null 2>&1; then
  fetcher="wget -qO-"
else
  err "need either curl or wget"
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  linux)  os_tag=linux ;;
  darwin) os_tag=darwin ;;
  *) err "unsupported OS: $os" ;;
esac

case "$arch" in
  x86_64|amd64) arch_tag=x64 ;;
  arm64|aarch64) arch_tag=arm64 ;;
  *) err "unsupported architecture: $arch" ;;
esac

# Available release artifacts (see scripts/build-go.sh):
#   deploy-shuttle-linux-x64
#   deploy-shuttle-darwin-x64
#   deploy-shuttle-darwin-arm64
case "$os_tag-$arch_tag" in
  linux-x64|darwin-x64|darwin-arm64) : ;;
  linux-arm64)
    err "linux-arm64 binary is not built yet; build from source: 'cd go-cli && go install ./cmd/deploy-shuttle'"
    ;;
  *) err "no prebuilt binary for $os_tag-$arch_tag" ;;
esac

asset="deploy-shuttle-${os_tag}-${arch_tag}"
checksums="checksums.txt"

if [ "$VERSION" = "latest" ]; then
  base="https://github.com/${REPO}/releases/latest/download"
else
  base="https://github.com/${REPO}/releases/download/${VERSION}"
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

printf 'Downloading %s ...\n' "$asset"
$fetcher "$base/$asset" > "$tmpdir/$asset" || err "download failed: $base/$asset"

if $fetcher "$base/$checksums" > "$tmpdir/$checksums" 2>/dev/null && [ -s "$tmpdir/$checksums" ]; then
  if command -v sha256sum >/dev/null 2>&1; then
    expected="$(grep " $asset\$" "$tmpdir/$checksums" | awk '{print $1}')"
    actual="$(sha256sum "$tmpdir/$asset" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    expected="$(grep " $asset\$" "$tmpdir/$checksums" | awk '{print $1}')"
    actual="$(shasum -a 256 "$tmpdir/$asset" | awk '{print $1}')"
  else
    expected=""
    actual=""
    printf 'No sha256sum/shasum available; skipping checksum verification.\n' >&2
  fi
  if [ -n "$expected" ] && [ "$expected" != "$actual" ]; then
    err "checksum mismatch (expected $expected, got $actual)"
  fi
else
  printf 'No checksums.txt published; skipping checksum verification.\n' >&2
fi

mkdir -p "$INSTALL_DIR"
mv "$tmpdir/$asset" "$INSTALL_DIR/deploy-shuttle"
chmod +x "$INSTALL_DIR/deploy-shuttle"

printf '\nInstalled: %s/deploy-shuttle\n' "$INSTALL_DIR"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf '\nNote: %s is not on your $PATH.\n' "$INSTALL_DIR"
    printf 'Add this to your shell profile:\n'
    printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac

printf '\nRun: deploy-shuttle doctor --help\n'
