#!/usr/bin/env bash

# Package (and optionally push) the Pulse Helm chart.
# Usage:
#   ./scripts/package-helm-chart.sh [version] [--push]
# Environment:
#   OCI_REPO (default: ghcr.io/rcourtman/pulse-chart)
#   HELM_BIN (default: helm)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="$REPO_ROOT/deploy/helm/pulse"
DIST_DIR="$REPO_ROOT/dist"
HELM_BIN="${HELM_BIN:-helm}"
OCI_REPO="${OCI_REPO:-ghcr.io/rcourtman/pulse-chart}"

if ! command -v "$HELM_BIN" >/dev/null 2>&1; then
  echo "Error: Helm not found (expected at \$HELM_BIN=$HELM_BIN). Install Helm 3.9+ first." >&2
  exit 1
fi

VERSION_DEFAULT="$(cat "$REPO_ROOT/VERSION")"
VERSION="${1:-$VERSION_DEFAULT}"
VERSION="${VERSION#v}" # strip leading v if provided

PUSH=false
for arg in "$@"; do
  if [ "$arg" = "--push" ]; then
    PUSH=true
  fi
done

echo "Packaging Pulse chart version $VERSION (appVersion=$VERSION)"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

"$HELM_BIN" lint "$CHART_DIR" --strict
"$HELM_BIN" package "$CHART_DIR" \
  --version "$VERSION" \
  --app-version "$VERSION" \
  --destination "$DIST_DIR"

PACKAGE_PATH="$DIST_DIR/pulse-$VERSION.tgz"
if [ ! -f "$PACKAGE_PATH" ]; then
  echo "Error: Expected package $PACKAGE_PATH not found" >&2
  exit 1
fi

if [ "$PUSH" = true ]; then
  echo "Pushing chart to oci://$OCI_REPO"
  "$HELM_BIN" push "$PACKAGE_PATH" "oci://$OCI_REPO"
fi

echo "Chart packaged at $PACKAGE_PATH"
if [ "$PUSH" = true ]; then
  echo "Chart pushed to oci://$OCI_REPO"
else
  echo "Run with --push (after logging into GHCR) to upload the artifact."
fi
