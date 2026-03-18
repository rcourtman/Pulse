#!/bin/sh

set -eu

usage() {
    cat >&2 <<'EOF'
usage: release_ldflags.sh <server|agent> --version <version> [--build-time <ts>] [--git-commit <sha>] [--license-public-key <base64>]
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
        ;;
    agent)
        ;;
    *)
        usage
        ;;
esac

printf '%s\n' "$ldflags"
