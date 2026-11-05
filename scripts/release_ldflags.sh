#!/bin/sh

set -eu

usage() {
    cat >&2 <<'EOF'
usage: release_ldflags.sh <server|agent> --version <version> [--build-time <ts>] [--git-commit <sha>] [--license-public-key <base64>] [--update-public-keys <csv>]
EOF
    exit 1
}

if [ "$#" -lt 1 ]; then
    usage
fi

mode="$1"
shift

version=""
build_time="unknown"
git_commit="unknown"
license_public_key=""
update_public_keys=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        --version)
            [ "$#" -ge 2 ] || usage
            version="$2"
            shift 2
            ;;
        --build-time)
            [ "$#" -ge 2 ] || usage
            build_time="$2"
            shift 2
            ;;
        --git-commit)
            [ "$#" -ge 2 ] || usage
            git_commit="$2"
            shift 2
            ;;
        --license-public-key)
            [ "$#" -ge 2 ] || usage
            license_public_key="$2"
            shift 2
            ;;
        --update-public-keys)
            [ "$#" -ge 2 ] || usage
            update_public_keys="$2"
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

if [ -z "$version" ]; then
    usage
fi

case "$version" in
    v*)
        normalized_version="$version"
        ;;
    *)
        normalized_version="v$version"
        ;;
esac

ldflags="-s -w -X main.Version=${normalized_version}"

case "$mode" in
    server)
        ldflags="${ldflags} -X main.BuildTime=${build_time}"
        ldflags="${ldflags} -X main.GitCommit=${git_commit}"
        ldflags="${ldflags} -X github.com/rcourtman/pulse-go-rewrite/internal/updates.BuildVersion=${normalized_version}"
        ldflags="${ldflags} -X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=${normalized_version}"
        if [ -n "$license_public_key" ]; then
            ldflags="${ldflags} -X github.com/rcourtman/pulse-go-rewrite/pkg/licensing.EmbeddedPublicKey=${license_public_key}"
            ldflags="${ldflags} -X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=${license_public_key}"
        fi
        if [ -n "$update_public_keys" ]; then
            ldflags="${ldflags} -X github.com/rcourtman/pulse-go-rewrite/internal/updatesignature.EmbeddedTrustedPublicKeys=${update_public_keys}"
        fi
        ;;
    agent)
        # Canonical build flags require one bounded version token. The grammar
        # is the same embedded source consumed by agentidentity.Version.
        case "$normalized_version" in
            *[!0-9A-Za-z.+-]*) echo "invalid agent release version: expected one semantic version token" >&2; exit 1 ;;
        esac
        if [ "${#normalized_version}" -gt 128 ]; then
            echo "invalid agent release version: exceeds identity limit" >&2
            exit 1
        fi
        release_script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
        release_version_file="$release_script_dir/../internal/agentidentity/version-pattern.txt"
        if [ ! -s "$release_version_file" ]; then
            echo "missing agent version grammar" >&2
            exit 1
        fi
        release_version_pattern=$(tr -d '\r\n' < "$release_version_file")
        if [ -z "$release_version_pattern" ]; then
            echo "empty agent version grammar" >&2
            exit 1
        fi
        if ! printf '%s\n' "$normalized_version" | LC_ALL=C grep -Eq "$release_version_pattern"; then
            echo "invalid agent release version: expected a semantic version" >&2
            exit 1
        fi
        ldflags="${ldflags} -X github.com/rcourtman/pulse-go-rewrite/internal/agentidentity.ReleaseIdentity=[[pulse-agent-release:v1:${normalized_version}]]"
        if [ -n "$update_public_keys" ]; then
            ldflags="${ldflags} -X github.com/rcourtman/pulse-go-rewrite/internal/updatesignature.EmbeddedTrustedPublicKeys=${update_public_keys}"
        fi
        ;;
    *)
        usage
        ;;
esac

printf '%s\n' "$ldflags"
