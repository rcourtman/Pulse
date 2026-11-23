#!/usr/bin/env bash
#
# install-cli.sh - Install the Pulse CLI (`pulse`)
#
# Detects the local platform/architecture, downloads the matching
# pulse-cli binary from a GitHub Release, verifies the signed checksum
# manifest against Pulse's pinned release key, verifies the binary's
# SHA256 digest, and installs it as `pulse` on PATH.
#
# Verify this script before running it (see docs/AGENT_SUBSTRATE.md):
#   PULSE_CLI_VERSION=vX.Y.Z bash ./install-cli.sh
#
# Options (env vars):
#   PULSE_CLI_VERSION   Override the release tag to install (e.g. "v6.9.0").
#                       Default: latest.
#   PULSE_CLI_BIN_DIR   Where to install the `pulse` binary.
#                       Default: $HOME/.local/bin if writable, else /usr/local/bin.
#   PULSE_CLI_REPO      GitHub repo to download from. Default: rcourtman/Pulse.
#
# The `pulse` CLI provides direct terminal reads (`pulse fleet`, `pulse changes`,
# `pulse metrics`, `pulse resource`, `pulse events`), governed actions, keychain-
# backed login (`pulse auth login`), automatic MCP client wiring (`pulse connect`),
# diagnostics (`pulse doctor`), and the built-in stdio MCP server (`pulse mcp`).

set -euo pipefail

usage() {
    cat <<'EOF'
Usage: install-cli.sh [--help | -h]

Install the Pulse CLI (`pulse`) for this platform after verifying its signed
checksum manifest and binary digest. Run without arguments to install.

Configure installation with environment variables:
  PULSE_CLI_VERSION  Release tag to install (default: latest stable release).
  PULSE_CLI_BIN_DIR  Destination directory (default: $HOME/.local/bin when
                     writable, otherwise /usr/local/bin).
  PULSE_CLI_REPO     GitHub repository (default: rcourtman/Pulse).

--help and -h show this message without preparing or changing an installation.
Other arguments are not supported. No installation is attempted if supplied.
EOF
}

# Inspect arguments before platform/destination discovery or any download.
# A help request or a misspelled option must never replace an existing binary.
if [ "$#" -gt 0 ]; then
    if [ "$#" -eq 1 ] && { [ "$1" = --help ] || [ "$1" = -h ]; }; then
        usage
        exit 0
    fi
    printf '[install-cli] error: unsupported arguments; no installation attempted\n' >&2
    usage >&2
    exit 2
fi

REPO="${PULSE_CLI_REPO:-rcourtman/Pulse}"
VERSION="${PULSE_CLI_VERSION:-latest}"
SIGNATURE_IDENTITY="pulse-installer"
SIGNATURE_NAMESPACE="pulse-install"
PINNED_RELEASE_SSH_PUBLIC_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
tmp=""
checksums_tmp=""
signature_tmp=""
allowed_signers=""
install_stage=""

log() {
    printf '[install-cli] %s\n' "$*"
}

err() {
    printf '[install-cli] error: %s\n' "$*" >&2
    exit 1
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

# Use the same verified HTTPS transport for the binary and both trust files.
# A stalled peer must fail before publication, not hang a CLI replacement
# indefinitely. Keep transient retries, but bound both each transfer and the
# retry window. First-option -q ignores curlrc without requiring curl 7.50's
# --disable spelling; implicit config cannot disable TLS or these limits.
download_asset() {
    curl -q -fsSL --connect-timeout 5 --max-time 60 \
        --retry 3 --retry-max-time 120 \
        --proto '=https' --proto-redir '=https' "$1" -o "$2"
}

cleanup() {
    local path
    for path in "${tmp}" "${checksums_tmp}" "${signature_tmp}" "${allowed_signers}"; do
        [ -z "${path}" ] || rm -f -- "${path}"
    done
    if [ -n "${install_stage}" ]; then
        rm -f -- "${install_stage}/pulse"
        rmdir "${install_stage}" 2>/dev/null || true
    fi
}

check_destination() {
    if [ -L "$1" ] || { [ -e "$1" ] && [ ! -f "$1" ]; }; then
        err "install destination is not a regular file: $1; left unchanged"
    fi
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
            err "Windows shells must use install-cli.ps1 instead. Download and verify install-cli.ps1 from https://github.com/${REPO}/releases before running it in PowerShell (see docs/AGENT_SUBSTRATE.md)."
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
            err "no published pulse-cli binary for ${os}-${arch}; build from source via go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-cli@latest"
            ;;
    esac

    printf '%s-%s' "$os" "$arch"
}

choose_install_dir() {
    if [ -n "${PULSE_CLI_BIN_DIR:-}" ]; then
        printf '%s' "${PULSE_CLI_BIN_DIR}"
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
    require_cmd mv
    require_cmd mkdir
    require_cmd mktemp
    require_cmd ssh-keygen

    local platform install_dir destination base bin_name url checksums_url
    local signature_url expected actual matches
    platform="$(detect_platform)"
    install_dir="$(choose_install_dir)"
    destination="${install_dir}/pulse"
    check_destination "${destination}"
    # A custom bin directory can be new, just like the default ~/.local/bin.
    # Confirm that its same-filesystem stage can be created before downloading
    # anything. mkdir -p preserves existing directory permissions and contents.
    if ! mkdir -p -- "${install_dir}"; then
        err "could not create the install directory; existing binary left unchanged"
    fi
    trap cleanup EXIT
    if ! install_stage="$(mktemp -d "${install_dir}/.pulse-cli.XXXXXX")"; then
        err "could not prepare the install directory; existing binary left unchanged"
    fi
    base="$(resolve_release_base)"
    bin_name="pulse-cli-${platform}"
    url="${base}/${bin_name}"

    log "platform: ${platform}"
    log "install dir: ${install_dir}"
    log "downloading: ${url}"

    tmp="$(mktemp -t pulse-cli.XXXXXX)"

    if ! download_asset "${url}" "${tmp}"; then
        err "download failed: ${url}
If a release exists for this version, the binary may not yet be published for ${platform}.
Build from source: go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-cli@latest"
    fi

    local sha_cmd
    if command -v sha256sum >/dev/null 2>&1; then
        sha_cmd="sha256sum"
    elif command -v sha256 >/dev/null 2>&1; then
        # FreeBSD base ships sha256(1) rather than GNU sha256sum.
        sha_cmd="sha256 -q"
    elif command -v shasum >/dev/null 2>&1; then
        sha_cmd="shasum -a 256"
    else
        err "no sha256 tool found (sha256sum, sha256 or shasum); refusing unverified install"
    fi

    checksums_url="${base}/checksums.txt"
    signature_url="${checksums_url}.sshsig"
    checksums_tmp="$(mktemp -t pulse-cli-sums.XXXXXX)"
    signature_tmp="$(mktemp -t pulse-cli-signature.XXXXXX)"
    allowed_signers="$(mktemp -t pulse-cli-signers.XXXXXX)"

    if ! download_asset "${checksums_url}" "${checksums_tmp}"; then
        err "could not fetch checksums.txt; refusing unverified install"
    fi
    if ! download_asset "${signature_url}" "${signature_tmp}"; then
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

    # A failed copy must not truncate or remove a working binary. Stage on
    # the destination filesystem, verify the copied bytes, then publish with
    # a single rename. This is not a concurrent-writer or power-loss guarantee.
    if ! install -m 0755 "${tmp}" "${install_stage}/pulse"; then
        err "could not stage the binary; existing binary left unchanged"
    fi
    actual="$(${sha_cmd} "${install_stage}/pulse" | awk '{print tolower($1)}')"
    if [ "${expected}" != "${actual}" ]; then
        err "staged binary checksum mismatch; existing binary left unchanged"
    fi
    check_destination "${destination}"
    if ! mv -f "${install_stage}/pulse" "${destination}"; then
        err "could not publish the binary; check the existing installation"
    fi
    if [ -e "${install_stage}/pulse" ] || [ -L "${install_stage}/pulse" ] ||
        [ -L "${destination}" ] || [ ! -f "${destination}" ]; then
        err "could not confirm binary publication; installation is incomplete"
    fi
    log "installed: ${destination}"

    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *)
            log "note: ${install_dir} is not on PATH. Add this line to your shell profile:
  export PATH=\"${install_dir}:\$PATH\""
            ;;
    esac

    log ""
    log "next steps:"
    log "  1. Authenticate with a read-only monitoring:read token (stored in your OS keychain):"
    log "     pulse auth login https://your-pulse-host"
    log "  2. Connect your AI agent (claude-code, codex, opencode, or claude-desktop):"
    log "     pulse connect claude-code"
    log "  3. Verify connectivity:"
    log "     pulse doctor"
}

main "$@"
