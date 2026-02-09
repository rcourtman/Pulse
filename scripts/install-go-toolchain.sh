#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION_FILE="${SCRIPT_DIR}/.go-version"
DEFAULT_VERSION="go1.25.7"
TARGET_ROOT="/opt/toolchains/go"
DOWNLOAD_ROOT="https://dl.google.com/go"
GOPATH_DIR="/var/lib/pulse/go"
CACHE_DIR="/var/cache/pulse/go-build"
TMP_DIR="/var/cache/pulse/tmp"

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "$DEFAULT_VERSION" | sudo tee "$VERSION_FILE" >/dev/null
fi
VERSION="$(tr -d '
' < "$VERSION_FILE")"
ARCHIVE="${VERSION}.linux-amd64.tar.gz"
DOWNLOAD_DIR="${TMPDIR:-/tmp}/go-install"
ARCHIVE_PATH="$DOWNLOAD_DIR/$ARCHIVE"
SHA_PATH="$ARCHIVE_PATH.sha256"

mkdir -p "$DOWNLOAD_DIR"

if [[ ! -f "$ARCHIVE_PATH" ]]; then
  curl -fsSL "$DOWNLOAD_ROOT/$ARCHIVE" -o "$ARCHIVE_PATH"
fi
curl -fsSL "$DOWNLOAD_ROOT/$ARCHIVE.sha256" -o "$SHA_PATH"

CHECKSUM="$(tr -d '
' < "$SHA_PATH")"
printf '%s  %s
' "$CHECKSUM" "$ARCHIVE_PATH" | sha256sum -c -

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
