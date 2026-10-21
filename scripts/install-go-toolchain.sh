#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION_FILE="${SCRIPT_DIR}/.go-version"
DEFAULT_VERSION="go1.26.9"
TARGET_ROOT="/opt/toolchains/go"
DOWNLOAD_ROOT="https://dl.google.com/go"
GOPATH_DIR="/var/lib/pulse/go"
CACHE_DIR="/var/cache/pulse/go-build"
TMP_DIR="/var/cache/pulse/tmp"

VERSION="$DEFAULT_VERSION"
if [[ -f "$VERSION_FILE" ]]; then
  VERSION="$(cat "$VERSION_FILE")"
  VERSION="${VERSION%$'\r'}"
fi
# This value becomes both a URL component and a privileged install path.
# Do not turn a malformed or multiline pin into a different version/path.
if [[ ! "$VERSION" =~ ^go[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid Go toolchain pin: expected goX.Y.Z in scripts/.go-version" >&2
  exit 1
fi

ARCHIVE="${VERSION}.linux-amd64.tar.gz"
# Never consume a cached archive from a predictable, possibly shared directory.
# The private directory also keeps the archive and digest together until use.
DOWNLOAD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulse-go-install.XXXXXXXX")"
ARCHIVE_PATH="$DOWNLOAD_DIR/$ARCHIVE"
SHA_PATH="$ARCHIVE_PATH.sha256"

cleanup_download() {
  local result=$?
  trap - EXIT HUP INT TERM
  if ! rm -rf -- "$DOWNLOAD_DIR"; then
    echo "Could not remove the private Go download directory" >&2
    if [[ "$result" == 0 ]]; then
      result=1
    fi
  fi
  exit "$result"
}
trap cleanup_download EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

download_go_asset() (
  umask 077
  curl --disable --fail --silent --show-error --location \
    --proto '=https' --proto-redir '=https' --noproxy '*' \
    --connect-timeout 5 --max-time "$3" \
    "$DOWNLOAD_ROOT/$1" --output "$2"
)

download_go_asset "$ARCHIVE" "$ARCHIVE_PATH" 300
download_go_asset "$ARCHIVE.sha256" "$SHA_PATH" 60

CHECKSUM="$(cat "$SHA_PATH")"
CHECKSUM="${CHECKSUM%$'\r'}"
if [[ ! "$CHECKSUM" =~ ^[a-fA-F0-9]{64}$ ]]; then
  echo "Invalid Go archive checksum response" >&2
  exit 1
fi
# Hash stdin rather than a filename, so even a TMPDIR with spaces/newlines
# cannot become checksum-file syntax. Both downloads must succeed first.
ACTUAL_CHECKSUM="$(sha256sum < "$ARCHIVE_PATH")"
ACTUAL_CHECKSUM="${ACTUAL_CHECKSUM%% *}"
if [[ "$ACTUAL_CHECKSUM" != "${CHECKSUM,,}" ]]; then
  echo "Go archive checksum mismatch; refusing installation" >&2
  exit 1
fi

sudo mkdir -p "$TARGET_ROOT"
sudo rm -rf "$TARGET_ROOT/$VERSION"
sudo tar -C "$TARGET_ROOT" -xzf "$ARCHIVE_PATH"
sudo mv "$TARGET_ROOT/go" "$TARGET_ROOT/$VERSION"
sudo ln -sfn "$TARGET_ROOT/$VERSION" "$TARGET_ROOT/current"
sudo ln -sfn /opt/toolchains/go/current /usr/local/go

sudo mkdir -p "$GOPATH_DIR" "$GOPATH_DIR/bin" "$GOPATH_DIR/pkg"
sudo chown -R pulse:pulse "$GOPATH_DIR"

sudo mkdir -p "$CACHE_DIR" "$TMP_DIR"
sudo chown -R pulse:pulse "$CACHE_DIR" "$TMP_DIR"

sudo ln -sfn /opt/toolchains/go/current/bin/go /usr/local/bin/go
sudo ln -sfn /opt/toolchains/go/current/bin/gofmt /usr/local/bin/gofmt
