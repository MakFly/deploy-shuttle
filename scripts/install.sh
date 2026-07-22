#!/usr/bin/env sh
# DeployShuttle installer
#
# Detects the current OS / architecture, downloads the latest release binary
# from GitHub, verifies the checksum, and installs it into ~/.local/bin
# (or $SHUTTLE_INSTALL_DIR if set).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
#
# Optional environment variables:
#   SHUTTLE_VERSION       Specific release tag (default: latest)
#   SHUTTLE_INSTALL_DIR   Install directory (default: $HOME/.local/bin)
#   SHUTTLE_INSTALL_SKILLS Install Dockerfile optimization skills (default: 1)

set -eu

REPO="MakFly/deploy-shuttle"
INSTALL_DIR="${SHUTTLE_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${SHUTTLE_VERSION:-latest}"

err() {
  printf 'shuttle install: %s\n' "$*" >&2
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
#   shuttle-linux-x64
#   shuttle-linux-arm64
#   shuttle-darwin-x64
#   shuttle-darwin-arm64
case "$os_tag-$arch_tag" in
  linux-x64|linux-arm64|darwin-x64|darwin-arm64) : ;;
  *) err "no prebuilt binary for $os_tag-$arch_tag" ;;
esac

asset="shuttle-${os_tag}-${arch_tag}"
skill_asset="dockerfile-optimizer.tar.gz"
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

skill_downloaded=0
if [ "${SHUTTLE_INSTALL_SKILLS:-1}" != "0" ]; then
  if $fetcher "$base/$skill_asset" > "$tmpdir/$skill_asset" 2>/dev/null; then
    skill_downloaded=1
    need tar
  else
    rm -f "$tmpdir/$skill_asset"
    printf 'No Dockerfile optimizer skill published for this release; skipping skill installation.\n' >&2
  fi
fi

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
  if [ "$skill_downloaded" -eq 1 ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      skill_expected="$(awk -v asset="$skill_asset" '$2 == asset { print $1 }' "$tmpdir/$checksums")"
      skill_actual="$(sha256sum "$tmpdir/$skill_asset" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      skill_expected="$(awk -v asset="$skill_asset" '$2 == asset { print $1 }' "$tmpdir/$checksums")"
      skill_actual="$(shasum -a 256 "$tmpdir/$skill_asset" | awk '{print $1}')"
    else
      skill_expected=""
      skill_actual=""
    fi
    if [ -z "$skill_expected" ]; then
      err "missing checksum for $skill_asset"
    fi
    if [ "$skill_expected" != "$skill_actual" ]; then
      err "skill checksum mismatch (expected $skill_expected, got $skill_actual)"
    fi
  fi
else
  printf 'No checksums.txt published; skipping checksum verification.\n' >&2
fi

mkdir -p "$INSTALL_DIR"
mv "$tmpdir/$asset" "$INSTALL_DIR/shuttle"
chmod +x "$INSTALL_DIR/shuttle"

printf '\nInstalled: %s/shuttle\n' "$INSTALL_DIR"

if [ "$skill_downloaded" -eq 1 ]; then
  codex_skills_dir="${CODEX_HOME:-$HOME/.codex}/skills"
  claude_skills_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills"

  mkdir -p "$codex_skills_dir" "$claude_skills_dir"
  tar -xzf "$tmpdir/$skill_asset" -C "$codex_skills_dir"
  tar -xzf "$tmpdir/$skill_asset" -C "$claude_skills_dir"
  chmod +x "$codex_skills_dir/dockerfile-optimizer/scripts/audit-image.sh"
  chmod +x "$claude_skills_dir/dockerfile-optimizer/scripts/audit-image.sh"

  printf 'Installed skill: %s/dockerfile-optimizer\n' "$codex_skills_dir"
  printf 'Installed skill: %s/dockerfile-optimizer\n' "$claude_skills_dir"
fi

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf '\nNote: %s is not on your $PATH.\n' "$INSTALL_DIR"
    printf 'Add this to your shell profile:\n'
    printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac

printf '\nRun: shuttle doctor --help\n'
