#!/usr/bin/env bash
#
# Release lifecycle rehearsal: install, upgrade and roll back Pulse between two
# PUBLISHED releases on a real systemd host. Default: shadow systemd container.
# --hosted-browser: fresh public GitHub VM, authenticated Tailscale Serve UI
# update/SSE, documented CLI recovery and browser/session readback.
#
#   1. install FROM with that release's signature-verified install.sh --version
#   2. assert FROM identity, health and unit state
#   3. seed persistent state through the real API (first-run security setup,
#      a webhook, a PVE node with fake credentials) and snapshot the data dir
#   4. upgrade with the installed updater users run: /bin/update --version TO
#   5. assert TO identity, health, unit state, seeded settings and data dir
#   6. roll back with the documented command: /bin/update --version FROM
#   7. assert FROM identity, health, unit state, seeded settings and data dir
#
# The result is a phase table on stdout and in $GITHUB_STEP_SUMMARY (when set).
# Nothing is uploaded. Exit status is non-zero when any phase fails.
#
# Usage:
#   scripts/release_lifecycle_rehearsal.sh --from vX.Y.Z --to vX.Y.Z
#
# Environment:
#   PULSE_REHEARSAL_REPO     owner/repo for release assets (default rcourtman/Pulse)
#   PULSE_REHEARSAL_ENGINE   docker (default) or podman
#   PULSE_REHEARSAL_WORKDIR  scratch directory (default: mktemp -d)
#   PULSE_REHEARSAL_README   README holding the pinned installer key
#                            (default: README.md next to this script's repo)

# Programs passed to cexec are single-quoted on purpose: they expand inside
# the container from -e variables. Mount options legitimately contain commas.
# shellcheck disable=SC2016,SC2054

set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYSTEMD_IMAGE="docker.io/jrei/systemd-debian:12@sha256:61d70dc3e574337bd9df794674a60ae73113460fff16ab41a2d234b4a11dcd98"
TAG_PATTERN='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
API="http://127.0.0.1:7655"
DATA_DIR="/etc/pulse"
SEED_WEBHOOK_NAME="lifecycle-rehearsal-webhook"
SEED_WEBHOOK_URL="https://example.com/pulse-lifecycle-rehearsal"
# PVE add-node skips live cluster detection for 192.168.77.x hosts and
# "test-" names, so the fake node is persisted without a reachable endpoint.
SEED_NODE_NAME="test-lifecycle-rehearsal"
SEED_NODE_HOST="https://192.168.77.10:8006"
SEED_NODE_TOKEN_NAME="root@pam!rehearsal"
SEED_WEBHOOK_HEADER="X-Rehearsal-Secret"
SEED_ADMIN_USER="rehearsal-admin"

FROM_TAG=""
TO_TAG=""
HOSTED_BROWSER=false
CHECK_SNAPSHOT=""
COMPARE_DATADIR=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --hosted-browser) HOSTED_BROWSER=true; shift ;;
        --from) FROM_TAG="${2:-}"; shift 2 ;;
        --to) TO_TAG="${2:-}"; shift 2 ;;
        # Offline self-tests used by the contract tests: apply the fixed
        # settings expectations to a snapshot file, or compare two data-dir
        # manifests (path<TAB>sha256), then exit.
        --check-snapshot) CHECK_SNAPSHOT="${2:-}"; shift 2 ;;
        --compare-datadir) COMPARE_DATADIR=("${2:-}" "${3:-}"); shift 3 ;;
        -h|--help) sed -n '2,/^$/p' "${BASH_SOURCE[0]}"; exit 0 ;;
        *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done

if [[ -z "$CHECK_SNAPSHOT" && ${#COMPARE_DATADIR[@]} -eq 0 ]]; then
    for tag in "$FROM_TAG" "$TO_TAG"; do
        if [[ ! "$tag" =~ $TAG_PATTERN ]]; then
            echo "::error::--from and --to must be Pulse release tags (vX.Y.Z[-pre]); got '${tag}'" >&2
            exit 2
        fi
    done
    if [[ "$FROM_TAG" == "$TO_TAG" ]]; then
        echo "::error::--from and --to must differ; a same-version run proves no upgrade" >&2
        exit 2
    fi
fi

# The browser route is a real first-level public CI host, not a container
# overriding Docker detection. It must never install onto a maintainer host.
if [[ "$HOSTED_BROWSER" == true ]]; then
    if [[ "$EUID" != 0 || "${GITHUB_ACTIONS:-}" != true \
        || "${RUNNER_ENVIRONMENT:-}" != github-hosted \
        || "${GITHUB_REPOSITORY:-}" != rcourtman/Pulse \
        || "${GITHUB_REF:-}" != refs/heads/main \
        || -e /opt/pulse || -e /etc/pulse \
        || -e /etc/systemd/system/pulse.service || -e /rehearsal ]]; then
        echo "::error::Browser lifecycle requires a fresh disposable public hosted runner."
        exit 2
    fi
    if [[ ! -f "${PULSE_REHEARSAL_PACKET_MANIFEST:-}" ]]; then
        echo "::error::A verified immutable release asset manifest is required."
        exit 2
    fi
    # The workflow derives this from the new runner's own Tailscale identity.
    # No caller endpoint, old stopped target or TLS-verification bypass exists.
    if [[ ! "${PULSE_REHEARSAL_BROWSER_ORIGIN:-}" =~ ^https://[a-z0-9-]+\.tawny-powan\.ts\.net$ ]]; then
        echo "::error::Browser lifecycle requires the ephemeral runner's HTTPS Serve origin."
        exit 2
    fi
fi

REPO="${PULSE_REHEARSAL_REPO:-rcourtman/Pulse}"
if [[ ! "$REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "::error::invalid PULSE_REHEARSAL_REPO '${REPO}'" >&2
    exit 2
fi
ENGINE="${PULSE_REHEARSAL_ENGINE:-docker}"
case "$ENGINE" in
    docker|podman) ;;
    *) echo "::error::PULSE_REHEARSAL_ENGINE must be docker or podman" >&2; exit 2 ;;
esac
README_PATH="${PULSE_REHEARSAL_README:-${ROOT_DIR}/README.md}"
WORK_DIR="${PULSE_REHEARSAL_WORKDIR:-$(mktemp -d)}"
mkdir -p "${WORK_DIR}/assets" "${WORK_DIR}/state"
CONTAINER="pulse-lifecycle-rehearsal-$$"
SUMMARY_FILE="${GITHUB_STEP_SUMMARY:-}"

# Throwaway credentials for an ephemeral container. They never leave it.
ADMIN_PASSWORD="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
API_TOKEN="$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')"
NODE_TOKEN_VALUE="$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"

PHASE_ROWS=()
FAILURES=()
CURRENT_FAILURE=""
OVERALL_STATUS=0
CONTAINER_STARTED=false
BASELINE_FAILED_UNITS=""
# D-Bus-activated host services that cannot run inside the test container
# (no hostname/clock/locale ownership). Any other new failed unit, and every
# pulse* unit, fails the phase.
CONTAINER_TOLERATED_UNITS="systemd-hostnamed.service systemd-timedated.service systemd-localed.service"

log() { printf '[rehearsal] %s\n' "$*"; }
note_failure() {
    CURRENT_FAILURE="${CURRENT_FAILURE:+${CURRENT_FAILURE}; }$*"
    echo "::error::$*"
}

# Run a bash program inside the container. Data reaches the program only as
# environment variables, never by interpolation into the program text.
cexec() {
    local program="$1"
    shift
    local -a env_args=(-e PULSE_INSTALL_ALLOW_DOCKER=1)
    local pair
    for pair in "$@"; do
        env_args+=(-e "$pair")
    done
    if [[ "$HOSTED_BROWSER" == true ]]; then
        # Never pass workflow, GitHub or tailnet-registration credentials to
        # the installed application or an acceptance command.
        env -i PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin LANG=C.UTF-8 \
            "${@}" bash -c "$program"
    else
        "$ENGINE" exec "${env_args[@]}" "$CONTAINER" bash -c "$program"
    fi
}

print_diagnostics() {
    [[ "$CONTAINER_STARTED" == "true" ]] || return 0
    if [[ "$HOSTED_BROWSER" == true ]]; then
        echo "::error::Hosted lifecycle failed; inspect bounded browser/identity receipts. No raw credentials or journal retained."
        return
    fi
    echo "::group::Diagnostics: pulse journal (tail)"
    cexec 'journalctl -u pulse --no-pager | tail -n 150' || true
    echo "::endgroup::"
    echo "::group::Diagnostics: unit state"
    cexec 'systemctl status pulse --no-pager; systemctl list-units --state=failed --no-pager; systemctl list-timers --all --no-pager | grep -i pulse' || true
    echo "::endgroup::"
    echo "::group::Diagnostics: pulse-update journal"
    cexec 'journalctl -u pulse-update --no-pager | tail -n 60' || true
    echo "::endgroup::"
    local logfile
    for logfile in "${WORK_DIR}"/state/*.log; do
        [[ -f "$logfile" ]] || continue
        echo "::group::Diagnostics: $(basename "$logfile") (tail)"
        tail -n 80 "$logfile" || true
        echo "::endgroup::"
    done
}

cleanup() {
    local status=$?
    if [[ "$OVERALL_STATUS" -ne 0 || "$status" -ne 0 ]]; then
        print_diagnostics
    fi
    if [[ "$CONTAINER_STARTED" == "true" ]]; then
        if [[ "$HOSTED_BROWSER" == true ]]; then
            local cleanup_status=0
            systemctl stop pulse.service || cleanup_status=1
            if [[ "$(systemctl show pulse-update.timer --property=LoadState --value)" != not-found ]]; then
                systemctl stop pulse-update.timer || cleanup_status=1
            fi
            if systemctl is-active --quiet pulse.service; then cleanup_status=1; fi
            if systemctl is-active --quiet pulse-update.timer; then cleanup_status=1; fi
            printf '{"cleanup_exit":%s,"host_disposal":"github-hosted-job"}\n' "$cleanup_status" > "${WORK_DIR}/state/browser-cleanup.json"
            [[ "$cleanup_status" == 0 ]] || status=1
        else
            "$ENGINE" rm -f "$CONTAINER" >/dev/null 2>&1 || true
        fi
    fi
    if [[ "$HOSTED_BROWSER" == true ]]; then
        # Ephemeral application credentials are never retained as evidence.
        if [[ -f "${WORK_DIR}/state/browser-auth.json" ]]; then
            : > "${WORK_DIR}/state/browser-auth.json"
        fi
    fi
    trap - EXIT
    exit "$status"
}
trap cleanup EXIT

write_summary() {
    local table
    table=$(
        echo "## Release lifecycle rehearsal"
        echo
        if [[ "$HOSTED_BROWSER" == true ]]; then
            echo "\`${FROM_TAG}\` -> \`${TO_TAG}\` -> \`${FROM_TAG}\` on a fresh public hosted VM with HTTPS Tailscale Serve. Browser acceptance evidence, not a stable promotion decision."
        else
            echo "\`${FROM_TAG}\` -> \`${TO_TAG}\` -> \`${FROM_TAG}\` on a systemd host (${SYSTEMD_IMAGE%%@*}). Shadow run, not a release gate."
        fi
        echo
        echo "| Phase | Expected | Reported version | Health | Settings | Data dir | Units | Result |"
        echo "| --- | --- | --- | --- | --- | --- | --- | --- |"
        local row
        for row in "${PHASE_ROWS[@]}"; do
            echo "$row"
        done
        if [[ ${#FAILURES[@]} -gt 0 ]]; then
            echo
            echo "### Failures"
            local failure
            for failure in "${FAILURES[@]}"; do
                echo "- ${failure}"
            done
        fi
    )
    if [[ "$HOSTED_BROWSER" == true ]]; then
        jq -cn --argjson overall_status "$OVERALL_STATUS" \
            --argjson phases "$(printf '%s\n' "${PHASE_ROWS[@]}" | jq -Rsc 'split("\n") | map(select(length > 0))')" \
            '{schema_version:1, overall_status:$overall_status, phases:$phases}' \
            > "${WORK_DIR}/state/lifecycle-result.json"
    fi
    echo
    echo "$table"
    if [[ -n "$SUMMARY_FILE" ]]; then
        printf '%s\n' "$table" >> "$SUMMARY_FILE"
    fi
}

# record_phase NAME EXPECTED VERSION HEALTH SETTINGS DATADIR UNITS
record_phase() {
    local result="pass"
    if [[ -n "$CURRENT_FAILURE" ]]; then
        result="FAIL"
        FAILURES+=("$1: ${CURRENT_FAILURE}")
        OVERALL_STATUS=1
    fi
    PHASE_ROWS+=("| $1 | \`$2\` | ${3:--} | ${4:--} | ${5:--} | ${6:--} | ${7:--} | ${result} |")
    CURRENT_FAILURE=""
    [[ "$result" == "pass" ]]
}

abort_run() {
    write_summary
    exit 1
}

# ---------------------------------------------------------------------------
# Published installer, verified against the README-pinned key
# ---------------------------------------------------------------------------

fetch_and_verify_installer() {
    local tag="$1"
    local dest="${WORK_DIR}/assets"
    local base="https://github.com/${REPO}/releases/download/${tag}"
    local asset
    local -a retry_args=(--retry 5 --retry-delay 5 --retry-all-errors)
    if [[ "$HOSTED_BROWSER" == true ]]; then retry_args=(--retry 0 --max-time 120); fi
    for asset in install.sh install.sh.sshsig; do
        if ! curl -fsSL "${retry_args[@]}" \
                -o "${dest}/${asset}" "${base}/${asset}"; then
            echo "::error::could not download ${base}/${asset}"
            return 1
        fi
    done

    local readme_key allowed_signers
    readme_key=$(grep -oE 'ssh-ed25519 [A-Za-z0-9+/=]+ pulse-installer' "$README_PATH" | head -1)
    if [[ -z "$readme_key" ]]; then
        echo "::error::Could not extract the pulse-installer key from ${README_PATH}"
        return 1
    fi
    allowed_signers="${WORK_DIR}/allowed_signers"
    printf 'pulse-installer %s\n' "$readme_key" > "$allowed_signers"
    if ! ssh-keygen -Y verify \
            -f "$allowed_signers" \
            -I pulse-installer \
            -n pulse-install \
            -s "${dest}/install.sh.sshsig" < "${dest}/install.sh"; then
        echo "::error::${tag} install.sh.sshsig does not verify against the README's pinned key"
        return 1
    fi
    if ! grep -qE '^# Pulse Installer Script' "${dest}/install.sh" \
        || grep -q 'Pulse Unified Agent Installer' "${dest}/install.sh" \
        || ! grep -qE '^[[:space:]]*--version\)' "${dest}/install.sh"; then
        echo "::error::${tag} install.sh is not the Pulse server installer with --version support"
        return 1
    fi
    if [[ "$HOSTED_BROWSER" == true ]]; then
        jq -cn --arg tag "$tag" \
            --arg installer_sha256 "$(sha256sum "${dest}/install.sh" | cut -d' ' -f1)" \
            --arg signature_sha256 "$(sha256sum "${dest}/install.sh.sshsig" | cut -d' ' -f1)" \
            '{tag:$tag,installer_sha256:$installer_sha256,signature_sha256:$signature_sha256,signature_verified:true}' \
            > "${WORK_DIR}/state/installer.json"
    fi
    log "${tag} install.sh signature verifies against the README-pinned key"
}

# ---------------------------------------------------------------------------
# Container
# ---------------------------------------------------------------------------

start_container() {
    if [[ "$HOSTED_BROWSER" == true ]]; then
        CONTAINER_STARTED=true
        BASELINE_FAILED_UNITS=$(cexec 'systemctl list-units --state=failed --no-legend --plain | awk "{print \$1}" | sort')
        # Keep the same telemetry opt-out, without changing deployment identity,
        # enabling Docker updates, mocking feeds or weakening authentication.
        cexec 'mkdir -p /etc/systemd/system/pulse.service.d && printf "[Service]\nEnvironment=PULSE_TELEMETRY=false\n" > /etc/systemd/system/pulse.service.d/50-rehearsal-no-telemetry.conf'
        return
    fi
    local -a run_args=(run -d --name "$CONTAINER" --privileged
        -v "${WORK_DIR}/assets:/rehearsal:ro")
    if [[ "$ENGINE" == "docker" ]]; then
        # GHA ubuntu-24.04 uses the cgroup v2 unified hierarchy; without
        # --cgroupns=host systemd PID 1 cannot mount the cgroup tree.
        run_args+=(--cgroupns=host -v /sys/fs/cgroup:/sys/fs/cgroup:rw
            --tmpfs /tmp:rw,size=2g --tmpfs /run --tmpfs /run/lock)
    else
        # Podman's systemd mode mounts /run, /run/lock and the cgroup tree.
        # The pinned image is amd64-only.
        run_args+=(--systemd=always --arch amd64 --tmpfs /tmp:rw,size=2g)
    fi
    CONTAINER_STARTED=true
    if ! "$ENGINE" "${run_args[@]}" "$SYSTEMD_IMAGE" >/dev/null; then
        echo "::error::${ENGINE} could not start ${SYSTEMD_IMAGE}"
        return 1
    fi

    local i state
    for i in $(seq 1 45); do
        if ! "$ENGINE" inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null | grep -q true; then
            "$ENGINE" logs "$CONTAINER" || true
            echo "::error::systemd container exited during boot"
            return 1
        fi
        state=$("$ENGINE" exec "$CONTAINER" systemctl is-system-running 2>/dev/null || true)
        if [[ "$state" == "running" || "$state" == "degraded" ]]; then
            break
        fi
        if [[ "$i" -eq 45 ]]; then
            echo "::error::systemd did not become ready inside the container (state: ${state:-unknown})"
            return 1
        fi
        sleep 2
    done
    BASELINE_FAILED_UNITS=$(cexec 'systemctl list-units --state=failed --no-legend --plain | awk "{print \$1}" | sort')
    log "systemd is up (pre-existing failed units: ${BASELINE_FAILED_UNITS:-none})"
    if ! cexec 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq >/dev/null && apt-get install -y -qq curl ca-certificates jq >/dev/null'; then
        echo "::error::could not install curl/jq inside the container"
        return 1
    fi
    # Keep a daily CI install out of real usage telemetry. A unit drop-in is
    # outside the installer's ownership and the data dir, so it does not
    # change what install, upgrade or rollback do.
    cexec 'mkdir -p /etc/systemd/system/pulse.service.d && printf "[Service]\nEnvironment=PULSE_TELEMETRY=false\n" > /etc/systemd/system/pulse.service.d/50-rehearsal-no-telemetry.conf'
}

# ---------------------------------------------------------------------------
# Assertions
# ---------------------------------------------------------------------------

PHASE_VERSION=""
PHASE_HEALTH=""
PHASE_SETTINGS=""
PHASE_DATADIR=""
PHASE_UNITS=""

reset_phase_cells() {
    PHASE_VERSION="-"; PHASE_HEALTH="-"; PHASE_SETTINGS="-"; PHASE_DATADIR="-"; PHASE_UNITS="-"
}

# Identity, health and systemd state for the expected release.
assert_runtime() {
    local expected_tag="$1"
    local expected="${expected_tag#v}"

    local active="" i
    for i in $(seq 1 60); do
        active=$(cexec 'systemctl is-active pulse' 2>/dev/null || true)
        [[ "$active" == "active" ]] && break
        sleep 2
    done

    local health_body health_status
    health_body=$(cexec 'curl -fsS --retry 30 --retry-delay 2 --retry-connrefused --retry-all-errors "$API/api/health"' "API=${API}" 2>/dev/null || true)
    health_status=$(jq -r '.status // empty' <<<"$health_body" 2>/dev/null || true)
    if [[ "$health_status" == "healthy" ]]; then
        PHASE_HEALTH="healthy"
    else
        PHASE_HEALTH="${health_status:-no response}"
        note_failure "/api/health did not report healthy (got '${PHASE_HEALTH}')"
    fi

    local version_body reported binary_version
    version_body=$(cexec 'curl -fsS "$API/api/version"' "API=${API}" 2>/dev/null || true)
    reported=$(jq -r '.version // empty' <<<"$version_body" 2>/dev/null || true)
    PHASE_VERSION="${reported:-none}"
    if [[ "${reported#v}" != "$expected" ]]; then
        note_failure "/api/version reported '${reported:-none}', expected ${expected_tag}"
    fi
    binary_version=$(cexec '/opt/pulse/bin/pulse --version 2>/dev/null | grep -oE "v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?" | head -1' 2>/dev/null || true)
    if [[ "${binary_version#v}" != "$expected" ]]; then
        note_failure "/opt/pulse/bin/pulse --version reported '${binary_version:-none}', expected ${expected_tag}"
    fi

    if [[ "$HOSTED_BROWSER" == true ]]; then
        local observed_digest expected_digest
        observed_digest=$(cexec 'sha256sum /opt/pulse/bin/pulse | cut -d" " -f1')
        expected_digest=$(jq -er --arg tag "$expected_tag" '.releases[] | select(.tag == $tag) | .binary_sha256' "$PULSE_REHEARSAL_PACKET_MANIFEST")
        printf '{"tag":"%s","binary_sha256":"%s","expected_binary_sha256":"%s"}\n' \
            "$expected_tag" "$observed_digest" "$expected_digest" > "${WORK_DIR}/state/installed-${expected_tag}.json"
        [[ "$observed_digest" == "$expected_digest" ]] || note_failure "Installed binary differs from the verified published archive"
    fi

    local enabled failed_now new_failed
    enabled=$(cexec 'systemctl is-enabled pulse' 2>/dev/null || true)
    failed_now=$(cexec 'systemctl list-units --state=failed --no-legend --plain | awk "{print \$1}" | sort' 2>/dev/null || true)
    new_failed=$(comm -13 <(printf '%s\n' "$BASELINE_FAILED_UNITS" | sed '/^$/d') \
                          <(printf '%s\n' "$failed_now" | sed '/^$/d') | tr '\n' ' ')
    PHASE_UNITS="pulse ${active:-unknown}/${enabled:-unknown}"
    if [[ "$active" != "active" ]]; then
        note_failure "pulse.service is '${active:-unknown}', expected active"
    fi
    if [[ "$enabled" != "enabled" ]]; then
        note_failure "pulse.service is '${enabled:-unknown}', expected enabled"
    fi
    local unit tolerated="" unexpected=""
    for unit in $new_failed; do
        if [[ " ${CONTAINER_TOLERATED_UNITS} " == *" ${unit} "* ]]; then
            tolerated="${tolerated:+${tolerated} }${unit}"
        else
            unexpected="${unexpected:+${unexpected} }${unit}"
        fi
    done
    if [[ -n "$unexpected" ]]; then
        PHASE_UNITS="${PHASE_UNITS}, failed: ${unexpected}"
        note_failure "systemd units failed during the rehearsal: ${unexpected}"
    fi
    if [[ -n "$tolerated" ]]; then
        PHASE_UNITS="${PHASE_UNITS}, container-only failures ignored: ${tolerated}"
        echo "::warning::container-environment units failed and were not counted: ${tolerated}"
    fi

    if ! cexec 'systemctl show -p Environment --value pulse | grep -q "PULSE_TELEMETRY=false"' 2>/dev/null; then
        echo "::warning::the rehearsal telemetry opt-out drop-in is no longer applied to pulse.service"
    fi

    # An unattended update firing mid-rehearsal would invalidate the version
    # assertions, so prove the timer-driven updater never started.
    local auto_update_started
    auto_update_started=$(cexec 'systemctl show -p ExecMainStartTimestampMonotonic --value pulse-update.service 2>/dev/null || echo 0' 2>/dev/null || echo 0)
    if [[ -n "$auto_update_started" && "$auto_update_started" != "0" ]]; then
        note_failure "pulse-update.service ran during the rehearsal; results are not attributable to the manual updater"
    fi
}

# Only bounded, non-secret intent fields leave the disposable install. A failed
# read is not an absent timer or a disabled setting.
auto_update_snapshot() {
    cexec 'set -euo pipefail
config=absent
if [[ -e "$DATA_DIR/system.json" ]]; then
    config=$(jq -er '\''if type != "object" then error("invalid system settings")
        elif has("autoUpdateEnabled") then
            if .autoUpdateEnabled == true then "enabled"
            elif .autoUpdateEnabled == false then "disabled"
            else error("invalid auto-update choice") end
        else "absent" end'\'' "$DATA_DIR/system.json")
fi
load=$(systemctl show -p LoadState --value pulse-update.timer)
case "$load" in
    not-found) enabled=absent; active=absent ;;
    loaded)
        enabled=$(systemctl is-enabled pulse-update.timer || true)
        active=$(systemctl is-active pulse-update.timer || true)
        case "$enabled" in
            enabled|enabled-runtime|disabled|masked|masked-runtime|static|indirect|linked|linked-runtime|generated|transient) ;;
            *) exit 1 ;;
        esac
        case "$active" in
            active|inactive|failed|activating|deactivating|reloading) ;;
            *) exit 1 ;;
        esac ;;
    *) exit 1 ;;
esac
printf "%s\t%s\t%s\n" "$config" "$enabled" "$active"' "DATA_DIR=${DATA_DIR}"
}

check_auto_update_intent() {
    local label="$1" current before_config before_enabled before_active config enabled active
    if [[ ! -s "${WORK_DIR}/state/auto-updates.baseline.tsv" ]] \
            || ! current=$(auto_update_snapshot); then
        note_failure "${label}: auto-update intent could not be read or has no baseline"
        return 1
    fi
    IFS=$'\t' read -r before_config before_enabled before_active < "${WORK_DIR}/state/auto-updates.baseline.tsv"
    IFS=$'\t' read -r config enabled active <<< "$current"
    # A newly persisted false is equivalent to an absent opt-in, not permission
    # to enable the timer. Preserve enabled and masked timer choices exactly.
    if [[ "$config" != "$before_config" \
            && ! ( "$config" != enabled && "$before_config" != enabled ) ]] \
            || [[ "$enabled" != "$before_enabled" || "$active" != "$before_active" ]]; then
        note_failure "${label}: auto-update choice or timer enablement/activity changed"
        return 1
    fi
    printf '%s\n' "$current" > "${WORK_DIR}/state/auto-updates.${label}.tsv"
    PHASE_UNITS="${PHASE_UNITS}; auto-updates ${config}/${enabled}/${active} preserved"
}

api_status() {
    # api_status METHOD PATH [auth: none|token|basic] [body]
    cexec 'args=(-sS -o /dev/null -w "%{http_code}" -X "$METHOD")
case "$AUTH" in
  token) args+=(-H "X-API-Token: $TOKEN") ;;
  basic) args+=(-u "$BASIC") ;;
esac
curl "${args[@]}" "$API$REQ_PATH"' \
        "API=${API}" "METHOD=$1" "REQ_PATH=$2" "AUTH=${3:-none}" \
        "TOKEN=${API_TOKEN}" "BASIC=${SEED_ADMIN_USER}:${ADMIN_PASSWORD}"
}

api_json() {
    # api_json METHOD PATH [body]; authenticates with the seeded API token.
    cexec 'if [[ -n "$BODY" ]]; then
  curl -fsS -X "$METHOD" -H "X-API-Token: $TOKEN" -H "Content-Type: application/json" --data "$BODY" "$API$REQ_PATH"
else
  curl -fsS -X "$METHOD" -H "X-API-Token: $TOKEN" "$API$REQ_PATH"
fi' "API=${API}" "METHOD=$1" "REQ_PATH=$2" "BODY=${3:-}" "TOKEN=${API_TOKEN}"
}

# Canonical, comparable view of the seeded settings as the API returns them.
settings_snapshot() {
    local webhooks nodes security unauth token basic
    webhooks=$(api_json GET /api/notifications/webhooks) || return 1
    nodes=$(api_json GET /api/config/nodes) || return 1
    security=$(cexec 'curl -fsS "$API/api/security/status"' "API=${API}") || return 1
    unauth=$(api_status GET /api/config/nodes none)
    token=$(api_status GET /api/config/nodes token)
    basic=$(api_status GET /api/config/nodes basic)
    jq -n -S \
        --argjson webhooks "$webhooks" \
        --argjson nodes "$nodes" \
        --argjson security "$security" \
        --arg unauth "$unauth" --arg token "$token" --arg basic "$basic" \
        --arg webhook_name "$SEED_WEBHOOK_NAME" --arg node_name "$SEED_NODE_NAME" '
        {
          auth: {
            requiresAuth: ($security.requiresAuth // null),
            unauthenticated: $unauth,
            api_token: $token,
            password: $basic
          },
          webhook: ([$webhooks | .. | objects | select(.name? == $webhook_name)
                     | {id, name, url, method, service, enabled,
                        header_keys: ((.headers // {}) | keys)}] | first),
          node: ([$nodes | .. | objects | select(.name? == $node_name)
                  | {name, type, host, tokenName, hasToken, hasPassword, verifySSL}] | first)
        }'
}

# Expected shape of a healthy snapshot (independent of the baseline, so a
# baseline that silently lost data cannot make later phases pass).
check_snapshot_shape() {
    local snapshot="$1"
    local problems
    problems=$(jq -r \
        --arg webhook_name "$SEED_WEBHOOK_NAME" --arg webhook_url "$SEED_WEBHOOK_URL" \
        --arg node_name "$SEED_NODE_NAME" --arg node_host "$SEED_NODE_HOST" \
        --arg node_token_name "$SEED_NODE_TOKEN_NAME" --arg header "$SEED_WEBHOOK_HEADER" '
        [ (if .auth.requiresAuth != true then "security status does not require auth" else empty end),
          (if .auth.unauthenticated != "401" then "unauthenticated request returned \(.auth.unauthenticated), expected 401" else empty end),
          (if .auth.api_token != "200" then "API token request returned \(.auth.api_token), expected 200" else empty end),
          (if .auth.password != "200" then "password request returned \(.auth.password), expected 200" else empty end),
          (if .webhook == null then "seeded webhook \($webhook_name) missing" else empty end),
          (if .webhook != null and .webhook.url != $webhook_url then "seeded webhook URL changed to \(.webhook.url)" else empty end),
          (if .webhook != null and .webhook.method != "POST" then "seeded webhook method changed to \(.webhook.method)" else empty end),
          (if .webhook != null and .webhook.service != "generic" then "seeded webhook service changed to \(.webhook.service)" else empty end),
          (if .webhook != null and .webhook.enabled != false then "seeded webhook enabled state changed to \(.webhook.enabled)" else empty end),
          (if .webhook != null and ((.webhook.id // "") == "") then "seeded webhook lost its id" else empty end),
          (if .webhook != null and .webhook.header_keys != [$header] then "seeded webhook headers changed to \(.webhook.header_keys)" else empty end),
          (if .node == null then "seeded node \($node_name) missing" else empty end),
          (if .node != null and .node.type != "pve" then "seeded node type changed to \(.node.type)" else empty end),
          (if .node != null and .node.host != $node_host then "seeded node host changed to \(.node.host)" else empty end),
          (if .node != null and .node.tokenName != $node_token_name then "seeded node token name changed to \(.node.tokenName)" else empty end),
          (if .node != null and .node.hasToken != true then "seeded node lost its API token secret" else empty end),
          (if .node != null and .node.hasPassword != false then "seeded node gained a password" else empty end),
          (if .node != null and .node.verifySSL != false then "seeded node verifySSL changed to \(.node.verifySSL)" else empty end)
        ] | .[]' <<<"$snapshot") || problems="snapshot is not valid JSON"
    if [[ -n "$problems" ]]; then
        while IFS= read -r problem; do
            note_failure "settings: ${problem}"
        done <<<"$problems"
        return 1
    fi
}

datadir_manifest() {
    # path<TAB>sha256 for every regular file, excluding SQLite sidecars and
    # temp files whose presence depends on shutdown timing.
    cexec 'cd "$DATA_DIR" && find . -type f ! -name "*-wal" ! -name "*-shm" ! -name "*.tmp" ! -name "*.lock" -printf "%P\n" | LC_ALL=C sort | while IFS= read -r f; do printf "%s\t%s\n" "$f" "$(sha256sum "$f" | cut -d" " -f1)"; done' \
        "DATA_DIR=${DATA_DIR}"
}

# Encrypted stores holding the seeded secrets (node token value, webhook
# header value). The API only exposes hasToken and redacted header values, so
# byte identity of these files under an unchanged key is the proof that the
# exact secrets survived. A version that legitimately rewrites them fails the
# rehearsal loudly rather than passing unproven.
SECRET_STORES=".encryption.key nodes.enc webhooks.enc"

# compare_datadir BASELINE CURRENT LABEL: manifest comparison (pure, testable).
compare_datadir() {
    local baseline="$1" current="$2" label="$3"
    local missing changed added store before after
    missing=$(comm -23 <(cut -f1 "$baseline") <(cut -f1 "$current") | sed '/^$/d')
    added=$(comm -13 <(cut -f1 "$baseline") <(cut -f1 "$current") | sed '/^$/d' | wc -l | tr -d ' ')
    changed=$(join -t $'\t' "$baseline" "$current" | awk -F'\t' '$2 != $3' | wc -l | tr -d ' ')

    PHASE_DATADIR="$(wc -l < "$current" | tr -d ' ') files, ${changed} changed, ${added} new"
    log "${DATA_DIR} after ${label}: ${PHASE_DATADIR}"
    if [[ -n "$missing" ]]; then
        PHASE_DATADIR="${PHASE_DATADIR}, $(wc -l <<<"$missing" | tr -d ' ') missing"
        note_failure "files present before the upgrade are gone from ${DATA_DIR}: $(tr '\n' ' ' <<<"$missing")"
    fi
    for store in $SECRET_STORES; do
        before=$(awk -F'\t' -v f="$store" '$1 == f {print $2}' "$baseline")
        after=$(awk -F'\t' -v f="$store" '$1 == f {print $2}' "$current")
        if [[ -z "$before" || "$before" != "$after" ]]; then
            note_failure "${DATA_DIR}/${store} is missing or was rewritten; the seeded secrets are no longer provably intact"
        fi
    done
    if [[ "$changed" -gt 0 ]]; then
        log "files rewritten in place (informational unless listed above):"
        join -t $'\t' "$baseline" "$current" | awk -F'\t' '$2 != $3 {print "  " $1}'
    fi
}

check_datadir() {
    local baseline="${WORK_DIR}/state/datadir.baseline.tsv"
    local current="${WORK_DIR}/state/datadir.$1.tsv"
    datadir_manifest > "$current" || { note_failure "could not list ${DATA_DIR}"; PHASE_DATADIR="unreadable"; return 1; }
    compare_datadir "$baseline" "$current" "$1"
}

check_settings() {
    local snapshot
    if ! snapshot=$(settings_snapshot); then
        note_failure "settings could not be read back through the API"
        PHASE_SETTINGS="unreadable"
        return 1
    fi
    printf '%s\n' "$snapshot" > "${WORK_DIR}/state/settings.$1.json"
    local ok=true
    check_snapshot_shape "$snapshot" || ok=false
    if ! diff -u "${WORK_DIR}/state/settings.baseline.json" "${WORK_DIR}/state/settings.$1.json"; then
        note_failure "seeded settings read back differently than before the upgrade (diff above)"
        ok=false
    fi
    if [[ "$ok" == "true" ]]; then
        PHASE_SETTINGS="auth, webhook, node intact"
    else
        PHASE_SETTINGS="drift"
        return 1
    fi
}

# ---------------------------------------------------------------------------
# Phases
# ---------------------------------------------------------------------------

install_baseline() {
    if [[ "$HOSTED_BROWSER" == true ]]; then
        cexec 'bash /rehearsal/install.sh --version "$FROM_TAG"' "FROM_TAG=${FROM_TAG}" \
            > "${WORK_DIR}/state/private-install.log" 2>&1
    else
        cexec 'bash /rehearsal/install.sh --version "$FROM_TAG"' "FROM_TAG=${FROM_TAG}" \
            2>&1 | tee "${WORK_DIR}/state/install-${FROM_TAG}.log"
    fi
}

phase_install() {
    reset_phase_cells
    log "Phase 1: install ${FROM_TAG} with its published install.sh --version"
    # No TTY: install.sh's prompts take their defaults, as for curl | bash.
    if [[ "$HOSTED_BROWSER" == true ]]; then
        # /rehearsal is the fixed mount spelling used by the existing harness.
        # It is absent on a fresh hosted runner; never overwrite an existing path.
        [[ ! -e /rehearsal ]] || return 1
        ln -s "${WORK_DIR}/assets" /rehearsal
    fi
    if ! install_baseline; then
        note_failure "install.sh --version ${FROM_TAG} exited non-zero"
    fi
    assert_runtime "$FROM_TAG"
    record_phase "1. install" "$FROM_TAG" "$PHASE_VERSION" "$PHASE_HEALTH" "-" "-" "$PHASE_UNITS"
}

phase_seed() {
    reset_phase_cells
    log "Phase 2: seed first-run security, a webhook and a PVE node through the API"
    local token_output setup_token setup_response
    token_output=$(cexec 'runuser -u pulse -- env PULSE_DATA_DIR="$DATA_DIR" /opt/pulse/bin/pulse bootstrap-token' "DATA_DIR=${DATA_DIR}" 2>&1 || true)
    setup_token=$(grep -oE 'Token: [^[:space:]]+' <<<"$token_output" | head -1 | cut -d' ' -f2)
    if [[ -z "$setup_token" ]]; then
        note_failure "pulse bootstrap-token did not print a setup token"
        record_phase "2. seed" "$FROM_TAG" "-" "-" "-" "-" "-"
        return 1
    fi

    local setup_body
    setup_body=$(jq -cn --arg u "$SEED_ADMIN_USER" --arg p "$ADMIN_PASSWORD" --arg t "$API_TOKEN" \
        '{username: $u, password: $p, apiToken: $t}')
    if ! setup_response=$(cexec 'curl -fsS -X POST -H "Content-Type: application/json" -H "X-Setup-Token: $SETUP_TOKEN" --data "$BODY" "$API/api/security/quick-setup"' \
            "API=${API}" "SETUP_TOKEN=${setup_token}" "BODY=${setup_body}") \
        || [[ "$(jq -r '.success // false' <<<"$setup_response" 2>/dev/null)" != "true" ]]; then
        note_failure "first-run security setup did not return success"
    fi

    local webhook_body node_body
    webhook_body=$(jq -cn --arg n "$SEED_WEBHOOK_NAME" --arg u "$SEED_WEBHOOK_URL" \
        --arg hk "$SEED_WEBHOOK_HEADER" --arg hv "$NODE_TOKEN_VALUE" \
        '{name: $n, url: $u, method: "POST", service: "generic", enabled: false,
          headers: {($hk): $hv}}')
    if ! api_json POST /api/notifications/webhooks "$webhook_body" >/dev/null; then
        note_failure "creating the seed webhook failed"
    fi
    node_body=$(jq -cn --arg n "$SEED_NODE_NAME" --arg h "$SEED_NODE_HOST" --arg v "$NODE_TOKEN_VALUE" \
        --arg tn "$SEED_NODE_TOKEN_NAME" \
        '{type: "pve", name: $n, host: $h, tokenName: $tn, tokenValue: $v, verifySSL: false}')
    if ! api_json POST /api/config/nodes "$node_body" >/dev/null; then
        note_failure "creating the seed PVE node failed"
    fi

    local snapshot=""
    if snapshot=$(settings_snapshot) && check_snapshot_shape "$snapshot"; then
        printf '%s\n' "$snapshot" > "${WORK_DIR}/state/settings.baseline.json"
        PHASE_SETTINGS="auth, webhook, node seeded"
        log "Seeded settings as read back through the API:"
        printf '%s\n' "$snapshot"
    else
        if [[ -n "$snapshot" ]]; then
            printf '%s\n' "$snapshot"
        fi
        note_failure "seeded settings did not read back through the API"
        PHASE_SETTINGS="not readable"
    fi

    # Persistence is asynchronous for some stores; let writes settle before
    # the data dir becomes the upgrade baseline.
    sleep 5
    if datadir_manifest > "${WORK_DIR}/state/datadir.baseline.tsv" \
        && grep -q $'^\\.encryption\\.key\t' "${WORK_DIR}/state/datadir.baseline.tsv"; then
        PHASE_DATADIR="$(wc -l < "${WORK_DIR}/state/datadir.baseline.tsv" | tr -d ' ') files recorded"
        log "${DATA_DIR} baseline:"
        cut -f1 "${WORK_DIR}/state/datadir.baseline.tsv" | sed 's/^/  /'
    else
        note_failure "could not record ${DATA_DIR} (or it has no .encryption.key)"
        PHASE_DATADIR="not recorded"
    fi
    if ! auto_update_snapshot > "${WORK_DIR}/state/auto-updates.baseline.tsv"; then
        note_failure "could not record the installed auto-update choice and timer state"
    fi
    record_phase "2. seed" "$FROM_TAG" "-" "-" "$PHASE_SETTINGS" "$PHASE_DATADIR" "-"
}

run_updater() {
    local target="$1" label="$2"
    # The helper must be the one the Pulse server installer wrote, not a
    # community-scripts updater that ignores --version.
    if ! cexec 'test -x /bin/update && grep -q "^# Pulse update command" /bin/update'; then
        note_failure "/bin/update is missing or is not the Pulse server installer's update helper"
        return 1
    fi
    # /bin/update downloads the latest published install.sh, verifies it with
    # its embedded signer key and runs it with the given arguments.
    if [[ "$HOSTED_BROWSER" == true ]]; then
        if ! cexec '/bin/update --version "$TARGET"' "TARGET=${target}" \
                > "${WORK_DIR}/state/private-updater.log" 2>&1; then
            note_failure "/bin/update --version ${target} exited non-zero"
        fi
    elif ! cexec '/bin/update --version "$TARGET"' "TARGET=${target}" \
            2>&1 | tee "${WORK_DIR}/state/${label}-${target}.log"; then
        note_failure "/bin/update --version ${target} exited non-zero"
    fi
}

run_browser_journey() {
    local mode="$1" expected="$2"
    if [[ "$mode" == recovery ]]; then
        # An unreadable/missing/incomplete receipt cannot authorise another
        # browser process either. Only an explicit non-refused terminal result
        # permits readback after the independent documented CLI recovery.
        local receipt="${WORK_DIR}/state/browser-upgrade.json" reason
        if ! jq -e 'type == "object" and .access_refused == false
                and (.status == "passed" or .status == "failed")' "$receipt" >/dev/null 2>&1; then
            reason=upgrade-receipt-unavailable
            if jq -e '.access_refused == true' "$receipt" >/dev/null 2>&1; then
                reason=stopped-access-no-reauthentication
            fi
            printf '{"status":"not-executed","reason":"%s"}\n' "$reason" \
                > "${WORK_DIR}/state/browser-recovery.json"
            return 1
        fi
    fi
    (umask 077; jq -cn --arg username "$SEED_ADMIN_USER" --arg password "$ADMIN_PASSWORD" \
        '{username:$username,password:$password}' > "${WORK_DIR}/state/browser-auth.json")
    env -i PATH=/usr/local/bin:/usr/bin:/bin HOME="${WORK_DIR}" \
        PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-/opt/pulse-browser}" \
        "${PULSE_REHEARSAL_NODE:?}" "${ROOT_DIR}/tests/integration/scripts/release-browser-journey.cjs" \
        --origin "$PULSE_REHEARSAL_BROWSER_ORIGIN" --mode "$mode" \
        --from "$FROM_TAG" --to "$TO_TAG" --expected "$expected" \
        --auth-file "${WORK_DIR}/state/browser-auth.json" --output-dir "${WORK_DIR}/state"
}

phase_upgrade() {
    reset_phase_cells
    if [[ "$HOSTED_BROWSER" == true ]]; then
        log "Phase 3: upgrade to ${TO_TAG} through the authenticated Tailscale Serve browser"
        run_browser_journey upgrade "$TO_TAG" || note_failure "Tailscale browser upgrade failed; no apply replay"
    else
        log "Phase 3: upgrade with /bin/update --version ${TO_TAG}"
        run_updater "$TO_TAG" upgrade || true
    fi
    assert_runtime "$TO_TAG"
    check_auto_update_intent upgrade || true
    check_settings upgrade || true
    check_datadir upgrade || true
    record_phase "3. upgrade ($([[ "$HOSTED_BROWSER" == true ]] && printf browser || printf /bin/update))" "$TO_TAG" "$PHASE_VERSION" "$PHASE_HEALTH" "$PHASE_SETTINGS" "$PHASE_DATADIR" "$PHASE_UNITS"
}

phase_rollback() {
    reset_phase_cells
    log "Phase 4: roll back with /bin/update --version ${FROM_TAG}"
    # The documented recovery command is exercised even after an adverse
    # browser update. It is not a second attempt to apply the candidate.
    run_updater "$FROM_TAG" rollback || true
    assert_runtime "$FROM_TAG"
    if [[ "$HOSTED_BROWSER" == true ]]; then
        run_browser_journey recovery "$FROM_TAG" || note_failure "Tailscale browser recovery/readback failed"
    fi
    check_auto_update_intent rollback || true
    check_settings rollback || true
    check_datadir rollback || true
    record_phase "4. rollback (/bin/update)" "$FROM_TAG" "$PHASE_VERSION" "$PHASE_HEALTH" "$PHASE_SETTINGS" "$PHASE_DATADIR" "$PHASE_UNITS"
}

main() {
    log "Rehearsing ${FROM_TAG} -> ${TO_TAG} -> ${FROM_TAG} from ${REPO}; hosted browser=${HOSTED_BROWSER}"
    if ! fetch_and_verify_installer "$FROM_TAG"; then
        CURRENT_FAILURE="published install.sh for ${FROM_TAG} failed verification"
        record_phase "0. verify installer" "$FROM_TAG" "-" "-" "-" "-" "-" || abort_run
    fi
    if ! start_container; then
        CURRENT_FAILURE="disposable systemd host did not start"
        record_phase "0. systemd host" "-" "-" "-" "-" "-" "-" || abort_run
    fi

    phase_install || abort_run
    phase_seed || abort_run
    # Rollback runs even when the upgrade fails: it is the recovery path a
    # user reaches for in exactly that situation.
    phase_upgrade || true
    phase_rollback || true

    write_summary
    return "$OVERALL_STATUS"
}

if [[ -n "$CHECK_SNAPSHOT" ]]; then
    check_snapshot_shape "$(cat "$CHECK_SNAPSHOT")"
    exit $?
fi
if [[ ${#COMPARE_DATADIR[@]} -gt 0 ]]; then
    compare_datadir "${COMPARE_DATADIR[0]}" "${COMPARE_DATADIR[1]}" self-test
    [[ -z "$CURRENT_FAILURE" ]]
    exit $?
fi

main
