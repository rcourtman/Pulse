#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: run-release-preflight.sh [options]

Options:
  --sha SHA             Exact pushed commit (default: HEAD)
  --profile PROFILE     rehearsal or release (required)
  --host HOST           SSH host (or PULSE_RELEASE_PREFLIGHT_HOST / git config)
  --wsl-distro NAME     Execute in this WSL distribution on the SSH host
  --if-configured       Succeed without running when no host is configured
  --plan                Print the resolved execution plan without connecting
  -h, --help            Show this help

Persistent repository configuration:
  git config pulse.releasePreflightHost <ssh-alias>
  git config pulse.releasePreflightWslDistro <distribution>
EOF
}

SOURCE_REF="HEAD"
PROFILE=""
HOST="${PULSE_RELEASE_PREFLIGHT_HOST:-}"
WSL_DISTRO="${PULSE_RELEASE_PREFLIGHT_WSL_DISTRO:-}"
IF_CONFIGURED=false
PLAN_ONLY=false

while [ "$#" -gt 0 ]; do
  case "$1" in
    --sha)
      SOURCE_REF="${2:-}"
      shift 2
      ;;
    --profile)
      PROFILE="${2:-}"
      shift 2
      ;;
    --host)
      HOST="${2:-}"
      shift 2
      ;;
    --wsl-distro)
      WSL_DISTRO="${2:-}"
      shift 2
      ;;
    --if-configured)
      IF_CONFIGURED=true
      shift
      ;;
    --plan)
      PLAN_ONLY=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ "$PROFILE" != "rehearsal" ] && [ "$PROFILE" != "release" ]; then
  echo "Error: --profile must be rehearsal or release." >&2
  exit 2
fi

REPOSITORY_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPOSITORY_ROOT"
SOURCE_SHA="$(git rev-parse --verify "${SOURCE_REF}^{commit}")"
if [[ ! "$SOURCE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Error: could not resolve an exact source SHA from ${SOURCE_REF}." >&2
  exit 2
fi

if [ -z "$HOST" ]; then
  HOST="$(git config --get pulse.releasePreflightHost || true)"
fi
if [ -z "$WSL_DISTRO" ]; then
  WSL_DISTRO="$(git config --get pulse.releasePreflightWslDistro || true)"
fi
if [ -n "$WSL_DISTRO" ] && [[ ! "$WSL_DISTRO" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "Error: WSL distribution names may contain only letters, digits, dot, underscore, and hyphen." >&2
  exit 2
fi
if [ -z "$HOST" ]; then
  if [ "$IF_CONFIGURED" = true ]; then
    echo "No accelerated release-preflight worker is configured; continuing with canonical hosted checks."
    exit 0
  fi
  echo "Error: configure --host, PULSE_RELEASE_PREFLIGHT_HOST, or pulse.releasePreflightHost." >&2
  exit 2
fi

echo "Accelerated exact-SHA release preflight"
echo "  Host:    ${HOST}"
echo "  SHA:     ${SOURCE_SHA}"
echo "  Profile: ${PROFILE}"
if [ -n "$WSL_DISTRO" ]; then
  echo "  Runtime: WSL ${WSL_DISTRO}"
else
  echo "  Runtime: native remote shell"
fi

if [ "$PLAN_ONLY" = true ]; then
  exit 0
fi

git fetch origin --quiet
if ! git for-each-ref --format='%(refname)' --contains "$SOURCE_SHA" refs/remotes/origin/ | grep -q .; then
  echo "Error: ${SOURCE_SHA} is not reachable from a fetched origin branch." >&2
  exit 2
fi

if ! git cat-file -e "${SOURCE_SHA}:scripts/release-preflight-worker.sh" 2>/dev/null; then
  echo "Error: ${SOURCE_SHA} does not contain scripts/release-preflight-worker.sh." >&2
  exit 2
fi

if [ -n "$WSL_DISTRO" ]; then
  git show "${SOURCE_SHA}:scripts/release-preflight-worker.sh" | \
    ssh "$HOST" wsl.exe -d "$WSL_DISTRO" -- bash -s -- "$SOURCE_SHA" "$PROFILE"
else
  git show "${SOURCE_SHA}:scripts/release-preflight-worker.sh" | \
    ssh "$HOST" bash -s -- "$SOURCE_SHA" "$PROFILE"
fi
