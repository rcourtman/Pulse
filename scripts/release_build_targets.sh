#!/usr/bin/env bash

# Canonical cross-compilation target matrices shared by release compilation and
# packaging. Keep target names aligned with published asset names.

PULSE_RELEASE_AGENT_TARGETS=(
    linux-amd64
    linux-arm64
    linux-armv7
    linux-armv6
    linux-386
    darwin-amd64
    darwin-arm64
    freebsd-amd64
    freebsd-arm64
    windows-amd64
    windows-arm64
    windows-386
)

PULSE_RELEASE_AGENT_HELPER_TARGETS=(
    linux-amd64
    linux-arm64
    linux-armv7
    linux-armv6
    linux-386
)

PULSE_RELEASE_AGENT_RUNNER_TARGETS=(
    linux-amd64
    linux-arm64
    linux-armv7
    linux-armv6
    linux-386
)

PULSE_RELEASE_SERVER_TARGETS=(
    linux-amd64
    linux-arm64
    linux-armv7
    linux-armv6
    linux-386
)

PULSE_RELEASE_CONTROL_PLANE_TARGETS=(
    linux-amd64
    linux-arm64
)

pulse_release_target_env() {
    case "$1" in
        linux-amd64) printf '%s\n' 'GOOS=linux GOARCH=amd64' ;;
        linux-arm64) printf '%s\n' 'GOOS=linux GOARCH=arm64' ;;
        linux-armv7) printf '%s\n' 'GOOS=linux GOARCH=arm GOARM=7' ;;
        linux-armv6) printf '%s\n' 'GOOS=linux GOARCH=arm GOARM=6' ;;
        linux-386) printf '%s\n' 'GOOS=linux GOARCH=386' ;;
        darwin-amd64) printf '%s\n' 'GOOS=darwin GOARCH=amd64' ;;
        darwin-arm64) printf '%s\n' 'GOOS=darwin GOARCH=arm64' ;;
        freebsd-amd64) printf '%s\n' 'GOOS=freebsd GOARCH=amd64' ;;
        freebsd-arm64) printf '%s\n' 'GOOS=freebsd GOARCH=arm64' ;;
        windows-amd64) printf '%s\n' 'GOOS=windows GOARCH=amd64' ;;
        windows-arm64) printf '%s\n' 'GOOS=windows GOARCH=arm64' ;;
        windows-386) printf '%s\n' 'GOOS=windows GOARCH=386' ;;
        *)
            echo "Error: unsupported release target: $1" >&2
            return 1
            ;;
    esac
}

pulse_release_binary_filename() {
    local component="$1"
    local target="$2"
    local filename

    case "${component}" in
        agent) filename="pulse-agent-${target}" ;;
        agent-helper) filename="pulse-agent-helper-${target}" ;;
        agent-runner) filename="pulse-agent-runner-${target}" ;;
        mcp) filename="pulse-mcp-${target}" ;;
        server) filename="pulse-${target}" ;;
        control-plane) filename="pulse-control-plane-${target}" ;;
        *)
            echo "Error: unsupported release component: ${component}" >&2
            return 1
            ;;
    esac
    if [[ "${target}" == windows-* ]]; then
        filename="${filename}.exe"
    fi
    printf '%s\n' "${filename}"
}
