#!/usr/bin/env bash
#
# install-mcp.sh - Install the Pulse MCP server adapter
#
# Detects the local platform/architecture, downloads the matching
# pulse-mcp binary from the latest GitHub Release, verifies the
# signed checksum manifest against Pulse's pinned release key, verifies the
# binary's SHA256 digest, and places the binary on PATH.
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
#
# After install, configure your MCP client per `cmd/pulse-mcp/README.md` in the
# Pulse repository (or `docs/AGENT_SUBSTRATE.md` in your installed Pulse server).
#
# pulse-mcp is the stdio JSON-RPC adapter that wraps Pulse's agent
# surface for Claude Desktop, Claude Code, and other MCP clients.

set -euo pipefail

REPO="${PULSE_MCP_REPO:-rcourtman/Pulse}"
VERSION="${PULSE_MCP_VERSION:-latest}"
SIGNATURE_IDENTITY="pulse-installer"
SIGNATURE_NAMESPACE="pulse-install"
PINNED_RELEASE_SSH_PUBLIC_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
tmp=""
checksums_tmp=""
signature_tmp=""
allowed_signers=""

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
    require_cmd ssh-keygen

    local platform install_dir base bin_name url checksums_url
    local signature_url expected actual matches
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

    local sha_cmd
    if command -v sha256sum >/dev/null 2>&1; then
        sha_cmd="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        sha_cmd="shasum -a 256"
    else
        err "no sha256 tool found (sha256sum or shasum); refusing unverified install"
    fi

    checksums_url="${base}/checksums.txt"
    signature_url="${checksums_url}.sshsig"
    checksums_tmp="$(mktemp -t pulse-mcp-sums.XXXXXX)"
    signature_tmp="$(mktemp -t pulse-mcp-signature.XXXXXX)"
    allowed_signers="$(mktemp -t pulse-mcp-signers.XXXXXX)"
    trap 'rm -f "${tmp}" "${checksums_tmp}" "${signature_tmp}" "${allowed_signers}"' EXIT

    if ! curl -fsSL --retry 3 "${checksums_url}" -o "${checksums_tmp}"; then
        err "could not fetch checksums.txt; refusing unverified install"
    fi
    if ! curl -fsSL --retry 3 "${signature_url}" -o "${signature_tmp}"; then
        err "could not fetch checksums.txt.sshsig; refusing unverified install"
    fi

    printf '%s %s\n' "${SIGNATURE_IDENTITY}" "${PINNED_RELEASE_SSH_PUBLIC_KEY}" > "${allowed_signers}"
    if ! ssh-keygen -Y verify \
        -f "${allowed_signers}" \
        -I "${SIGNATURE_IDENTITY}" \
        -n "${SIGNATURE_NAMESPACE}" \
        -s "${signature_tmp}" < "${checksums_tmp}" >/dev/null 2>&1; then
        err "cryptographic signature verification failed for checksums.txt"
    fi
    log "release signature verified"

    matches="$(awk -v name="${bin_name}" '$2 == name && NF == 2 {count++; checksum=$1} END {if (count == 1) print checksum; else exit 1}' "${checksums_tmp}" || true)"
    expected="$(printf '%s' "${matches}" | tr '[:upper:]' '[:lower:]')"
    if ! printf '%s' "${expected}" | grep -Eq '^[0-9a-f]{64}$'; then
        err "checksums.txt must contain exactly one valid SHA256 entry for ${bin_name}"
    fi

    actual="$(${sha_cmd} "${tmp}" | awk '{print tolower($1)}')"
    if [ "${expected}" != "${actual}" ]; then
        err "sha256 mismatch for ${bin_name}: expected ${expected}, got ${actual}"
    fi
    log "sha256 verified"

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
