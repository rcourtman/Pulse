#!/usr/bin/env bash
#
# install-mcp.sh - Install the Pulse MCP server adapter
#
# Detects the local platform/architecture, downloads the matching
# pulse-mcp binary from the latest GitHub Release, verifies the
# SHA256 checksum against the published checksums file, and places
# the binary on PATH.
#
# Usage:
#   curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh | bash
#
# Options (env vars):
#   PULSE_MCP_VERSION   Override the version to install (e.g. "v6.0.0-rc.5").
#                       Default: latest.
#   PULSE_MCP_BIN_DIR   Where to install the binary.
#                       Default: $HOME/.local/bin if writable, else /usr/local/bin.
#   PULSE_MCP_REPO      GitHub repo to download from. Default: rcourtman/Pulse.
#   PULSE_MCP_NO_VERIFY If "1", skip SHA256 verification (not recommended).
#
# After install, configure your MCP client per `cmd/pulse-mcp/README.md` in the
# Pulse repository (or `docs/AGENT_SUBSTRATE.md` in your installed Pulse server).
#
# pulse-mcp is the stdio JSON-RPC adapter that wraps Pulse's agent
# surface for Claude Desktop, Claude Code, and other MCP clients.

set -euo pipefail

REPO="${PULSE_MCP_REPO:-rcourtman/Pulse}"
VERSION="${PULSE_MCP_VERSION:-latest}"
NO_VERIFY="${PULSE_MCP_NO_VERIFY:-}"

log() {
    printf '[install-mcp] %s\n' "$*"
}

err() {
    printf '[install-mcp] error: %s\n' "$*" >&2
    exit 1
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux)   os="linux" ;;
        darwin)  os="darwin" ;;
        freebsd) os="freebsd" ;;
        mingw*|msys*|cygwin*)
            err "Windows shells must use install-mcp.ps1 instead. Run via PowerShell:
  iwr https://github.com/${REPO}/releases/latest/download/install-mcp.ps1 -UseBasicParsing | iex"
            ;;
        *)
            err "unsupported OS: $os"
            ;;
    esac

    case "$arch" in
        x86_64|amd64)         arch="amd64" ;;
        aarch64|arm64)        arch="arm64" ;;
        armv7l|armv7)         arch="armv7" ;;
        armv6l|armv6)         arch="armv6" ;;
        i386|i686)            arch="386" ;;
        *)
            err "unsupported architecture: $arch"
            ;;
    esac

    # Trim impossible combinations so we fail with a clear message
    # rather than a confusing 404 from the GitHub asset endpoint.
    case "${os}-${arch}" in
        darwin-amd64|darwin-arm64) ;;
        linux-amd64|linux-arm64|linux-armv7|linux-armv6|linux-386) ;;
        freebsd-amd64|freebsd-arm64) ;;
        *)
            err "no published pulse-mcp binary for ${os}-${arch}; build from source via go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-mcp@latest"
            ;;
    esac

    printf '%s-%s' "$os" "$arch"
}

choose_install_dir() {
    if [ -n "${PULSE_MCP_BIN_DIR:-}" ]; then
        printf '%s' "${PULSE_MCP_BIN_DIR}"
        return
    fi
    local home_bin="${HOME}/.local/bin"
    if [ -d "${home_bin}" ] && [ -w "${home_bin}" ]; then
        printf '%s' "${home_bin}"
        return
    fi
    if mkdir -p "${home_bin}" 2>/dev/null && [ -w "${home_bin}" ]; then
        printf '%s' "${home_bin}"
        return
    fi
    printf '%s' "/usr/local/bin"
}

resolve_release_base() {
    if [ "${VERSION}" = "latest" ]; then
        printf 'https://github.com/%s/releases/latest/download' "${REPO}"
    else
        printf 'https://github.com/%s/releases/download/%s' "${REPO}" "${VERSION}"
    fi
}

main() {
    require_cmd curl
    require_cmd uname
    require_cmd install

    local platform install_dir base bin_name url tmp checksums_url checksums_tmp
    platform="$(detect_platform)"
    install_dir="$(choose_install_dir)"
    base="$(resolve_release_base)"
    bin_name="pulse-mcp-${platform}"
    url="${base}/${bin_name}"

    log "platform: ${platform}"
    log "install dir: ${install_dir}"
    log "downloading: ${url}"

    tmp="$(mktemp -t pulse-mcp.XXXXXX)"
    trap 'rm -f "${tmp}"' EXIT

    if ! curl -fsSL --retry 3 "${url}" -o "${tmp}"; then
        err "download failed: ${url}
If a release exists for this version, the binary may not yet be published for ${platform}.
Build from source: go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-mcp@latest"
    fi

    if [ "${NO_VERIFY}" != "1" ]; then
        local sha_cmd
        if command -v sha256sum >/dev/null 2>&1; then
            sha_cmd="sha256sum"
        elif command -v shasum >/dev/null 2>&1; then
            sha_cmd="shasum -a 256"
        else
            err "no sha256 tool found (sha256sum or shasum). Set PULSE_MCP_NO_VERIFY=1 to skip verification."
        fi

        checksums_url="${base}/checksums.txt"
        checksums_tmp="$(mktemp -t pulse-mcp-sums.XXXXXX)"
        trap 'rm -f "${tmp}" "${checksums_tmp}"' EXIT
        if ! curl -fsSL --retry 3 "${checksums_url}" -o "${checksums_tmp}"; then
            log "warning: could not fetch checksums.txt; skipping verification"
        else
            local expected actual
            expected="$(awk -v name="${bin_name}" '$2 == name {print $1; exit}' "${checksums_tmp}" || true)"
            if [ -z "${expected}" ]; then
                log "warning: ${bin_name} not listed in checksums.txt; skipping verification"
            else
                actual="$(${sha_cmd} "${tmp}" | awk '{print $1}')"
                if [ "${expected}" != "${actual}" ]; then
                    err "sha256 mismatch for ${bin_name}: expected ${expected}, got ${actual}"
                fi
                log "sha256 verified"
            fi
        fi
    fi

    install -m 0755 "${tmp}" "${install_dir}/pulse-mcp"
    log "installed: ${install_dir}/pulse-mcp"

    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *)
            log "note: ${install_dir} is not on PATH. Add this line to your shell profile:
  export PATH=\"${install_dir}:\$PATH\""
            ;;
    esac

    log ""
    log "next steps:"
    log "  1. Mint a Pulse API token in Settings -> API Access (with monitoring:read,"
    log "     and monitoring:write if you want the operator-state write tools)."
    log "  2. Wire pulse-mcp into your MCP client per the cmd/pulse-mcp/README.md"
    log "     in the Pulse repository (or docs/AGENT_SUBSTRATE.md in your Pulse install)."
}

main "$@"
