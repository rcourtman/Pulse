#!/bin/bash

# install-sensor-proxy.sh - Installs pulse-sensor-proxy on Proxmox host for secure temperature monitoring
# Supports --uninstall [--purge] to remove the proxy and cleanup resources.
# This script is idempotent and can be safely re-run

set -euo pipefail

CONFIG_FILE="/etc/pulse-sensor-proxy/config.yaml"
ALLOWED_NODES_FILE="/etc/pulse-sensor-proxy/allowed_nodes.yaml"
MIN_ALLOWED_NODES_FILE_VERSION="v4.31.1"

ALLOWLIST_MODE="file"
INSTALLED_PROXY_VERSION=""

PENDING_CONTROL_PLANE_FILE="/etc/pulse-sensor-proxy/pending-control-plane.env"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_info() {
    if [ "$QUIET" != true ]; then
        echo -e "${GREEN}[INFO]${NC} $1"
    fi
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

normalize_semver() {
    local ver="${1#v}"
    ver="${ver%%+*}"
    ver="${ver%%-*}"
    printf '%s' "$ver"
}

semver_to_tuple() {
    local ver
    ver="$(normalize_semver "$1")"
    IFS='.' read -r major minor patch <<< "$ver"
    [[ -n "$major" ]] || major=0
    [[ -n "$minor" ]] || minor=0
    [[ -n "$patch" ]] || patch=0
    printf '%s %s %s' "$major" "$minor" "$patch"
}

version_at_least() {
    local current target
    read -r c_major c_minor c_patch <<< "$(semver_to_tuple "$1")"
    read -r t_major t_minor t_patch <<< "$(semver_to_tuple "$2")"

    if (( c_major > t_major )); then
        return 0
    elif (( c_major < t_major )); then
        return 1
    fi

    if (( c_minor > t_minor )); then
        return 0
    elif (( c_minor < t_minor )); then
        return 1
    fi

    if (( c_patch >= t_patch )); then
        return 0
    fi
    return 1
}

detect_proxy_version() {
    local binary="$1"
    if [[ -x "$binary" ]]; then
        "$binary" version 2>/dev/null | awk '/pulse-sensor-proxy/{print $2; exit}'
    fi
}

config_command_supported() {
    local subcommand="$1"
    shift || true

    if [[ ! -x "$BINARY_PATH" ]]; then
        return 1
    fi

    local help_output
    help_output="$("$BINARY_PATH" config --help 2>/dev/null || true)"
    if ! grep -Eq "^[[:space:]]*${subcommand}([[:space:]]|$)" <<<"$help_output"; then
        return 1
    fi

    if [[ "$#" -eq 0 ]]; then
        return 0
    fi

    local sub_help
    sub_help="$("$BINARY_PATH" config "$subcommand" --help 2>/dev/null || true)"
    for flag in "$@"; do
        if ! grep -q -- "$flag" <<<"$sub_help"; then
            return 1
        fi
    done

    return 0
}

determine_allowlist_mode() {
    INSTALLED_PROXY_VERSION="$(detect_proxy_version "$BINARY_PATH")"

    if [[ -z "$INSTALLED_PROXY_VERSION" ]]; then
        # During initial install, version detection fails - that's expected
        ALLOWLIST_MODE="file"
        return
    fi

    if version_at_least "$INSTALLED_PROXY_VERSION" "$MIN_ALLOWED_NODES_FILE_VERSION"; then
        if [[ "$QUIET" != true ]]; then
            print_info "Detected pulse-sensor-proxy ${INSTALLED_PROXY_VERSION} (allowed_nodes_file supported)"
        fi
        ALLOWLIST_MODE="file"
        return
    fi

    # Refuse to install/upgrade on unsupported versions
    print_error "pulse-sensor-proxy ${INSTALLED_PROXY_VERSION} is too old (< ${MIN_ALLOWED_NODES_FILE_VERSION})"
    print_error "File-based allowlist is now required. Please upgrade the proxy binary first."
    print_error "Download latest from: https://github.com/rcourtman/Pulse/releases/latest"
    exit 1
}

record_pending_control_plane() {
    local mode="$1"
    if [[ -z "$PULSE_SERVER" ]]; then
        return
    fi

    cat > "$PENDING_CONTROL_PLANE_FILE" <<EOF
PENDING_PULSE_SERVER="${PULSE_SERVER}"
PENDING_MODE="${mode}"
PENDING_STANDALONE="${STANDALONE}"
PENDING_HTTP_MODE="${HTTP_MODE}"
PENDING_HTTP_ADDR="${HTTP_ADDR}"
EOF
    chmod 0600 "$PENDING_CONTROL_PLANE_FILE" 2>/dev/null || true
}

clear_pending_control_plane() {
    rm -f "$PENDING_CONTROL_PLANE_FILE" 2>/dev/null || true
}

format_ip_to_cidr() {
    local ip="$1"
    if [[ -z "$ip" ]]; then
        return
    fi

    if [[ "$ip" == */* ]]; then
        printf '%s' "$ip"
        return
    fi

    if [[ "$ip" == *:* ]]; then
        printf '%s/128' "$ip"
    else
        printf '%s/32' "$ip"
    fi
}

ensure_allowed_source_subnet() {
    local subnet="$1"
    if [[ -z "$subnet" || ! -f "$CONFIG_FILE" ]]; then
        return
    fi

    # Use robust binary config management if available
    if config_command_supported "add-subnet"; then
        if "$BINARY_PATH" config add-subnet "$subnet" --config "$CONFIG_FILE"; then
            print_info "Added allowed_source_subnets entry ${subnet}"
            return
        else
            print_warn "Failed to add subnet using binary; falling back to legacy method"
        fi
    fi

    local escaped_subnet="${subnet//\//\\/}"
    if grep -Eq "^[[:space:]]+-[[:space:]]*${escaped_subnet}([[:space:]]|$)" "$CONFIG_FILE"; then
        return
    fi

    local tmp
    tmp=$(mktemp)

    if grep -Eq "^[[:space:]]*allowed_source_subnets:" "$CONFIG_FILE"; then
    awk -v subnet="$subnet" '
/^allowed_source_subnets:/ {print; in_block=1; indent="    "; next}
in_block && /^[[:space:]]+-/ {
    # Capture indentation from existing items
    if (!captured) {
        match($0, /^[[:space:]]+/)
        indent = substr($0, RSTART, RLENGTH)
        captured = 1
    }
}
in_block && /^[^[:space:]]/ {
    if (!added) { printf("%s- %s\n", indent, subnet); added=1 }
    in_block=0
}
{print}
END {
    if (in_block && !added) {
        printf("%s- %s\n", indent, subnet)
    }
}
' "$CONFIG_FILE" > "$tmp"
    else
        cat "$CONFIG_FILE" > "$tmp"
        {
            echo ""
            echo "allowed_source_subnets:"
            echo "    - $subnet"
        } >> "$tmp"
    fi

    if mv "$tmp" "$CONFIG_FILE"; then
        print_info "Added allowed_source_subnets entry ${subnet}"
    else
        rm -f "$tmp"
        print_warn "Failed to update allowed_source_subnets with ${subnet}"
    fi
}


configure_local_authorized_key() {
    local auth_line=$1
    local auth_keys_file="/root/.ssh/authorized_keys"

    mkdir -p /root/.ssh
    touch "$auth_keys_file"
    chmod 600 "$auth_keys_file"

    # Extract just the key portion (without forced command prefix) for matching
    # Format: command="...",options KEY_TYPE KEY_DATA COMMENT
    local key_data
    key_data=$(echo "$auth_line" | grep -oE 'ssh-[a-z0-9-]+ [A-Za-z0-9+/=]+')

    # Check if this exact key already exists (regardless of forced command)
    if grep -qF "$key_data" "$auth_keys_file" 2>/dev/null; then
        if [ "$QUIET" != true ]; then
            print_success "SSH key already configured on localhost"
        fi
        return 0
    fi

    # Key not present, add it
    echo "${auth_line}" >>"$auth_keys_file"
    if [ "$QUIET" != true ]; then
        print_success "SSH key configured on localhost"
    fi
}

configure_container_proxy_env() {
    local socket_line="PULSE_SENSOR_PROXY_SOCKET=/mnt/pulse-proxy/pulse-sensor-proxy.sock"
    if ! SOCKET_LINE="$socket_line" pct exec "$CTID" -- bash <<'EOF'
set -e
ENV_FILE="/etc/pulse/.env"
mkdir -p /etc/pulse
if [[ -f "$ENV_FILE" ]] && grep -q "^PULSE_SENSOR_PROXY_SOCKET=" "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^PULSE_SENSOR_PROXY_SOCKET=.*|$SOCKET_LINE|" "$ENV_FILE"
else
  echo "$SOCKET_LINE" >> "$ENV_FILE"
fi
chmod 600 "$ENV_FILE" 2>/dev/null || true
chown pulse:pulse "$ENV_FILE" 2>/dev/null || true
EOF
    then
        print_warn "Unable to update /etc/pulse/.env inside container $CTID"
    fi
}

ensure_allowed_nodes_file_reference() {
    if [[ "$ALLOWLIST_MODE" != "file" ]]; then
        if [[ -f "$CONFIG_FILE" ]]; then
            sed -i '/^[[:space:]]*allowed_nodes_file:/d' "$CONFIG_FILE" 2>/dev/null || true
        fi
        return
    fi

    normalize_allowed_nodes_section
}

remove_allowed_nodes_block() {
    if [[ "$ALLOWLIST_MODE" != "file" ]]; then
        return
    fi

    normalize_allowed_nodes_section
}

normalize_allowed_nodes_section() {
    if [[ ! -f "$CONFIG_FILE" ]]; then
        return
    fi

    if command -v python3 >/dev/null 2>&1; then
        python3 - "$CONFIG_FILE" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
if not path.exists():
    sys.exit(0)

lines = path.read_text().splitlines()
to_skip = set()
saved_comment = None

def capture_comment_block(idx: int):
    global saved_comment
    blanks = []
    comments = []
    j = idx - 1
    while j >= 0 and lines[j].strip() == "":
        blanks.append((j, lines[j]))
        j -= 1
    while j >= 0 and lines[j].lstrip().startswith("#"):
        comments.append((j, lines[j]))
        j -= 1
    if not comments:
        return []
    blanks.reverse()
    comments.reverse()
    block = blanks + comments
    for index, _ in block:
        to_skip.add(index)
    return [text for _, text in block]

i = 0
while i < len(lines):
    line = lines[i]
    stripped = line.lstrip()
    if stripped.startswith("allowed_nodes_file:"):
        comment_block = capture_comment_block(i)
        if comment_block:
            saved_comment = comment_block
        to_skip.add(i)
        i += 1
        continue

    if stripped.startswith("allowed_nodes:"):
        comment_block = capture_comment_block(i)
        if comment_block:
            saved_comment = comment_block
        to_skip.add(i)
        i += 1
        while i < len(lines):
            next_line = lines[i]
            next_stripped = next_line.lstrip()
            if (
                next_stripped == ""
                or next_stripped.startswith("#")
                or next_stripped.startswith("-")
                or next_line.startswith((" ", "\t"))
            ):
                to_skip.add(i)
                i += 1
                continue
            break
        continue

    i += 1

result = [text for idx, text in enumerate(lines) if idx not in to_skip]

default_comment = [
    "# Cluster nodes (auto-discovered during installation)",
    "# These nodes are allowed to request temperature data when cluster IPC validation is unavailable",
]

if saved_comment is None:
    saved_comment = [""] + default_comment
else:
    while saved_comment and saved_comment[-1].strip() == "":
        saved_comment.pop()
    if saved_comment and saved_comment[0].strip() != "":
        saved_comment.insert(0, "")

if result and result[-1].strip() != "":
    result.append("")

result.extend(saved_comment)
result.append('allowed_nodes_file: "/etc/pulse-sensor-proxy/allowed_nodes.yaml"')

path.write_text("\n".join(result).rstrip() + "\n")
PY
        return
    fi

    # Fallback when python3 is unavailable
    sed -i '/^[[:space:]]*allowed_nodes:/,/^[^[:space:]]/d' "$CONFIG_FILE" 2>/dev/null || true
    sed -i '/^[[:space:]]*allowed_nodes_file:/d' "$CONFIG_FILE" 2>/dev/null || true
    if ! grep -q "allowed_nodes_file" "$CONFIG_FILE" 2>/dev/null; then
        {
            echo ""
            echo "# Cluster nodes (auto-discovered during installation)"
            echo "# These nodes are allowed to request temperature data when cluster IPC validation is unavailable"
            echo 'allowed_nodes_file: "/etc/pulse-sensor-proxy/allowed_nodes.yaml"'
        } >>"$CONFIG_FILE"
    fi
}

migrate_inline_allowed_nodes_to_file() {
    # Phase 2: Use config CLI for migration - no Python manipulation
    if [[ ! -f "$CONFIG_FILE" ]]; then
        return
    fi

    if [[ ! -x "$BINARY_PATH" ]]; then
        print_warn "Binary not available yet; skipping migration"
        return
    fi

    # Use CLI to atomically migrate inline nodes to file
    if "$BINARY_PATH" config migrate-to-file --config "$CONFIG_FILE" --allowed-nodes "$ALLOWED_NODES_FILE"; then
        print_success "Migration complete: inline allowed_nodes moved to file"
    fi
}

write_inline_allowed_nodes() {
    local comment_line="$1"
    shift || true
    local nodes=("$@")

    if [[ "$ALLOWLIST_MODE" != "inline" ]]; then
        return
    fi

    if ! command -v python3 >/dev/null 2>&1; then
        print_warn "python3 is required to manage inline allowed_nodes; skipping update"
        return
    fi

    python3 - "$CONFIG_FILE" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
if not path.exists():
    sys.exit(0)

lines = path.read_text().splitlines()
to_skip = set()

def capture_leading_comment(idx: int):
    """Remove contiguous blank/comment lines immediately above idx."""
    j = idx - 1
    while j >= 0 and lines[j].strip() == "":
        to_skip.add(j)
        j -= 1
    while j >= 0 and lines[j].lstrip().startswith("#"):
        to_skip.add(j)
        j -= 1

i = 0
while i < len(lines):
    line = lines[i]
    stripped = line.lstrip()
    if stripped.startswith("allowed_nodes_file:"):
        to_skip.add(i)
        i += 1
        continue
    if stripped.startswith("allowed_nodes:"):
        capture_leading_comment(i)
        to_skip.add(i)
        i += 1
        while i < len(lines):
            next_line = lines[i]
            next_stripped = next_line.lstrip()
            if (
                next_stripped == ""
                or next_stripped.startswith("#")
                or next_stripped.startswith("-")
                or next_line.startswith((" ", "\t"))
            ):
                to_skip.add(i)
                i += 1
                continue
            break
        continue
    i += 1

result = [text for idx, text in enumerate(lines) if idx not in to_skip]
path.write_text("\n".join(result).rstrip() + "\n")
PY

    python3 - "$CONFIG_FILE" "$comment_line" "${nodes[@]}" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
comment = (sys.argv[2] or "").strip()
new_nodes = [n.strip() for n in sys.argv[3:] if n.strip()]

lines = []
if path.exists():
    lines = path.read_text().splitlines()

skip = set()
existing = []
i = 0
while i < len(lines):
    line = lines[i]
    stripped = line.lstrip()
    if stripped.startswith("allowed_nodes_file:"):
        skip.add(i)
        i += 1
        continue
    if stripped.startswith("allowed_nodes:"):
        skip.add(i)
        i += 1
        while i < len(lines):
            current = lines[i]
            current_stripped = current.lstrip()
            if current_stripped.startswith("-"):
                value = current_stripped[1:].strip()
                if value:
                    existing.append(value)
                skip.add(i)
                i += 1
                continue
            if (
                current_stripped == "" or
                current_stripped.startswith("#") or
                current.startswith((" ", "\t"))
            ):
                skip.add(i)
                i += 1
                continue
            break
        continue
    i += 1

result = [line for idx, line in enumerate(lines) if idx not in skip]

seen = set()
merged = []
for entry in existing + new_nodes:
    normalized = entry.strip()
    if not normalized:
        continue
    key = normalized.lower()
    if key in seen:
        continue
    seen.add(key)
    merged.append(normalized)

if merged:
    if result and result[-1].strip() != "":
        result.append("")
    if comment:
        result.append(f"# {comment}")
    else:
        result.append("# Cluster nodes (auto-discovered during installation)")
    result.append("allowed_nodes:")
    for entry in merged:
        result.append(f"  - {entry}")

    content = "\n".join(result).rstrip()
    if content:
        content += "\n"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)
PY
}

cleanup_inline_allowed_nodes() {
    if [[ "$ALLOWLIST_MODE" != "inline" ]]; then
        return
    fi
    if ! command -v python3 >/dev/null 2>&1; then
        return
    fi
    if [[ ! -f "$CONFIG_FILE" ]]; then
        return
    fi

    python3 - "$CONFIG_FILE" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
if not path.exists():
    sys.exit(0)

lines = path.read_text().splitlines()
blocks = []
i = 0
while i < len(lines):
    line = lines[i]
    stripped = line.lstrip()
    if stripped.startswith("allowed_nodes:"):
        start = i
        entries = []
        j = i + 1
        while j < len(lines):
            nxt = lines[j]
            nxt_stripped = nxt.lstrip()
            if nxt_stripped.startswith("-"):
                entries.append(nxt_stripped[1:].strip())
                j += 1
                continue
            if (
                nxt_stripped.startswith("#")
                or nxt_stripped == ""
                or nxt.startswith((" ", "\t"))
            ):
                j += 1
                continue
            break

        comment_indices = set()
        comment_text = []
        k = start - 1
        while k >= 0 and lines[k].strip() == "":
            comment_indices.add(k)
            k -= 1
        while k >= 0 and lines[k].lstrip().startswith("#"):
            comment_indices.add(k)
            comment_text.append(lines[k])
            k -= 1
        comment_text.reverse()

        blocks.append(
            {
                "start": start,
                "end": j,
                "comment_indices": comment_indices,
                "comment_text": comment_text,
                "entries": entries,
            }
        )
        i = j
        continue
    i += 1

if len(blocks) <= 1:
    sys.exit(0)

seen = set()
merged = []
for block in blocks:
    for entry in block["entries"]:
        key = entry.lower()
        if not key or key in seen:
            continue
        seen.add(key)
        merged.append(entry)

if not merged:
    sys.exit(0)

first_block = blocks[0]
insert_at = min(
    [first_block["start"]] + list(first_block["comment_indices"])
) if first_block["comment_indices"] else first_block["start"]

def build_comment():
    if first_block["comment_text"]:
        return first_block["comment_text"]
    return ["# Cluster nodes (auto-discovered during installation)"]

comment_block = build_comment()
replacement = []
replacement.extend(comment_block)
if replacement and replacement[-1].strip() != "":
    replacement.append("")
replacement.append("allowed_nodes:")
for entry in merged:
    replacement.append(f"  - {entry}")
replacement.append("")

indices_to_remove = set()
for block in blocks:
    indices_to_remove.update(range(block["start"], block["end"]))
    indices_to_remove.update(block["comment_indices"])

result = []
inserted = False
for idx, line in enumerate(lines):
    if not inserted and idx == insert_at:
        result.extend(replacement)
        inserted = True
    if idx in indices_to_remove:
        continue
    result.append(line)

if not inserted:
    if result and result[-1].strip() != "":
        result.append("")
    result.extend(replacement)

content = "\n".join(result).rstrip() + "\n"
path.write_text(content)
PY
}

update_allowed_nodes() {
    local comment_line="$1"
    shift
    local nodes=("$@")

    # Phase 2: Use config CLI exclusively - no shell/Python manipulation
    # First, migrate any inline allowed_nodes to file mode (failures are fatal)
    if ! "$BINARY_PATH" config migrate-to-file --config "$CONFIG_FILE" --allowed-nodes "$ALLOWED_NODES_FILE"; then
        print_error "Failed to migrate config to file mode"
        return 1
    fi

    # Build --merge flags for the CLI
    local merge_args=()
    for node in "${nodes[@]}"; do
        if [[ -n "$node" ]]; then
            merge_args+=(--merge "$node")
        fi
    done

    if [[ ${#merge_args[@]} -eq 0 ]]; then
        return
    fi

    # Use the config CLI for atomic, locked updates
    if "$BINARY_PATH" config set-allowed-nodes --allowed-nodes "$ALLOWED_NODES_FILE" "${merge_args[@]}"; then
        chmod 0644 "$ALLOWED_NODES_FILE" 2>/dev/null || true
        chown pulse-sensor-proxy:pulse-sensor-proxy "$ALLOWED_NODES_FILE" 2>/dev/null || true
    else
        print_error "Failed to update allowed_nodes using config CLI"
        return 1
    fi
}



# Installation root - writable location that works on read-only /usr systems
INSTALL_ROOT="/opt/pulse/sensor-proxy"

# Binaries and scripts (in writable location)
BINARY_PATH="${INSTALL_ROOT}/bin/pulse-sensor-proxy"
WRAPPER_SCRIPT="${INSTALL_ROOT}/bin/pulse-sensor-wrapper.sh"
CLEANUP_SCRIPT_PATH="${INSTALL_ROOT}/bin/pulse-sensor-cleanup.sh"
SELFHEAL_SCRIPT="${INSTALL_ROOT}/bin/pulse-sensor-proxy-selfheal.sh"
STORED_INSTALLER="${INSTALL_ROOT}/install-sensor-proxy.sh"

# System configuration (standard locations)
SERVICE_PATH="/etc/systemd/system/pulse-sensor-proxy.service"
RUNTIME_DIR="/run/pulse-sensor-proxy"
SOCKET_PATH="${RUNTIME_DIR}/pulse-sensor-proxy.sock"
WORK_DIR="/var/lib/pulse-sensor-proxy"
SSH_DIR="${WORK_DIR}/ssh"
CONFIG_DIR="/etc/pulse-sensor-proxy"
CTID_FILE="${CONFIG_DIR}/ctid"
CLEANUP_PATH_UNIT="/etc/systemd/system/pulse-sensor-cleanup.path"
CLEANUP_SERVICE_UNIT="/etc/systemd/system/pulse-sensor-cleanup.service"
CLEANUP_REQUEST_PATH="${WORK_DIR}/cleanup-request.json"
SERVICE_USER="pulse-sensor-proxy"
LOG_DIR="/var/log/pulse/sensor-proxy"
SELFHEAL_SERVICE_UNIT="/etc/systemd/system/pulse-sensor-proxy-selfheal.service"
SELFHEAL_TIMER_UNIT="/etc/systemd/system/pulse-sensor-proxy-selfheal.timer"
SCRIPT_SOURCE="$(readlink -f "${BASH_SOURCE[0]:-$0}" 2>/dev/null || printf '%s' "${BASH_SOURCE[0]:-$0}")"
SKIP_SELF_HEAL_SETUP="${PULSE_SENSOR_PROXY_SELFHEAL:-false}"
GITHUB_REPO="rcourtman/Pulse"
LATEST_RELEASE_TAG=""
REQUESTED_VERSION=""
INSTALLER_CACHE_REASON=""
DEFER_SOCKET_VERIFICATION=false

# Get cluster node names via Proxmox API (pvesh) - returns one hostname per line
# Falls back to pvecm CLI parsing if pvesh unavailable
get_cluster_node_names() {
    # Try pvesh API first (structured JSON, version-stable)
    if command -v pvesh >/dev/null 2>&1; then
        local json_output
        if json_output=$(pvesh get /cluster/status --output-format json 2>/dev/null); then
            # Parse JSON: extract "name" from entries where "type" is "node"
            # Using python3 for reliable JSON parsing (always available on Proxmox)
            if command -v python3 >/dev/null 2>&1; then
                local names
                names=$(echo "$json_output" | python3 -c '
import sys, json
try:
    data = json.load(sys.stdin)
    for item in data:
        if item.get("type") == "node" and item.get("name"):
            print(item["name"])
except:
    pass
' 2>/dev/null)
                if [[ -n "$names" ]]; then
                    echo "$names"
                    return 0
                fi
            fi
        fi
    fi

    # Fallback: parse pvecm nodes CLI output (fragile, position-dependent)
    if command -v pvecm >/dev/null 2>&1; then
        pvecm nodes 2>/dev/null | awk '/^[[:space:]]+[0-9]/ && !/Qdevice/ {print $4}' || true
        return 0
    fi

    return 1
}

cleanup_local_authorized_keys() {
    local auth_keys_file="/root/.ssh/authorized_keys"
    if [[ ! -f "$auth_keys_file" ]]; then
        return
    fi

    if grep -q '# pulse-\(managed\|proxy\)-key$' "$auth_keys_file"; then
        if sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' "$auth_keys_file"; then
            print_info "Removed Pulse SSH keys from ${auth_keys_file}"
        else
            print_warn "Failed to clean Pulse SSH keys from ${auth_keys_file}"
        fi
    fi
}

cleanup_cluster_authorized_keys_manual() {
    local nodes=()
    # Use get_cluster_node_names helper (prefers pvesh API, falls back to pvecm CLI)
    while IFS= read -r nodename; do
        [[ -z "$nodename" ]] && continue
        local resolved_ip
        resolved_ip=$(getent hosts "$nodename" 2>/dev/null | awk '{print $1; exit}')
        [[ -n "$resolved_ip" ]] && nodes+=("$resolved_ip")
    done < <(get_cluster_node_names 2>/dev/null || true)
    # Fallback to pvecm status if hostname resolution didn't work
    if [[ ${#nodes[@]} -eq 0 ]] && command -v pvecm >/dev/null 2>&1; then
        while IFS= read -r node_ip; do
            [[ -n "$node_ip" ]] && nodes+=("$node_ip")
        done < <(pvecm status 2>/dev/null | grep -vEi "qdevice" | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {for(i=1;i<=NF;i++) if($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) print $i}' || true)
    fi

    if [[ ${#nodes[@]} -eq 0 ]]; then
        cleanup_local_authorized_keys
        return
    fi

    local local_ips
    local_ips="$(hostname -I 2>/dev/null || echo "")"
    local local_hostnames
    local_hostnames="$(hostname 2>/dev/null || echo "") $(hostname -f 2>/dev/null || echo "")"

    for node_ip in "${nodes[@]}"; do
        local is_local=false
        for local_ip in $local_ips; do
            if [[ "$node_ip" == "$local_ip" ]]; then
                is_local=true
                break
            fi
        done

        if [[ " $local_hostnames " == *" $node_ip "* ]]; then
            is_local=true
        fi

        if [[ "$node_ip" == "127.0.0.1" || "$node_ip" == "localhost" ]]; then
            is_local=true
        fi

        if [[ "$is_local" == true ]]; then
            cleanup_local_authorized_keys
            continue
        fi

        print_info "Removing Pulse SSH keys from node ${node_ip}"
        if ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
            "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys" 2>/dev/null; then
            print_info "  SSH keys cleaned on ${node_ip}"
        else
            print_warn "  Unable to clean Pulse SSH keys on ${node_ip}"
        fi
    done

    cleanup_local_authorized_keys
}

determine_installer_ref() {
    if [[ -n "$REQUESTED_VERSION" && "$REQUESTED_VERSION" != "latest" && "$REQUESTED_VERSION" != "main" ]]; then
        printf '%s' "$REQUESTED_VERSION"
        return 0
    fi

    if [[ -n "$LATEST_RELEASE_TAG" ]]; then
        printf '%s' "$LATEST_RELEASE_TAG"
        return 0
    fi

    if [[ "$REQUESTED_VERSION" == "main" ]]; then
        printf 'main'
        return 0
    fi

    printf 'main'
}

cache_installer_for_self_heal() {
    INSTALLER_CACHE_REASON=""
    install -d "${INSTALL_ROOT}"

    local source_issue=""
    if [[ -n "$SCRIPT_SOURCE" && -f "$SCRIPT_SOURCE" ]]; then
        if install -m 0755 "$SCRIPT_SOURCE" "$STORED_INSTALLER"; then
            return 0
        fi
        source_issue="failed to copy ${SCRIPT_SOURCE}"
    else
        source_issue="no readable source"
    fi

    local repo="${GITHUB_REPO:-rcourtman/Pulse}"
    local ref
    ref="$(determine_installer_ref)"
    [[ -z "$ref" ]] && ref="main"

    local candidate_urls=()
    if [[ "$ref" != "main" ]]; then
        # Try specific version from releases first
        candidate_urls+=("https://github.com/${repo}/releases/download/${ref}/install-sensor-proxy.sh")
    fi
    # Fall back to latest release
    candidate_urls+=("https://github.com/${repo}/releases/latest/download/install-sensor-proxy.sh")

    local tmp_file
    tmp_file=$(mktemp)
    local tmp_err
    tmp_err=$(mktemp)
    local last_error=""

    for url in "${candidate_urls[@]}"; do
        if curl --fail --silent --location --connect-timeout 10 --max-time 60 "$url" -o "$tmp_file" 2>"$tmp_err"; then
            if install -m 0755 "$tmp_file" "$STORED_INSTALLER"; then
                rm -f "$tmp_file" "$tmp_err"
                if [[ "$QUIET" != true ]]; then
                    print_info "Cached installer script for self-heal from ${url}"
                fi
                return 0
            fi
            last_error="failed to write cached installer to ${STORED_INSTALLER}"
            break
        fi

        if [[ -s "$tmp_err" ]]; then
            last_error="$(cat "$tmp_err")"
        else
            last_error="HTTP error"
        fi
        : >"$tmp_err"
    done

    rm -f "$tmp_file" "$tmp_err"

    if [[ -n "$source_issue" && -n "$last_error" ]]; then
        INSTALLER_CACHE_REASON="${source_issue}; download failed (${last_error})"
    elif [[ -n "$source_issue" ]]; then
        INSTALLER_CACHE_REASON="$source_issue"
    elif [[ -n "$last_error" ]]; then
        INSTALLER_CACHE_REASON="download failed (${last_error})"
    else
        INSTALLER_CACHE_REASON="unknown failure"
    fi

    return 1
}

perform_uninstall() {
    print_info "Starting pulse-sensor-proxy uninstall..."

    if command -v systemctl >/dev/null 2>&1; then
        print_info "Stopping pulse-sensor-proxy service"
        systemctl stop pulse-sensor-proxy 2>/dev/null || true
        print_info "Disabling pulse-sensor-proxy service"
        systemctl disable pulse-sensor-proxy 2>/dev/null || true

        print_info "Stopping cleanup path watcher"
        systemctl stop pulse-sensor-cleanup.path 2>/dev/null || true
        systemctl disable pulse-sensor-cleanup.path 2>/dev/null || true
        systemctl stop pulse-sensor-cleanup.service 2>/dev/null || true
        systemctl disable pulse-sensor-cleanup.service 2>/dev/null || true
    else
        print_warn "systemctl not available; skipping service disable"
    fi

    if [[ -x "$CLEANUP_SCRIPT_PATH" ]]; then
        print_info "Invoking cleanup script to remove Pulse SSH keys"
        mkdir -p "$WORK_DIR"
        cat > "$CLEANUP_REQUEST_PATH" <<'EOF'
{"host":""}
EOF
        if "$CLEANUP_SCRIPT_PATH"; then
            print_success "Cleanup script removed Pulse SSH keys"
        else
            print_warn "Cleanup script reported errors; attempting manual cleanup"
            cleanup_cluster_authorized_keys_manual
        fi
        rm -f "$CLEANUP_REQUEST_PATH"
    else
        cleanup_cluster_authorized_keys_manual
    fi

    if [[ -f "$BINARY_PATH" ]]; then
        rm -f "$BINARY_PATH"
        print_success "Removed binary ${BINARY_PATH}"
    else
        print_info "Binary already absent at ${BINARY_PATH}"
    fi

    if [[ -f "$SERVICE_PATH" ]]; then
        rm -f "$SERVICE_PATH"
        print_success "Removed service unit ${SERVICE_PATH}"
    fi

    if [[ -f "$CLEANUP_PATH_UNIT" ]]; then
        rm -f "$CLEANUP_PATH_UNIT"
        print_success "Removed cleanup path unit ${CLEANUP_PATH_UNIT}"
    fi

    if [[ -f "$CLEANUP_SERVICE_UNIT" ]]; then
        rm -f "$CLEANUP_SERVICE_UNIT"
        print_success "Removed cleanup service unit ${CLEANUP_SERVICE_UNIT}"
    fi

    if [[ -f "$SELFHEAL_TIMER_UNIT" ]]; then
        systemctl stop pulse-sensor-proxy-selfheal.timer 2>/dev/null || true
        systemctl disable pulse-sensor-proxy-selfheal.timer 2>/dev/null || true
        rm -f "$SELFHEAL_TIMER_UNIT"
        print_success "Removed self-heal timer ${SELFHEAL_TIMER_UNIT}"
    fi

    if [[ -f "$SELFHEAL_SERVICE_UNIT" ]]; then
        systemctl stop pulse-sensor-proxy-selfheal.service 2>/dev/null || true
        systemctl disable pulse-sensor-proxy-selfheal.service 2>/dev/null || true
        rm -f "$SELFHEAL_SERVICE_UNIT"
        print_success "Removed self-heal service ${SELFHEAL_SERVICE_UNIT}"
    fi

    if [[ -f "$SELFHEAL_SCRIPT" ]]; then
        rm -f "$SELFHEAL_SCRIPT"
        print_success "Removed self-heal helper ${SELFHEAL_SCRIPT}"
    fi

    if [[ -f "$STORED_INSTALLER" ]]; then
        rm -f "$STORED_INSTALLER"
        print_success "Removed cached installer ${STORED_INSTALLER}"
    fi

    if [[ -f "$CTID_FILE" ]]; then
        rm -f "$CTID_FILE"
    fi

    rm -f "$PENDING_CONTROL_PLANE_FILE" 2>/dev/null || true

    if command -v systemctl >/dev/null 2>&1; then
        systemctl daemon-reload 2>/dev/null || true
    fi

    rm -f "$CLEANUP_SCRIPT_PATH" "$CLEANUP_REQUEST_PATH" 2>/dev/null || true
    rm -f "$SOCKET_PATH" 2>/dev/null || true
    rm -rf "$RUNTIME_DIR" 2>/dev/null || true

    # Always remove HTTP secrets and TLS material (security best practice)
    if [[ -f "/etc/pulse-sensor-proxy/.http-auth-token" ]]; then
        rm -f "/etc/pulse-sensor-proxy/.http-auth-token"
        print_success "Removed HTTP auth token"
    fi
    if [[ -d "/etc/pulse-sensor-proxy/tls" ]]; then
        rm -rf "/etc/pulse-sensor-proxy/tls"
        print_success "Removed TLS certificates"
    fi

    # Check for and remove LXC bind mounts on any containers
    if command -v pct >/dev/null 2>&1; then
        print_info "Checking for LXC bind mounts..."
        # Find all containers with pulse-sensor-proxy bind mounts
        for ctid in $(pct list | awk 'NR>1 {print $1}'); do
            if grep -q "pulse-sensor-proxy" /etc/pve/lxc/${ctid}.conf 2>/dev/null; then
                CONTAINER_NAME=$(pct list | awk -v id="$ctid" '$1==id {print $3}')
                print_info "Found bind mount in container $ctid ($CONTAINER_NAME)"
                if sed -i '/pulse-sensor-proxy/d' /etc/pve/lxc/${ctid}.conf 2>/dev/null; then
                    print_success "Removed bind mount from container $ctid ($CONTAINER_NAME)"
                    print_warn "Container restart required for change to take effect"
                else
                    print_warn "Failed to remove bind mount from container $ctid"
                fi
            fi
        done
    fi

    if [[ "$PURGE" == true ]]; then
        print_info "Purging Pulse sensor proxy state"
        rm -rf "$WORK_DIR" "$CONFIG_DIR" 2>/dev/null || true
        if [[ -d "$LOG_DIR" ]]; then
            print_info "Removing log directory ${LOG_DIR}"
        fi
        rm -rf "$LOG_DIR" 2>/dev/null || true

        if id -u "$SERVICE_USER" >/dev/null 2>&1; then
            if userdel --remove "$SERVICE_USER" 2>/dev/null; then
                print_success "Removed service user ${SERVICE_USER}"
            elif userdel "$SERVICE_USER" 2>/dev/null; then
                print_success "Removed service user ${SERVICE_USER}"
            else
                print_warn "Failed to remove service user ${SERVICE_USER}"
            fi
        fi

        if getent group "$SERVICE_USER" >/dev/null 2>&1; then
            if groupdel "$SERVICE_USER" 2>/dev/null; then
                print_success "Removed service group ${SERVICE_USER}"
            else
                print_warn "Failed to remove service group ${SERVICE_USER}"
            fi
        fi
    else
        if [[ -d "$WORK_DIR" ]]; then
            print_info "Preserving data directory ${WORK_DIR} (use --purge to remove)"
        fi
        if [[ -d "$CONFIG_DIR" ]]; then
            print_info "Preserving config directory ${CONFIG_DIR} (use --purge to remove)"
        fi
        if [[ -d "$LOG_DIR" ]]; then
            print_info "Preserving log directory ${LOG_DIR} (use --purge to remove)"
        fi
    fi

    print_success "pulse-sensor-proxy uninstall complete"
}

# Parse arguments first to check for standalone mode
CTID=""
VERSION=""
LOCAL_BINARY=""
QUIET=false
PULSE_SERVER=""
STANDALONE=false
HTTP_MODE=false
HTTP_ADDR="0.0.0.0:8443"  # Explicitly IPv4 to avoid IPv6-only binding on some systems
FALLBACK_BASE="${PULSE_SENSOR_PROXY_FALLBACK_URL:-}"
SKIP_RESTART=false
RESTART_PULSE=false
UNINSTALL=false
PURGE=false
CONTROL_PLANE_TOKEN=""
CONTROL_PLANE_REFRESH=""
PROXY_URL=""  # Manual override for advertised proxy URL
SHORT_HOSTNAME=$(hostname -s 2>/dev/null || hostname | cut -d'.' -f1)

while [[ $# -gt 0 ]]; do
    case $1 in
        --ctid)
            CTID="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --local-binary)
            LOCAL_BINARY="$2"
            shift 2
            ;;
        --pulse-server)
            PULSE_SERVER="$2"
            shift 2
            ;;
        --quiet)
            QUIET=true
            shift
            ;;
        --standalone)
            STANDALONE=true
            shift
            ;;
        --http-mode)
            HTTP_MODE=true
            shift
            ;;
        --http-addr)
            HTTP_ADDR="$2"
            shift 2
            ;;
        --skip-restart)
            SKIP_RESTART=true
            shift
            ;;
        --restart-pulse)
            RESTART_PULSE=true
            shift
            ;;
        --uninstall)
            UNINSTALL=true
            shift
            ;;
        --purge)
            PURGE=true
            shift
            ;;
        --proxy-url)
            PROXY_URL="$2"
            shift 2
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ "$PURGE" == true && "$UNINSTALL" != true ]]; then
    print_warn "--purge is only valid together with --uninstall; ignoring"
    PURGE=false
fi

if [[ "$UNINSTALL" == true ]]; then
    perform_uninstall
    exit 0
fi

REQUESTED_VERSION="${VERSION:-latest}"

# If --pulse-server was provided, use it as the fallback base
if [[ -n "$PULSE_SERVER" ]]; then
    FALLBACK_BASE="${PULSE_SERVER}/api/install/pulse-sensor-proxy"
fi

# Preflight checks
if [[ $EUID -ne 0 ]]; then
   print_error "This script must be run as root"
   print_error "Use: sudo $0 $*"
   exit 1
fi

# Check required commands
REQUIRED_CMDS="curl openssl systemctl useradd groupadd install chmod chown mkdir jq"
if [[ "$HTTP_MODE" == true ]]; then
    REQUIRED_CMDS="$REQUIRED_CMDS hostname awk"
fi
if [[ "$STANDALONE" == false ]]; then
    REQUIRED_CMDS="$REQUIRED_CMDS pvecm"
fi

for cmd in $REQUIRED_CMDS; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        print_error "Required command not found: $cmd"
        print_error "Please install it and try again"
        exit 1
    fi
done

# Check if running on Proxmox host (only required for LXC mode)
if [[ "$STANDALONE" == false ]]; then
    if ! command -v pvecm >/dev/null 2>&1; then
        print_error "This script must be run on a Proxmox VE host"
        exit 1
    fi
fi

# Validate arguments based on mode
CONTAINER_ON_THIS_NODE=true
if [[ "$STANDALONE" == false ]]; then
    if [[ -z "$CTID" ]]; then
        print_error "Missing required argument: --ctid <container-id>"
        echo "Usage: $0 --ctid <container-id> [--pulse-server <url>] [--version <version>] [--local-binary <path>]"
        echo "   Or: $0 --standalone [--pulse-server <url>] [--version <version>] [--local-binary <path>]"
        echo "   Or: $0 --uninstall [--purge]"
        exit 1
    fi

    # Verify container exists on this node
    if ! pct status "$CTID" >/dev/null 2>&1; then
        # Container doesn't exist locally - might be on another cluster node
        # Continue installation for host temperature monitoring, skip container-specific config
        print_warn "Container $CTID does not exist on this node"
        print_warn "Will install sensor-proxy for host temperature monitoring only"
        print_warn "Container-specific socket mount configuration will be skipped"
        CONTAINER_ON_THIS_NODE=false
    fi
fi

if [[ "$STANDALONE" == true ]]; then
    print_info "Installing pulse-sensor-proxy for standalone/Docker deployment"
elif [[ "$CONTAINER_ON_THIS_NODE" == true ]]; then
    print_info "Installing pulse-sensor-proxy for container $CTID"
else
    print_info "Installing pulse-sensor-proxy for host monitoring (container $CTID on another node)"
fi

# Create dedicated service account if it doesn't exist
if ! id -u pulse-sensor-proxy >/dev/null 2>&1; then
    print_info "Creating pulse-sensor-proxy service account..."
    useradd --system --user-group --no-create-home --shell /usr/sbin/nologin pulse-sensor-proxy
    print_info "Service account created"
fi

# Ensure group exists (in case user was created without it)
if ! getent group pulse-sensor-proxy >/dev/null 2>&1; then
    print_info "Creating pulse-sensor-proxy group..."
    groupadd --system pulse-sensor-proxy
    usermod -aG pulse-sensor-proxy pulse-sensor-proxy
fi

# Add pulse-sensor-proxy user to www-data group for Proxmox IPC access (pvecm commands)
if ! groups pulse-sensor-proxy | grep -q '\bwww-data\b'; then
    print_info "Adding pulse-sensor-proxy to www-data group for Proxmox IPC access..."
    usermod -aG www-data pulse-sensor-proxy
fi

# Create installation directories before binary installation (handles fresh installs and upgrades)
print_info "Setting up installation directories..."
install -d -o root -g root -m 0755 "${INSTALL_ROOT}"
install -d -o root -g root -m 0755 "${INSTALL_ROOT}/bin"

# Install binary - either from local file or download from GitHub
if [[ -n "$LOCAL_BINARY" ]]; then
    # Use local binary for testing
    print_info "Using local binary: $LOCAL_BINARY"
    if [[ ! -f "$LOCAL_BINARY" ]]; then
        print_error "Local binary not found: $LOCAL_BINARY"
        exit 1
    fi
    cp "$LOCAL_BINARY" "$BINARY_PATH"
    chmod +x "$BINARY_PATH"
    print_info "Binary installed to $BINARY_PATH"
else
    # Detect architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            BINARY_NAME="pulse-sensor-proxy-linux-amd64"
            ARCH_LABEL="linux-amd64"
            ;;
        aarch64|arm64)
            BINARY_NAME="pulse-sensor-proxy-linux-arm64"
            ARCH_LABEL="linux-arm64"
            ;;
        armv7l|armhf)
            BINARY_NAME="pulse-sensor-proxy-linux-armv7"
            ARCH_LABEL="linux-armv7"
            ;;
        armv6l)
            BINARY_NAME="pulse-sensor-proxy-linux-armv6"
            ARCH_LABEL="linux-armv6"
            ;;
        i386|i686)
            BINARY_NAME="pulse-sensor-proxy-linux-386"
            ARCH_LABEL="linux-386"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    DOWNLOAD_SUCCESS=false
    ATTEMPTED_SOURCES=()

    fetch_latest_release_tag() {
        local api_url="https://api.github.com/repos/$GITHUB_REPO/releases?per_page=25"
        local tmp_err
        tmp_err=$(mktemp)
        local response
        response=$(curl --fail --silent --location --connect-timeout 10 --max-time 30 "$api_url" 2>"$tmp_err")
        local status=$?
        if [[ $status -ne 0 ]]; then
            if [[ -s "$tmp_err" ]]; then
                print_warn "Failed to resolve latest GitHub release: $(cat "$tmp_err")"
            else
                print_warn "Failed to resolve latest GitHub release (HTTP $status)"
            fi
            rm -f "$tmp_err"
            return 1
        fi
        rm -f "$tmp_err"

        local tag=""
        if command -v python3 >/dev/null 2>&1; then
            if ! tag=$(printf '%s' "$response" | python3 -c '
import json
import sys

binary_name = sys.argv[1]
arch_label = sys.argv[2]
tar_suffix = arch_label or ""

def has_sensor_assets(tag, assets):
    names = {asset.get("name") for asset in assets if isinstance(asset, dict) and asset.get("name")}
    if binary_name and binary_name in names:
        return True
    if tar_suffix:
        tarball = f"pulse-{tag}-{tar_suffix}.tar.gz"
        if tarball in names or f"{tarball}.sha256" in names:
            return True
    universal = f"pulse-{tag}.tar.gz"
    if universal in names or f"{universal}.sha256" in names:
        return True
    return False

try:
    releases = json.load(sys.stdin)
except json.JSONDecodeError:
    sys.exit(1)

for release in releases:
    if release.get("prerelease"):
        continue
    tag_name = (release.get("tag_name") or "").strip()
    if not tag_name or tag_name.startswith("helm-chart"):
        continue
    assets = release.get("assets") or []
    if has_sensor_assets(tag_name, assets):
        sys.stdout.write(tag_name)
        sys.exit(0)

sys.exit(0)
' "$BINARY_NAME" "$ARCH_LABEL"); then
                print_warn "Failed to parse GitHub releases via python3; falling back to heuristic tag detection"
                tag=""
            fi
        fi

        if [[ -z "$tag" ]]; then
            # Fallback: use grep-based parsing, filtering out helm-chart and common prerelease patterns
            tag=$(printf '%s\n' "$response" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4 | grep -Ev '^helm-chart-|-rc\.|-(alpha|beta|rc)[0-9]*$' | head -n 1 || true)
        fi

        if [[ -n "$tag" ]]; then
            tag="${tag%%$'\n'*}"
        fi

        if [[ -z "$tag" ]]; then
            print_warn "Could not determine latest GitHub release for pulse-sensor-proxy"
            return 1
        fi
        LATEST_RELEASE_TAG="$tag"
        return 0
    }

    attempt_github_asset_or_tarball() {
        local tag="$1"
        [[ -z "$tag" ]] && return 1

        local asset_url="https://github.com/$GITHUB_REPO/releases/download/${tag}/${BINARY_NAME}"
        ATTEMPTED_SOURCES+=("GitHub release asset ${tag}")
        print_info "Downloading $BINARY_NAME from GitHub release ${tag}..."
        local tmp_err
        tmp_err=$(mktemp)
        if curl --fail --silent --location --connect-timeout 10 --max-time 120 "$asset_url" -o "$BINARY_PATH.tmp" 2>"$tmp_err"; then
            rm -f "$tmp_err"
            DOWNLOAD_SUCCESS=true
            return 0
        fi

        local asset_error=""
        if [[ -s "$tmp_err" ]]; then
            asset_error="$(cat "$tmp_err")"
        fi
        rm -f "$tmp_err"
        rm -f "$BINARY_PATH.tmp" 2>/dev/null || true

        local tarball_name="pulse-${tag}-linux-${ARCH_LABEL#linux-}.tar.gz"
        local tarball_url="https://github.com/$GITHUB_REPO/releases/download/${tag}/${tarball_name}"
        ATTEMPTED_SOURCES+=("GitHub release tarball ${tarball_name}")
        print_info "Downloading ${tarball_name} to extract pulse-sensor-proxy..."
        tmp_err=$(mktemp)
        local tarball_tmp
        tarball_tmp=$(mktemp)
        if curl --fail --silent --location --connect-timeout 10 --max-time 240 "$tarball_url" -o "$tarball_tmp" 2>"$tmp_err"; then
            if tar -tzf "$tarball_tmp" >/dev/null 2>&1 && tar -xzf "$tarball_tmp" -C "$(dirname "$tarball_tmp")" ./bin/pulse-sensor-proxy >/dev/null 2>&1; then
                mv "$(dirname "$tarball_tmp")/bin/pulse-sensor-proxy" "$BINARY_PATH.tmp"
                rm -f "$tarball_tmp" "$tmp_err"
                DOWNLOAD_SUCCESS=true
                return 0
            else
                print_warn "Release tarball did not contain expected ./bin/pulse-sensor-proxy"
            fi
        else
            if [[ -s "$tmp_err" ]]; then
                print_warn "Tarball download failed: $(cat "$tmp_err")"
            else
                print_warn "Tarball download failed (HTTP error)"
            fi
        fi
        rm -f "$tarball_tmp" "$tmp_err"
        if [[ -n "$asset_error" ]]; then
            print_warn "GitHub release asset error: $asset_error"
        fi
        return 1
    }

    if [[ "$REQUESTED_VERSION" == "latest" || "$REQUESTED_VERSION" == "main" || -z "$REQUESTED_VERSION" ]]; then
        if fetch_latest_release_tag; then
            attempt_github_asset_or_tarball "$LATEST_RELEASE_TAG" || true
        fi
    else
        attempt_github_asset_or_tarball "$REQUESTED_VERSION" || true
    fi

    if [[ "$DOWNLOAD_SUCCESS" != true ]] && [[ -n "$FALLBACK_BASE" ]]; then
        fallback_url="$FALLBACK_BASE"
        if [[ "$fallback_url" == *"?"* ]]; then
            fallback_url="$fallback_url"
        elif [[ "$fallback_url" == *"pulse-sensor-proxy-"* ]]; then
            fallback_url="${fallback_url}"
        else
            fallback_url="${fallback_url%/}?arch=${ARCH_LABEL}"
        fi

        ATTEMPTED_SOURCES+=("Fallback ${fallback_url}")
        print_info "Downloading $BINARY_NAME from fallback source..."
        fallback_err=$(mktemp)
        if curl --fail --silent --location --connect-timeout 10 --max-time 120 "$fallback_url" -o "$BINARY_PATH.tmp" 2>"$fallback_err"; then
            rm -f "$fallback_err"
            DOWNLOAD_SUCCESS=true
        else
            if [[ -s "$fallback_err" ]]; then
                print_error "Fallback download failed: $(cat "$fallback_err")"
            else
                print_error "Fallback download failed (HTTP error)"
            fi
            rm -f "$fallback_err"
            rm -f "$BINARY_PATH.tmp" 2>/dev/null || true
        fi
    fi

    # NOTE: Previous versions attempted pct pull from container paths like
    # /opt/pulse/bin/pulse-sensor-proxy-linux-amd64, but these paths don't exist
    # in Pulse containers. Removed to prevent spurious PVE task errors (#817).

    if [[ "$DOWNLOAD_SUCCESS" != true ]]; then
        print_error "Unable to download pulse-sensor-proxy binary."
        if [[ ${#ATTEMPTED_SOURCES[@]} -gt 0 ]]; then
            print_error "Sources attempted:"
            for src in "${ATTEMPTED_SOURCES[@]}"; do
                print_error "  - $src"
            done
        fi
        print_error "Publish a GitHub release with binary assets or ensure a Pulse server is reachable."
        exit 1
    fi

    chmod +x "$BINARY_PATH.tmp"
    mv "$BINARY_PATH.tmp" "$BINARY_PATH"
    print_info "Binary installed to $BINARY_PATH"
fi

# Create remaining directories with proper ownership (handles fresh installs and upgrades)
print_info "Setting up service directories with proper ownership..."
if ! install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0750 /var/lib/pulse-sensor-proxy; then
    print_error "Failed to create /var/lib/pulse-sensor-proxy"
    exit 1
fi
if ! install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0700 "$SSH_DIR"; then
    print_error "Failed to create $SSH_DIR"
    exit 1
fi
if ! install -m 0600 -o pulse-sensor-proxy -g pulse-sensor-proxy /dev/null "$SSH_DIR/known_hosts"; then
    print_error "Failed to create $SSH_DIR/known_hosts"
    exit 1
fi
if ! install -d -o pulse-sensor-proxy -g pulse-sensor-proxy -m 0755 /etc/pulse-sensor-proxy; then
    print_error "Failed to create /etc/pulse-sensor-proxy"
    exit 1
fi

if [[ -n "$CTID" ]]; then
    echo "$CTID" > "$CTID_FILE"
    chmod 0644 "$CTID_FILE"
fi

# HTTP Mode Setup Functions
setup_tls_certificates() {
    local cert_path="$1"
    local key_path="$2"

    # Create TLS directory
    install -d -o root -g pulse-sensor-proxy -m 0750 /etc/pulse-sensor-proxy/tls

    if [[ -n "$cert_path" && -n "$key_path" ]]; then
        # Use provided certificates
        print_info "Using provided TLS certificates..."
        cp "$cert_path" /etc/pulse-sensor-proxy/tls/server.crt
        cp "$key_path" /etc/pulse-sensor-proxy/tls/server.key
        chmod 640 /etc/pulse-sensor-proxy/tls/server.crt
        chmod 640 /etc/pulse-sensor-proxy/tls/server.key
        chown root:pulse-sensor-proxy /etc/pulse-sensor-proxy/tls/server.crt
        chown root:pulse-sensor-proxy /etc/pulse-sensor-proxy/tls/server.key
    else
        # Generate self-signed certificate
        print_info "Generating self-signed TLS certificate..."

        # Get hostname and IPs for SAN
        HOSTNAME=$(hostname -f 2>/dev/null || hostname)
        IP_ADDRESSES=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -v '^$' | head -5)

        # Build SAN list
        SAN="DNS:${HOSTNAME},DNS:localhost"
        for ip in $IP_ADDRESSES; do
            SAN="${SAN},IP:${ip}"
        done

        # Generate 4096-bit RSA key and self-signed cert valid for 10 years
        openssl req -newkey rsa:4096 -nodes -x509 -days 3650 \
            -subj "/CN=${HOSTNAME}/O=Pulse Sensor Proxy" \
            -addext "subjectAltName=${SAN}" \
            -keyout /etc/pulse-sensor-proxy/tls/server.key \
            -out /etc/pulse-sensor-proxy/tls/server.crt \
            2>/dev/null || {
                print_error "Failed to generate TLS certificate"
                exit 1
            }

        chmod 640 /etc/pulse-sensor-proxy/tls/server.key
        chmod 640 /etc/pulse-sensor-proxy/tls/server.crt
        chown root:pulse-sensor-proxy /etc/pulse-sensor-proxy/tls/server.key
        chown root:pulse-sensor-proxy /etc/pulse-sensor-proxy/tls/server.crt

        # Log certificate fingerprint for audit
        CERT_FINGERPRINT=$(openssl x509 -in /etc/pulse-sensor-proxy/tls/server.crt -noout -fingerprint -sha256 2>/dev/null | cut -d= -f2)
        print_success "TLS certificate generated (SHA256: ${CERT_FINGERPRINT})"
    fi
}

register_with_pulse() {
    local pulse_url="$1"
    local hostname="$2"
    local proxy_url="$3"
    local mode="${4:-}"
    if [[ -z "$mode" ]]; then
        mode="socket"
    fi

    # Output to stderr so it doesn't interfere with command substitution
    print_info "Registering temperature proxy with Pulse at $pulse_url..." >&2

    # Build registration request with retry logic
    local response body
    local http_code
    local attempt
    local max_attempts=3
    local register_url="${pulse_url}/api/temperature-proxy/register"

    for attempt in $(seq 1 $max_attempts); do
        if [[ $attempt -gt 1 ]]; then
            print_info "Retry attempt $attempt/$max_attempts..." >&2
            sleep 2
        fi

        response=$(curl -w "\n%{http_code}" -sS -X POST \
            -H "Content-Type: application/json" \
            -d "{\"hostname\":\"${hostname}\",\"proxy_url\":\"${proxy_url}\",\"mode\":\"${mode}\"}" \
            "$register_url")

        local curl_exit=$?
        http_code=$(echo "$response" | tail -1)
        body=$(echo "$response" | head -n -1)

        # Retry network errors
        if [[ $curl_exit -ne 0 && -z "$http_code" ]]; then
            continue
        fi

        if [[ "$http_code" =~ ^20 ]]; then
            print_success "Registered successfully" >&2
            echo "$body"
            return 0
        fi

        if [[ "$http_code" == "404" && "$body" == *'"pve_instance_not_found"'* ]]; then
            print_warn "Pulse has not been configured with a Proxmox instance named '$hostname' yet." >&2
            print_warn "Add the node in Pulse (Settings → Nodes) and re-run the sensor proxy installer to enable control-plane sync." >&2
            return 0
        fi
        if [[ "$http_code" == "400" && "$body" == *'"missing_proxy_url"'* && "$mode" != "http" ]]; then
            if [[ $attempt -lt $max_attempts ]]; then
                print_warn "Pulse reported node '$hostname' is not ready yet; retrying..." >&2
                sleep 2
                continue
            fi
            print_warn "Pulse refused proxy registration because the node '$hostname' hasn't been added yet." >&2
            print_warn "Control-plane sync will be deferred until the node exists in Pulse; temperature proxy will run with a local allow list." >&2
            return 0
        fi

        if [[ $attempt -eq $max_attempts ]]; then
            print_error "Failed to register with Pulse API after $max_attempts attempts" >&2
            print_error "" >&2
            print_error "═══════════════════════════════════════════════════════" >&2
            print_error "Registration Details:" >&2
            print_error "═══════════════════════════════════════════════════════" >&2
            print_error "URL: $register_url" >&2
            print_error "HTTP Code: $http_code" >&2
            print_error "Hostname: $hostname" >&2
            print_error "Proxy URL: $proxy_url" >&2
            print_error "Response: $body" >&2
            print_error "" >&2
            print_error "Troubleshooting:" >&2
            print_error "  1. Ensure this PVE instance is added to Pulse first" >&2
            print_error "  2. Verify hostname matches instance name: $hostname" >&2
            print_error "  3. Check Pulse logs: docker logs pulse | tail -50" >&2
            print_error "  4. Test registration manually:" >&2
            print_error "     curl -X POST -H 'Content-Type: application/json' \\" >&2
            print_error "       -d '{\"hostname\":\"${hostname}\",\"proxy_url\":\"${proxy_url}\"}' \\" >&2
            print_error "       $register_url" >&2
            return 1
        fi
    done

    return 1
}

write_control_plane_token() {
    local token="$1"
    if [[ -z "$token" ]]; then
        return
    fi
    print_info "Writing control plane token..."
    echo "$token" > /etc/pulse-sensor-proxy/.pulse-control-token
    chmod 600 /etc/pulse-sensor-proxy/.pulse-control-token
    chown pulse-sensor-proxy:pulse-sensor-proxy /etc/pulse-sensor-proxy/.pulse-control-token
}

ensure_control_plane_config() {
    local pulse_url="$1"
    local refresh="$2"
    local config_file="/etc/pulse-sensor-proxy/config.yaml"

    if [[ -z "$pulse_url" ]]; then
        return
    fi
    if [[ -z "$refresh" ]]; then
        refresh=60
    fi

    # Use robust binary config management if available
    if config_command_supported "set-control-plane" "--url" "--token-file" "--refresh"; then
        if "$BINARY_PATH" config set-control-plane --url "$pulse_url" --token-file "/etc/pulse-sensor-proxy/.pulse-control-token" --refresh "$refresh" --config "$config_file"; then
            chown pulse-sensor-proxy:pulse-sensor-proxy "$config_file"
            chmod 0644 "$config_file"
            return
        else
            print_warn "Failed to set control plane using binary; falling back to legacy method"
        fi
    fi

    if grep -q "^pulse_control_plane:" "$config_file" 2>/dev/null; then
        # Re-write the existing control-plane block with the latest URL/token path.
        local tmp
        tmp=$(mktemp)
        awk -v url="$pulse_url" -v refresh="$refresh" '
            BEGIN { in_block = 0 }
            /^pulse_control_plane:/ {
                print "pulse_control_plane:"
                print "  url: " url
                print "  token_file: /etc/pulse-sensor-proxy/.pulse-control-token"
                print "  refresh_interval: " refresh
                print ""
                in_block = 1
                next
            }
            # Exit the replacement block when we hit a non-indented line
            in_block && /^[^[:space:]]/ { in_block = 0 }
            in_block { next }
            { print }
        ' "$config_file" > "$tmp"
        mv "$tmp" "$config_file"
        chown pulse-sensor-proxy:pulse-sensor-proxy "$config_file"
        chmod 0644 "$config_file"
        return
    fi

    cat >> "$config_file" << EOF

# Pulse control plane configuration (auto-generated)
pulse_control_plane:
  url: "$pulse_url"
  token_file: "/etc/pulse-sensor-proxy/.pulse-control-token"
  refresh_interval: $refresh
EOF
}

declare -a CONTROL_PLANE_ALLOWED_NODE_LIST=()

apply_allowed_nodes_from_response() {
    local response="$1"
    if [[ -z "$response" ]]; then
        return
    fi
    if ! command -v python3 >/dev/null 2>&1; then
        return
    fi

    local parsed_nodes
    parsed_nodes=$(printf '%s' "$response" | python3 -c '
import json, sys
try:
    payload = json.load(sys.stdin)
except Exception:
    payload = {}
nodes = payload.get("allowed_nodes") or []
for entry in nodes:
    ip = entry.get("ip") or ""
    name = entry.get("name") or ""
    value = (ip or name).strip()
    if value:
        print(value)
')

    if [[ -z "$parsed_nodes" ]]; then
        return
    fi

    mapfile -t __allowed_nodes <<<"$parsed_nodes"
    if [[ ${#__allowed_nodes[@]} -gt 0 ]]; then
        CONTROL_PLANE_ALLOWED_NODE_LIST=("${__allowed_nodes[@]}")
    fi
}

determine_allowlist_mode

# Migrate any existing inline allowed_nodes to file (Phase 1 hotfix for config corruption)
migrate_inline_allowed_nodes_to_file

cleanup_inline_allowed_nodes

# Create base config file if it doesn't exist
if [[ ! -f /etc/pulse-sensor-proxy/config.yaml ]]; then
    print_info "Creating base configuration file..."
    cat > /etc/pulse-sensor-proxy/config.yaml << 'EOF'
# Pulse Temperature Proxy Configuration
allowed_peer_uids: [1000]

# Allow ID-mapped root (LXC containers with sub-UID mapping)
allow_idmapped_root: true
allowed_idmap_users:
  - root

metrics_address: "127.0.0.1:9127"

rate_limit:
  per_peer_interval_ms: 333
  per_peer_burst: 10
EOF
    chown pulse-sensor-proxy:pulse-sensor-proxy /etc/pulse-sensor-proxy/config.yaml
    chmod 0644 /etc/pulse-sensor-proxy/config.yaml
fi

# Phase 2: Migration handled by update_allowed_nodes() -> config migrate-to-file
# No need to call ensure_allowed_nodes_file_reference anymore

# Register socket-mode proxy with Pulse if server provided
if [[ "$HTTP_MODE" != true ]]; then
    if [[ -z "$PULSE_SERVER" ]]; then
        print_warn "PULSE_SERVER not provided; control plane sync disabled. Temperatures will only work on this host."
    else
        print_info "Registering socket proxy with Pulse server ${PULSE_SERVER}..."
        registration_response=$(register_with_pulse "$PULSE_SERVER" "$SHORT_HOSTNAME" "" "socket")
        if [[ $? -eq 0 && -n "$registration_response" ]]; then
            CONTROL_PLANE_TOKEN=$(echo "$registration_response" | grep -o '"control_token":"[^"]*"' | head -1 | cut -d'"' -f4)
            CONTROL_PLANE_REFRESH=$(echo "$registration_response" | grep -o '"refresh_interval":[0-9]*' | head -1 | awk -F: '{print $2}')
            if [[ -z "$CONTROL_PLANE_REFRESH" ]]; then
                CONTROL_PLANE_REFRESH="60"
            fi
            apply_allowed_nodes_from_response "$registration_response"
            clear_pending_control_plane
        else
            print_warn "Failed to register socket proxy with Pulse; continuing without control plane sync"
            record_pending_control_plane "socket"
        fi
    fi
fi

# HTTP Mode Configuration
if [[ "$HTTP_MODE" == true ]]; then
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  HTTP Mode Setup (External PVE Host)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Validate required parameters
    if [[ -z "$PULSE_SERVER" ]]; then
        print_error "HTTP mode requires --pulse-server parameter"
        print_error "Example: --pulse-server https://pulse.example.com:7655"
        exit 1
    fi

    # Test Pulse server reachability before proceeding
    print_info "Testing connection to Pulse server..."
    if ! curl -f -s -m 5 "${PULSE_SERVER}/api/health" >/dev/null 2>&1; then
        print_error "Cannot reach Pulse server at: $PULSE_SERVER"
        print_error ""
        print_error "Troubleshooting:"
        print_error "  1. Verify Pulse is running: docker ps | grep pulse"
        print_error "  2. Check URL is correct (include protocol and port)"
        print_error "  3. Test connectivity: curl -v ${PULSE_SERVER}/api/health"
        print_error "  4. Check firewall allows access from this host"
        print_error ""
        print_error "Installation aborted to prevent incomplete setup"
        exit 1
    fi
    print_success "Pulse server is reachable"

    # Check if port is already in use
    # Extract port from HTTP_ADDR which can be ":8443" or "0.0.0.0:8443"
    PORT_NUMBER="${HTTP_ADDR##*:}"
    if ss -ltn | grep -q ":${PORT_NUMBER} "; then
        # Port is in use - check if it's our own service (refresh scenario)
        if systemctl is-active --quiet pulse-sensor-proxy 2>/dev/null; then
            # Check if the process using the port is pulse-sensor-proxy
            PORT_OWNER=$(ss -ltnp | grep ":${PORT_NUMBER} " | grep -o 'pulse-sensor-pr' || true)
            if [[ -n "$PORT_OWNER" ]]; then
                # Our service is using the port - this is a refresh, continue
                print_info "Existing pulse-sensor-proxy detected on port ${PORT_NUMBER} - will refresh configuration"
            else
                # Service is active but something else is using the port
                print_error "Port ${PORT_NUMBER} is already in use by another process"
                print_error ""
                print_error "Currently using port ${PORT_NUMBER}:"
                ss -ltnp | grep ":${PORT_NUMBER} " || true
                exit 1
            fi
        else
            # Service not active, port conflict with something else
            print_error "Port ${PORT_NUMBER} is already in use"
            print_error ""
            print_error "Currently using port ${PORT_NUMBER}:"
            ss -ltnp | grep ":${PORT_NUMBER} " || true
            print_error ""
            print_error "Options:"
            print_error "  1. Stop the conflicting service"
            print_error "  2. Use a different port: --http-addr :PORT"
            print_error "  3. If this is a previous sensor-proxy, uninstall first:"
            print_error "     $0 --uninstall"
            exit 1
        fi
    fi

    # Setup TLS certificates
    setup_tls_certificates "" ""  # Empty params = auto-generate

    # Determine proxy URL - use provided URL or auto-detect IP address
    if [[ -n "$PROXY_URL" ]]; then
        # User provided --proxy-url, use it directly
        print_info "Using manually specified proxy URL: $PROXY_URL"
    else
        # Auto-detect from primary IP
        PRIMARY_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
        if [[ -z "$PRIMARY_IP" ]]; then
            print_error "Failed to determine primary IP address"
            print_error "Use --proxy-url to specify manually"
            exit 1
        fi

        # Validate it's an IPv4 address (not IPv6)
        if [[ ! "$PRIMARY_IP" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            print_warn "Primary IP appears to be IPv6 or invalid: $PRIMARY_IP"
            print_warn "Attempting to find first IPv4 address..."
            PRIMARY_IP=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
            if [[ -z "$PRIMARY_IP" ]]; then
                print_error "No IPv4 address found"
                print_error "Use --proxy-url https://YOUR_IP:${PORT_NUMBER} to specify manually"
                exit 1
            fi
            print_info "Using IPv4 address: $PRIMARY_IP"
        fi

        # Warn if using loopback
        if [[ "$PRIMARY_IP" == "127.0.0.1" ]]; then
            print_warn "Primary IP is loopback (127.0.0.1)"
            print_warn "Pulse will not be able to reach this proxy!"
            print_error "Use --proxy-url https://YOUR_REAL_IP:${PORT_NUMBER} to specify a reachable address"
            exit 1
        fi

        PROXY_URL="https://${PRIMARY_IP}:${PORT_NUMBER}"
        print_info "Proxy will be accessible at: $PROXY_URL"
    fi

    # Register with Pulse and get auth/control tokens
    registration_response=$(register_with_pulse "$PULSE_SERVER" "$SHORT_HOSTNAME" "$PROXY_URL" "http")
    if [[ $? -ne 0 || -z "$registration_response" ]]; then
        print_error "Failed to register with Pulse - aborting installation"
        print_error "Fix the issue and re-run the installer"
        record_pending_control_plane "http"
        exit 1
    fi

    HTTP_AUTH_TOKEN=$(echo "$registration_response" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
    CONTROL_PLANE_TOKEN=$(echo "$registration_response" | grep -o '"control_token":"[^"]*"' | head -1 | cut -d'"' -f4)
    CONTROL_PLANE_REFRESH=$(echo "$registration_response" | grep -o '"refresh_interval":[0-9]*' | head -1 | awk -F: '{print $2}')
    if [[ -z "$CONTROL_PLANE_REFRESH" ]]; then
        CONTROL_PLANE_REFRESH="60"
    fi
    apply_allowed_nodes_from_response "$registration_response"
    clear_pending_control_plane

    if [[ -z "$HTTP_AUTH_TOKEN" ]]; then
        print_error "Registration succeeded but Pulse did not return an auth token"
        print_error "Response: $registration_response"
        exit 1
    fi

    echo "$HTTP_AUTH_TOKEN" > /etc/pulse-sensor-proxy/.http-auth-token
    chmod 600 /etc/pulse-sensor-proxy/.http-auth-token
    chown pulse-sensor-proxy:pulse-sensor-proxy /etc/pulse-sensor-proxy/.http-auth-token

    # Backup config and token files before modifying
    if [[ -f "$CONFIG_FILE" ]]; then
        BACKUP_TIMESTAMP="$(date +%s)"
        BACKUP_CONFIG="${CONFIG_FILE}.backup.$BACKUP_TIMESTAMP"
        cp "$CONFIG_FILE" "$BACKUP_CONFIG"
        print_info "Config backed up to: $BACKUP_CONFIG"

        # Also backup token files so rollback restores matching secrets
        if [[ -f /etc/pulse-sensor-proxy/.pulse-control-token ]]; then
            BACKUP_CONTROL_TOKEN="/etc/pulse-sensor-proxy/.pulse-control-token.backup.$BACKUP_TIMESTAMP"
            cp /etc/pulse-sensor-proxy/.pulse-control-token "$BACKUP_CONTROL_TOKEN"
        fi

        # Remove any existing HTTP configuration to prevent duplicates
        if grep -q "^# HTTP Mode Configuration" "$CONFIG_FILE"; then
            print_info "Removing existing HTTP configuration..."
            # Remove from "# HTTP Mode Configuration" to end of file
            sed -i '/^# HTTP Mode Configuration/,$ d' "$CONFIG_FILE"
        fi
    fi

    # Extract Pulse server IP/hostname for allowed_source_subnets
    # Remove protocol and port to get just the host
    PULSE_HOST=$(echo "$PULSE_SERVER" | sed -E 's#^https?://##' | sed -E 's#:[0-9]+$##')

    # Try to resolve to IP if it's a hostname
    PULSE_IP=$(getent hosts "$PULSE_HOST" 2>/dev/null | awk '{print $1; exit}')
    if [[ -z "$PULSE_IP" ]]; then
        # Fallback: assume PULSE_HOST is already an IP or use it as-is
        PULSE_IP="$PULSE_HOST"
    fi

    print_info "Pulse server detected at: $PULSE_IP"

    HTTP_ALLOWED_SUBNETS=()
    PULSE_HTTP_SUBNET="$(format_ip_to_cidr "$PULSE_IP")"
    LOCAL_HTTP_SUBNET="$(format_ip_to_cidr "$PRIMARY_IP")"
    LOOPBACK_HTTP_SUBNET="127.0.0.1/32"

    [[ -n "$PULSE_HTTP_SUBNET" ]] && HTTP_ALLOWED_SUBNETS+=("$PULSE_HTTP_SUBNET")
    HTTP_ALLOWED_SUBNETS+=("$LOOPBACK_HTTP_SUBNET")
    [[ -n "$LOCAL_HTTP_SUBNET" ]] && HTTP_ALLOWED_SUBNETS+=("$LOCAL_HTTP_SUBNET")

    declare -A HTTP_SUBNET_SEEN=()
    deduped_http_subnets=()
    for subnet in "${HTTP_ALLOWED_SUBNETS[@]}"; do
        [[ -z "$subnet" ]] && continue
        if [[ -z "${HTTP_SUBNET_SEEN[$subnet]+x}" ]]; then
            HTTP_SUBNET_SEEN[$subnet]=1
            deduped_http_subnets+=("$subnet")
        fi
    done
    HTTP_ALLOWED_SUBNETS=("${deduped_http_subnets[@]}")

    # Configure HTTP mode - check if already configured to avoid duplicates
    print_info "Configuring HTTP mode..."
    if config_command_supported "set-http" "--enabled" "--listen-addr" "--auth-token" "--tls-cert" "--tls-key"; then
        if "$BINARY_PATH" config set-http \
            --enabled=true \
            --listen-addr="$HTTP_ADDR" \
            --auth-token="$HTTP_AUTH_TOKEN" \
            --tls-cert="/etc/pulse-sensor-proxy/tls/server.crt" \
            --tls-key="/etc/pulse-sensor-proxy/tls/server.key" \
            --config="$CONFIG_FILE"; then
            
            for subnet in "${HTTP_ALLOWED_SUBNETS[@]}"; do
                ensure_allowed_source_subnet "$subnet"
            done
            print_info "HTTP mode configured successfully (using binary)"
        else
            print_warn "Failed to set HTTP config using binary; falling back to legacy method"
            # Fallback to legacy logic
            if grep -q "^http_enabled:" "$CONFIG_FILE" 2>/dev/null; then
                sed -i "s|^http_auth_token:.*|http_auth_token: $HTTP_AUTH_TOKEN|" "$CONFIG_FILE"
                for subnet in "${HTTP_ALLOWED_SUBNETS[@]}"; do
                    ensure_allowed_source_subnet "$subnet"
                done
                print_info "Updated HTTP auth token (existing HTTP mode configuration kept)"
            else
                cat >> "$CONFIG_FILE" << EOF

# HTTP Mode Configuration (External PVE Host)
http_enabled: true
http_listen_addr: "$HTTP_ADDR"
http_tls_cert: /etc/pulse-sensor-proxy/tls/server.crt
http_tls_key: /etc/pulse-sensor-proxy/tls/server.key
http_auth_token: "$HTTP_AUTH_TOKEN"

# Allow HTTP connections from Pulse server, localhost, and this host
allowed_source_subnets:
EOF
                for subnet in "${HTTP_ALLOWED_SUBNETS[@]}"; do
                    echo "    - $subnet" >> "$CONFIG_FILE"
                done
            fi
        fi
    else
        if grep -q "^http_enabled:" "$CONFIG_FILE" 2>/dev/null; then
            # HTTP mode already configured - only update the token (avoid duplicates)
            sed -i "s|^http_auth_token:.*|http_auth_token: $HTTP_AUTH_TOKEN|" "$CONFIG_FILE"
            for subnet in "${HTTP_ALLOWED_SUBNETS[@]}"; do
                ensure_allowed_source_subnet "$subnet"
            done
            print_info "Updated HTTP auth token (existing HTTP mode configuration kept)"
        else
            # Fresh HTTP mode configuration - append to file
            cat >> "$CONFIG_FILE" << EOF

# HTTP Mode Configuration (External PVE Host)
http_enabled: true
http_listen_addr: "$HTTP_ADDR"
http_tls_cert: /etc/pulse-sensor-proxy/tls/server.crt
http_tls_key: /etc/pulse-sensor-proxy/tls/server.key
http_auth_token: "$HTTP_AUTH_TOKEN"

# Allow HTTP connections from Pulse server, localhost, and this host
allowed_source_subnets:
EOF
            for subnet in "${HTTP_ALLOWED_SUBNETS[@]}"; do
                echo "    - $subnet" >> "$CONFIG_FILE"
            done
        fi
    fi
    chown pulse-sensor-proxy:pulse-sensor-proxy "$CONFIG_FILE"
    chmod 0644 "$CONFIG_FILE"

    print_success "HTTP mode configured successfully"
    echo ""
    # Extract port number correctly from HTTP_ADDR (handles both ":8443" and "0.0.0.0:8443")
    HTTP_PORT="${HTTP_ADDR##*:}"
    print_info "Firewall configuration required:"
    print_info "  Allow inbound TCP connections on port ${HTTP_PORT} from Pulse server"
    print_info "  Command: ufw allow from <pulse-server-ip> to any port ${HTTP_PORT}"
    echo ""
fi

if [[ -n "$CONTROL_PLANE_TOKEN" && -n "$PULSE_SERVER" ]]; then
    write_control_plane_token "$CONTROL_PLANE_TOKEN"
    ensure_control_plane_config "$PULSE_SERVER" "$CONTROL_PLANE_REFRESH"
fi

# Stop existing service if running (for upgrades)
if systemctl is-active --quiet pulse-sensor-proxy 2>/dev/null; then
    print_info "Stopping existing service for upgrade..."
    # Tolerate timeout from slow HTTPS shutdown (can take 30s)
    systemctl stop pulse-sensor-proxy || true
    # Clear any failed state from the stop
    systemctl reset-failed pulse-sensor-proxy 2>/dev/null || true
fi

# Install hardened systemd service
print_info "Installing hardened systemd service..."

# Generate service file based on mode (Proxmox vs standalone)
if [[ "$STANDALONE" == true ]]; then
    # Standalone/Docker mode - no Proxmox-specific paths
    cat > "$SERVICE_PATH" <<EOF
[Unit]
Description=Pulse Temperature Proxy
Documentation=https://github.com/rcourtman/Pulse
After=network.target

[Service]
Type=simple
User=pulse-sensor-proxy
Group=pulse-sensor-proxy
WorkingDirectory=/var/lib/pulse-sensor-proxy
# Validate config before starting (Phase 2: prevent corruption from starting service)
ExecStartPre=${BINARY_PATH} config validate --config /etc/pulse-sensor-proxy/config.yaml
ExecStart=${BINARY_PATH} --config /etc/pulse-sensor-proxy/config.yaml
Restart=on-failure
RestartSec=5s

# Runtime dirs/sockets
RuntimeDirectory=pulse-sensor-proxy
RuntimeDirectoryMode=0775
RuntimeDirectoryPreserve=yes
LogsDirectory=pulse/sensor-proxy
LogsDirectoryMode=0750
UMask=0007

# Core hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/lib/pulse-sensor-proxy
ReadWritePaths=-/run/corosync
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ProtectClock=true
PrivateTmp=true
PrivateDevices=true
ProtectProc=invisible
ProcSubset=pid
LockPersonality=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
RestrictNamespaces=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
CapabilityBoundingSet=
AmbientCapabilities=
KeyringMode=private
LimitNOFILE=1024

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-sensor-proxy

[Install]
WantedBy=multi-user.target
EOF
else
    # Proxmox mode - include Proxmox paths
    cat > "$SERVICE_PATH" <<EOF
[Unit]
Description=Pulse Temperature Proxy
Documentation=https://github.com/rcourtman/Pulse
After=network.target

[Service]
Type=simple
User=pulse-sensor-proxy
Group=pulse-sensor-proxy
SupplementaryGroups=www-data
WorkingDirectory=/var/lib/pulse-sensor-proxy
# Validate config before starting (Phase 2: prevent corruption from starting service)
ExecStartPre=${BINARY_PATH} config validate --config /etc/pulse-sensor-proxy/config.yaml
ExecStart=${BINARY_PATH}
Restart=on-failure
RestartSec=5s

# Runtime dirs/sockets
RuntimeDirectory=pulse-sensor-proxy
RuntimeDirectoryMode=0775
RuntimeDirectoryPreserve=yes
LogsDirectory=pulse/sensor-proxy
LogsDirectoryMode=0750
UMask=0007

# Core hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/lib/pulse-sensor-proxy
ReadWritePaths=-/run/corosync
ReadOnlyPaths=/run/pve-cluster /etc/pve
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ProtectClock=true
PrivateTmp=true
PrivateDevices=true
ProtectProc=invisible
ProcSubset=pid
LockPersonality=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
RestrictNamespaces=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
CapabilityBoundingSet=
AmbientCapabilities=
KeyringMode=private
LimitNOFILE=1024

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-sensor-proxy

[Install]
WantedBy=multi-user.target
EOF
fi

# Add HTTP mode paths to service file if needed
if [[ "$HTTP_MODE" == true ]]; then
    print_info "Updating service file for HTTP mode..."

    # Add ReadOnlyPaths for TLS directory (after ReadWritePaths line)
    sed -i '/^ReadWritePaths=\/var\/lib\/pulse-sensor-proxy/a ReadOnlyPaths=/etc/pulse-sensor-proxy/tls' "$SERVICE_PATH"

    print_success "Service file updated for HTTP mode"
fi

# Reload systemd and start service
print_info "Enabling and starting service..."
if ! systemctl daemon-reload; then
    print_error "Failed to reload systemd daemon"
    journalctl -u pulse-sensor-proxy -n 20 --no-pager
    exit 1
fi

if ! systemctl enable pulse-sensor-proxy.service; then
    print_error "Failed to enable pulse-sensor-proxy service"
    journalctl -u pulse-sensor-proxy -n 20 --no-pager
    exit 1
fi

if ! systemctl start pulse-sensor-proxy.service; then
    print_error "Failed to start pulse-sensor-proxy service"
    print_error ""

    # Attempt rollback if HTTP mode and we have a backup
    if [[ "$HTTP_MODE" == true && -n "$BACKUP_CONFIG" && -f "$BACKUP_CONFIG" ]]; then
        print_warn "Attempting to rollback to previous configuration..."
        if cp "$BACKUP_CONFIG" /etc/pulse-sensor-proxy/config.yaml; then
            print_info "Config restored from backup"
            # Also restore token files to match the old config
            if [[ -n "$BACKUP_CONTROL_TOKEN" && -f "$BACKUP_CONTROL_TOKEN" ]]; then
                cp "$BACKUP_CONTROL_TOKEN" /etc/pulse-sensor-proxy/.pulse-control-token
                print_info "Control plane token restored from backup"
            fi
            # Clear failed state before attempting rollback start
            systemctl reset-failed pulse-sensor-proxy 2>/dev/null || true
            if systemctl start pulse-sensor-proxy.service; then
                print_success "Service restarted with previous configuration"
                print_error ""
                print_error "HTTP mode installation failed but previous config restored"
                print_error "Temperature monitoring should still work via Unix socket"
                print_error "Review the error above and fix before retrying"
                exit 1
            else
                print_error "Rollback failed - service won't start even with old config"
            fi
        else
            print_error "Failed to restore config from backup"
        fi
    fi

    print_error "═══════════════════════════════════════════════════════"
    print_error "Service Status:"
    print_error "═══════════════════════════════════════════════════════"
    systemctl status pulse-sensor-proxy --no-pager --lines=0 2>&1 || true
    print_error ""
    print_error "═══════════════════════════════════════════════════════"
    print_error "Recent Logs (last 40 lines):"
    print_error "═══════════════════════════════════════════════════════"
    journalctl -u pulse-sensor-proxy -n 40 --no-pager 2>&1 || true
    print_error ""
    print_error "═══════════════════════════════════════════════════════"
    print_error "Common Issues:"
    print_error "═══════════════════════════════════════════════════════"
    print_error "1. Missing user: Run 'useradd --system --no-create-home --group pulse-sensor-proxy'"
    print_error "2. Permission errors: Check ownership of /var/lib/pulse-sensor-proxy"
    print_error "3. lm-sensors not installed: Run 'apt-get install lm-sensors && sensors-detect --auto'"
    print_error "4. Standalone node detection: If you see 'pvecm' errors, this is expected for non-clustered hosts"
    print_error "5. Port already in use: Check 'ss -tlnp | grep ${HTTP_ADDR##*:}'"
    print_error ""
    print_error "For more help: https://github.com/rcourtman/Pulse/blob/main/docs/TROUBLESHOOTING.md"
    exit 1
fi

# Wait for socket to appear
print_info "Waiting for socket..."
for i in {1..10}; do
    if [[ -S "$SOCKET_PATH" ]]; then
        break
    fi
    sleep 1
done

if [[ ! -S "$SOCKET_PATH" ]]; then
    print_error "Socket did not appear at $SOCKET_PATH after 10 seconds"
    print_error ""
    print_error "═══════════════════════════════════════════════════════"
    print_error "Diagnostics:"
    print_error "═══════════════════════════════════════════════════════"
    print_error "Service Status:"
    systemctl status pulse-sensor-proxy --no-pager 2>&1 || true
    print_error ""
    print_error "Socket Directory Permissions:"
    ls -la /run/pulse-sensor-proxy/ 2>&1 || echo "Directory does not exist"
    print_error ""
    print_error "Recent Logs:"
    journalctl -u pulse-sensor-proxy -n 20 --no-pager 2>&1 || true
    print_error ""
    print_error "Common Causes:"
    print_error "  • Service failed to start (check logs above)"
    print_error "  • RuntimeDirectory permissions issue"
    print_error "  • Systemd socket creation failed"
    print_error ""
    print_error "Try: systemctl restart pulse-sensor-proxy && watch -n 0.5 'ls -la /run/pulse-sensor-proxy/'"
    exit 1
fi

print_info "Socket ready at $SOCKET_PATH"

# If socket verification was deferred because the runtime directory was
# missing earlier, test the container mount now that the proxy is running.
if [[ "$STANDALONE" == false && "$DEFER_SOCKET_VERIFICATION" = true ]]; then
    print_info "Validating container socket visibility now that host proxy is running..."
    if pct exec "$CTID" -- test -S "${MOUNT_TARGET}/pulse-sensor-proxy.sock"; then
        print_info "✓ Secure socket communication ready"
        DEFER_SOCKET_VERIFICATION=false
        [ -n "$LXC_CONFIG_BACKUP" ] && rm -f "$LXC_CONFIG_BACKUP"
    else
        print_error "Socket not visible at ${MOUNT_TARGET}/pulse-sensor-proxy.sock"
        print_error "Bind mount exists but container still cannot access the proxy socket"
        print_error "This usually indicates the container needs a restart or the mount failed to attach"

        if [ -n "$LXC_CONFIG_BACKUP" ] && [ -f "$LXC_CONFIG_BACKUP" ]; then
            print_warn "Rolling back container configuration changes..."
            cp "$LXC_CONFIG_BACKUP" "$LXC_CONFIG"
            rm -f "$LXC_CONFIG_BACKUP"
            print_info "Container configuration restored to previous state"
        fi
        exit 1
    fi
fi

# Validate HTTP endpoint if HTTP mode is enabled
if [[ "$HTTP_MODE" == true ]]; then
    print_info "Validating HTTP endpoint..."

    # Wait a moment for HTTP server to fully start
    sleep 2

    # Test HTTP endpoint
    HTTP_CHECK_URL="https://${PRIMARY_IP}${HTTP_ADDR}/health"
    if curl -f -s -k -m 5 \
        -H "Authorization: Bearer ${HTTP_AUTH_TOKEN}" \
        "$HTTP_CHECK_URL" >/dev/null 2>&1; then
        print_success "HTTP endpoint validated successfully"
    else
        print_error "HTTP endpoint validation failed"
        print_error "URL: $HTTP_CHECK_URL"
        print_error ""
        print_error "Troubleshooting:"
        print_error "  1. Check if port ${HTTP_ADDR##*:} is listening: ss -tlnp | grep ${HTTP_ADDR##*:}"
        print_error "  2. Check sensor-proxy logs: journalctl -u pulse-sensor-proxy -n 50"
        print_error "  3. Test manually: curl -k -H 'Authorization: Bearer \$TOKEN' $HTTP_CHECK_URL"
        print_error ""
        print_warn "Service is running but HTTP endpoint may not be accessible"
        print_warn "Temperature monitoring may not work properly"
    fi
fi

# Install sensor wrapper script for combined sensor and SMART data collection
print_info "Installing sensor wrapper script..."
cat > "$WRAPPER_SCRIPT" << 'WRAPPER_EOF'
#!/bin/bash
#
# pulse-sensor-wrapper.sh
# Combined sensor and SMART temperature collection for Pulse monitoring
#
# This script is deployed as the SSH forced command for the sensor proxy.
# It collects CPU/GPU temps via sensors and disk temps via smartctl,
# returning a unified JSON payload.

set -euo pipefail

# Configuration
CACHE_DIR="/var/cache/pulse-sensor-proxy"
SMART_CACHE_TTL=1800  # 30 minutes
MAX_SMARTCTL_TIME=5   # seconds per disk

# Ensure cache directory exists
mkdir -p "$CACHE_DIR" 2>/dev/null || true

# Function to get cached SMART data
get_cached_smart() {
    local cache_file="$CACHE_DIR/smart-temps.json"
    local now=$(date +%s)

    # Check if cache exists and is fresh
    if [[ -f "$cache_file" ]]; then
        local mtime=$(stat -c %Y "$cache_file" 2>/dev/null || echo 0)
        local age=$((now - mtime))

        if [[ $age -lt $SMART_CACHE_TTL ]]; then
            cat "$cache_file"
            return 0
        fi
    fi

    # Cache miss or stale - return empty array and trigger background refresh
    echo "[]"

    # Trigger async refresh if not already running (use lock file)
    local lock_file="$CACHE_DIR/smart-refresh.lock"
    if ! [ -f "$lock_file" ]; then
        (refresh_smart_cache &)
    fi

    return 0
}

# Function to refresh SMART cache in background
refresh_smart_cache() {
    local lock_file="$CACHE_DIR/smart-refresh.lock"
    local cache_file="$CACHE_DIR/smart-temps.json"
    local temp_file="${cache_file}.tmp.$$"

    # Create lock file and ensure cleanup on exit
    touch "$lock_file" 2>/dev/null || return 1
    trap "rm -f '$lock_file' '$temp_file'" EXIT

    local disks=()

    # Find all physical disks (skip partitions, loop devices, etc.)
    while IFS= read -r dev; do
        [[ -b "$dev" ]] && disks+=("$dev")
    done < <(lsblk -nd -o NAME,TYPE | awk '$2=="disk" {print "/dev/"$1}')

    local results=()

    for dev in "${disks[@]}"; do
        # Use smartctl with standby check to avoid waking sleeping drives
        # -n standby: skip if drive is in standby/sleep mode
        # -i: include identity data (serial/WWN/model)
        # --json=o: output original smartctl JSON format
        # timeout: prevent hanging on problematic drives

        local output
        if output=$(timeout ${MAX_SMARTCTL_TIME}s smartctl -n standby -i -A --json=o "$dev" 2>/dev/null); then
            # Parse the JSON output
            local temp=$(echo "$output" | jq -r '
                .temperature.current //
                (.ata_smart_attributes.table[] | select(.id == 194) | .raw.value) //
                (.nvme_smart_health_information_log.temperature // empty)
            ' 2>/dev/null)

            local serial=$(echo "$output" | jq -r '.serial_number // empty' 2>/dev/null)
            local wwn=$(echo "$output" | jq -r '.wwn.naa // .wwn.oui // empty' 2>/dev/null)
            local model=$(echo "$output" | jq -r '.model_name // .model_family // empty' 2>/dev/null)
            local transport=$(echo "$output" | jq -r '.device.type // empty' 2>/dev/null)

            # Only include if we got a valid temperature
            if [[ -n "$temp" && "$temp" != "null" && "$temp" =~ ^[0-9]+$ ]]; then
                local entry=$(jq -n \
                    --arg dev "$dev" \
                    --arg serial "$serial" \
                    --arg wwn "$wwn" \
                    --arg model "$model" \
                    --arg transport "$transport" \
                    --argjson temp "$temp" \
                    --arg updated "$(date -Iseconds)" \
                    '{
                        device: $dev,
                        serial: $serial,
                        wwn: $wwn,
                        model: $model,
                        type: $transport,
                        temperature: $temp,
                        lastUpdated: $updated,
                        standbySkipped: false
                    }')
                results+=("$entry")
            fi
        elif echo "$output" | grep -q "standby"; then
            # Drive is in standby - record it but don't wake it
            local entry=$(jq -n \
                --arg dev "$dev" \
                --arg updated "$(date -Iseconds)" \
                '{
                    device: $dev,
                    temperature: null,
                    lastUpdated: $updated,
                    standbySkipped: true
                }')
            results+=("$entry")
        fi

        # Small delay between disks to avoid saturating SATA controller
        sleep 0.1
    done

    # Build final JSON array
    if [[ ${#results[@]} -gt 0 ]]; then
        local json=$(printf '%s\n' "${results[@]}" | jq -s '.')
    else
        local json="[]"
    fi

    # Atomic write to cache
    echo "$json" > "$temp_file"
    mv "$temp_file" "$cache_file"
    chmod 644 "$cache_file" 2>/dev/null || true
}

# Main execution

# Collect sensor data (CPU, GPU temps)
sensors_data=$(sensors -j 2>/dev/null || echo '{}')

# Get SMART data from cache
smart_data=$(get_cached_smart)

# Combine into unified payload
jq -n \
    --argjson sensors "$sensors_data" \
    --argjson smart "$smart_data" \
    '{
        sensors: $sensors,
        smart: $smart
    }'
WRAPPER_EOF

chmod +x "$WRAPPER_SCRIPT"
print_success "Sensor wrapper installed at $WRAPPER_SCRIPT"

# Install cleanup system for full Pulse removal when nodes are deleted
print_info "Installing cleanup system..."

# Install cleanup script
cat > "$CLEANUP_SCRIPT_PATH" <<'CLEANUP_EOF'
#!/bin/bash

# pulse-sensor-cleanup.sh - Complete Pulse footprint removal when nodes are removed
# Removes: SSH keys, proxy service, binaries, API tokens, and LXC bind mounts
# This script is triggered by systemd path unit when cleanup-request.json is created

set -euo pipefail

# Configuration
WORK_DIR="/var/lib/pulse-sensor-proxy"
CLEANUP_REQUEST="${WORK_DIR}/cleanup-request.json"
LOCKFILE="${WORK_DIR}/cleanup.lock"
LOG_TAG="pulse-sensor-cleanup"
INSTALLER_PATH="/opt/pulse/sensor-proxy/install-sensor-proxy.sh"

# Logging functions
log_info() {
    logger -t "$LOG_TAG" -p user.info "$1"
    echo "[INFO] $1"
}

log_warn() {
    logger -t "$LOG_TAG" -p user.warning "$1"
    echo "[WARN] $1"
}

log_error() {
    logger -t "$LOG_TAG" -p user.err "$1"
    echo "[ERROR] $1" >&2
}

# Acquire exclusive lock to prevent concurrent cleanup runs
exec 200>"$LOCKFILE"
if ! flock -n 200; then
    log_info "Another cleanup instance is running, exiting"
    exit 0
fi

# Check if cleanup request file exists
if [[ ! -f "$CLEANUP_REQUEST" ]]; then
    log_info "No cleanup request found at $CLEANUP_REQUEST"
    exit 0
fi

log_info "Processing cleanup request from $CLEANUP_REQUEST"

# Read and parse the cleanup request
CLEANUP_DATA=$(cat "$CLEANUP_REQUEST")
HOST=$(echo "$CLEANUP_DATA" | grep -o '"host":"[^"]*"' | cut -d'"' -f4 || echo "")
REQUESTED_AT=$(echo "$CLEANUP_DATA" | grep -o '"requestedAt":"[^"]*"' | cut -d'"' -f4 || echo "")

log_info "Cleanup requested at: ${REQUESTED_AT:-unknown}"

# Rename request file to .processing to prevent re-triggering while allowing retry on failure
PROCESSING_FILE="${CLEANUP_REQUEST}.processing"
mv "$CLEANUP_REQUEST" "$PROCESSING_FILE" 2>/dev/null || {
    log_warn "Failed to rename cleanup request file, may have been processed by another instance"
    exit 0
}

# If no specific host was provided, clean up all known nodes
if [[ -z "$HOST" ]]; then
    log_info "No specific host provided - cleaning up all cluster nodes"

    # Discover cluster nodes (prefer hostname resolution for multi-network clusters)
    # Use get_cluster_node_names helper (prefers pvesh API, falls back to pvecm CLI)
    CLUSTER_NODES=""
    while IFS= read -r nodename; do
        [[ -z "$nodename" ]] && continue
        resolved_ip=$(getent hosts "$nodename" 2>/dev/null | awk '{print $1; exit}')
        if [[ -n "$resolved_ip" ]]; then
            CLUSTER_NODES="${CLUSTER_NODES:+$CLUSTER_NODES }$resolved_ip"
        fi
    done < <(get_cluster_node_names 2>/dev/null || true)
    # Fallback to pvecm status if hostname resolution didn't work
    if [[ -z "$CLUSTER_NODES" ]] && command -v pvecm >/dev/null 2>&1; then
        CLUSTER_NODES=$(pvecm status 2>/dev/null | grep -vEi "qdevice" | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {for(i=1;i<=NF;i++) if($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) print $i}' || true)
    fi

    if [[ -n "$CLUSTER_NODES" ]]; then
        for node_ip in $CLUSTER_NODES; do
            log_info "Cleaning up SSH keys on node $node_ip"

            # Remove both pulse-managed-key and pulse-proxy-key entries
            ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys" 2>&1 | \
                logger -t "$LOG_TAG" -p user.info || \
                log_warn "Failed to clean up SSH keys on $node_ip"
        done
        log_info "Cluster cleanup completed"
    else
        # Standalone node - clean up localhost
        log_info "Standalone node detected - cleaning up localhost"
        sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
            logger -t "$LOG_TAG" -p user.info || \
            log_warn "Failed to clean up SSH keys on localhost"
    fi
else
    log_info "Cleaning up specific host: $HOST"

    # Extract hostname/IP from host URL
    HOST_CLEAN=$(echo "$HOST" | sed -e 's|^https\?://||' -e 's|:.*$||')

    # Check if this is localhost (by IP, hostname, or FQDN)
    LOCAL_IPS=$(hostname -I 2>/dev/null || echo "")
    LOCAL_HOSTNAME=$(hostname 2>/dev/null || echo "")
    LOCAL_FQDN=$(hostname -f 2>/dev/null || echo "")
    IS_LOCAL=false

    # Check against all local IPs
    for local_ip in $LOCAL_IPS; do
        if [[ "$HOST_CLEAN" == "$local_ip" ]]; then
            IS_LOCAL=true
            break
        fi
    done

    # Check against hostname and FQDN
    if [[ "$HOST_CLEAN" == "127.0.0.1" || "$HOST_CLEAN" == "localhost" || \
          "$HOST_CLEAN" == "$LOCAL_HOSTNAME" || "$HOST_CLEAN" == "$LOCAL_FQDN" ]]; then
        IS_LOCAL=true
    fi

    if [[ "$IS_LOCAL" == true ]]; then
        log_info "Performing full cleanup on localhost"

        # 1. Remove SSH keys
        log_info "Removing SSH keys from authorized_keys"
        sed -i -e '/# pulse-managed-key$/d' -e '/# pulse-proxy-key$/d' /root/.ssh/authorized_keys 2>&1 | \
            logger -t "$LOG_TAG" -p user.info || \
            log_warn "Failed to clean up SSH keys"

        # 2. Delete API tokens and user
        log_info "Removing Proxmox API tokens and pulse-monitor user"
        if command -v pveum >/dev/null 2>&1; then
            # Try JSON output first (pveum with --output-format json)
            TOKEN_IDS=""
            if command -v python3 >/dev/null 2>&1; then
                # Try pveum with JSON output
                if TOKEN_JSON=$(pveum user token list pulse-monitor@pam --output-format json 2>/dev/null); then
                    TOKEN_IDS=$(echo "$TOKEN_JSON" | python3 -c '
import sys, json
try:
    data = json.load(sys.stdin)
    if isinstance(data, list):
        for item in data:
            if "tokenid" in item:
                print(item["tokenid"])
except: pass
' || true)
                fi
            fi

            # Fall back to pvesh JSON API if pveum JSON didn't work
            if [[ -z "$TOKEN_IDS" ]] && command -v pvesh >/dev/null 2>&1; then
                if TOKEN_JSON=$(pvesh get /access/users/pulse-monitor@pam/token 2>/dev/null); then
                    TOKEN_IDS=$(echo "$TOKEN_JSON" | python3 -c '
import sys, json
try:
    data = json.load(sys.stdin)
    if isinstance(data, dict) and "data" in data:
        for item in data["data"]:
            if "tokenid" in item:
                print(item["tokenid"])
except: pass
' 2>/dev/null || true)
                fi
            fi

            # Last resort: parse table output with better filtering
            if [[ -z "$TOKEN_IDS" ]]; then
                TOKEN_IDS=$(pveum user token list pulse-monitor@pam 2>/dev/null | \
                    awk 'NR>1 && /^[[:space:]]*pulse/ {print $1}' | grep -v '^[│┌└╞─]' | grep -v '^$' || true)
            fi

            if [[ -n "$TOKEN_IDS" ]]; then
                for token_id in $TOKEN_IDS; do
                    log_info "Deleting API token: $token_id"
                    pveum user token remove pulse-monitor@pam "${token_id}" 2>&1 | \
                        logger -t "$LOG_TAG" -p user.info || \
                        log_warn "Failed to delete token $token_id"
                done
            else
                log_info "No API tokens found for pulse-monitor@pam"
            fi

            # Remove the pulse-monitor user
            log_info "Removing pulse-monitor@pam user"
            pveum user delete pulse-monitor@pam 2>&1 | \
                logger -t "$LOG_TAG" -p user.info || \
                log_warn "pulse-monitor@pam user not found or already removed"
        else
            log_warn "pveum command not available, skipping API token cleanup"
        fi

        # 3. Remove LXC bind mounts
        log_info "Removing LXC bind mounts from container configs"
        if command -v pct >/dev/null 2>&1; then
            for ctid in $(pct list 2>/dev/null | awk 'NR>1 {print $1}' || true); do
                CONF_FILE="/etc/pve/lxc/${ctid}.conf"
                if [[ -f "$CONF_FILE" ]]; then
                    # Find pulse-sensor-proxy mount points and remove them using pct
                    for mp_key in $(grep -o "^mp[0-9]\+:" "$CONF_FILE" | grep -f <(grep "pulse-sensor-proxy" "$CONF_FILE" | grep -o "^mp[0-9]\+:") || true); do
                        mp_num="${mp_key%:}"
                        log_info "Removing ${mp_num} (pulse-sensor-proxy) from container $ctid"
                        if pct set "$ctid" -delete "${mp_num}" 2>&1 | logger -t "$LOG_TAG" -p user.info; then
                            log_info "Successfully removed ${mp_num} from container $ctid"
                        else
                            log_warn "Failed to remove ${mp_num} from container $ctid"
                        fi
                    done
                fi
            done
        fi

        # 4. Uninstall proxy service and remove binaries via isolated transient unit
        log_info "Starting full uninstallation (service, binaries, configs)"
        if [[ -x "$INSTALLER_PATH" ]]; then
            # Use systemd-run to create isolated transient unit that won't be killed
            # when we stop pulse-sensor-proxy.service
            if command -v systemd-run >/dev/null 2>&1; then
                # Use UUID for unique unit name (prevents same-second collisions)
                UNINSTALL_UUID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || date +%s%N)
                UNINSTALL_UNIT="pulse-uninstall-${UNINSTALL_UUID}"
                log_info "Spawning isolated uninstaller unit: $UNINSTALL_UNIT"

                systemd-run \
                    --unit="${UNINSTALL_UNIT}" \
                    --property="Type=oneshot" \
                    --property="Conflicts=pulse-sensor-proxy.service" \
                    --collect \
                    --wait \
                    --quiet \
                    -- bash -c "$INSTALLER_PATH --uninstall --purge --quiet >> /var/log/pulse/sensor-proxy/uninstall.log 2>&1" \
                    2>&1 | logger -t "$LOG_TAG" -p user.info

                UNINSTALL_EXIT=$?
                if [[ $UNINSTALL_EXIT -eq 0 ]]; then
                    log_info "Uninstaller completed successfully"
                else
                    log_error "Uninstaller failed with exit code $UNINSTALL_EXIT"
                    exit 1
                fi
            else
                log_warn "systemd-run not available, attempting direct uninstall (may fail)"
                bash "$INSTALLER_PATH" --uninstall --quiet >> /var/log/pulse/sensor-proxy/uninstall.log 2>&1 || \
                    log_error "Uninstaller failed - manual cleanup may be required"
            fi
        else
            log_warn "Installer not found at $INSTALLER_PATH, cannot run uninstaller"
            log_info "Manual cleanup required: systemctl stop pulse-sensor-proxy && systemctl disable pulse-sensor-proxy"
        fi

        log_info "Localhost cleanup initiated (uninstaller running in background)"
    else
        log_info "Cleaning up remote host: $HOST_CLEAN"

        # Try to use proxy's SSH key first (for standalone nodes), fall back to default
        PROXY_KEY="/var/lib/pulse-sensor-proxy/ssh/id_ed25519"
        SSH_CMD="ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5"

        if [[ -f "$PROXY_KEY" ]]; then
            log_info "Using proxy SSH key for cleanup"
            SSH_CMD="$SSH_CMD -i $PROXY_KEY"
        fi

        # Remove both pulse-managed-key and pulse-proxy-key entries from remote host
        CLEANUP_OUTPUT=$($SSH_CMD root@"$HOST_CLEAN" \
            "sed -i -e '/# pulse-managed-key\$/d' -e '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys && echo 'SUCCESS'" 2>&1)

        if echo "$CLEANUP_OUTPUT" | grep -q "SUCCESS"; then
            log_info "Successfully cleaned up SSH keys on $HOST_CLEAN"
        else
            # Check if this is a standalone node with forced commands (common case)
            if echo "$CLEANUP_OUTPUT" | grep -q "cpu_thermal\|coretemp\|k10temp"; then
                log_warn "Cannot cleanup standalone node $HOST_CLEAN (forced command prevents cleanup)"
                log_info "Standalone node keys are read-only (sensors -j) - low security risk"
                log_info "Manual cleanup: ssh root@$HOST_CLEAN \"sed -i '/# pulse-proxy-key\$/d' /root/.ssh/authorized_keys\""
            else
                log_error "Failed to clean up SSH keys on $HOST_CLEAN: $CLEANUP_OUTPUT"
                exit 1
            fi
        fi
    fi
fi

# Remove processing file on success
rm -f "$PROCESSING_FILE"

log_info "Cleanup completed successfully"
exit 0
CLEANUP_EOF

chmod +x "$CLEANUP_SCRIPT_PATH"
print_info "Cleanup script installed"

# Install systemd path unit
CLEANUP_PATH_UNIT="/etc/systemd/system/pulse-sensor-cleanup.path"
cat > "$CLEANUP_PATH_UNIT" << 'PATH_EOF'
[Unit]
Description=Watch for Pulse sensor cleanup requests
Documentation=https://github.com/rcourtman/Pulse

[Path]
# Watch for the cleanup request file
PathChanged=/var/lib/pulse-sensor-proxy/cleanup-request.json
# Also watch for modifications
PathModified=/var/lib/pulse-sensor-proxy/cleanup-request.json

[Install]
WantedBy=multi-user.target
PATH_EOF

# Install systemd service unit
CLEANUP_SERVICE_UNIT="/etc/systemd/system/pulse-sensor-cleanup.service"
cat > "$CLEANUP_SERVICE_UNIT" <<SERVICE_EOF
[Unit]
Description=Pulse Sensor Cleanup Service
Documentation=https://github.com/rcourtman/Pulse
After=network.target

[Service]
Type=oneshot
ExecStart=${CLEANUP_SCRIPT_PATH}
User=root
Group=root
WorkingDirectory=/var/lib/pulse-sensor-proxy

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-sensor-cleanup

# Security hardening (less restrictive than the proxy since we need SSH access and Proxmox config access)
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/pulse-sensor-proxy /root/.ssh /etc/pve /etc/systemd/system
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
PrivateTmp=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
LimitNOFILE=1024

[Install]
# This service is triggered by the .path unit, no need to enable it directly
SERVICE_EOF

# Enable and start the path unit
systemctl daemon-reload || true
systemctl enable pulse-sensor-cleanup.path
systemctl start pulse-sensor-cleanup.path

print_info "Cleanup system installed and enabled"

# Configure SSH keys for cluster temperature monitoring
print_info "Configuring proxy SSH access to cluster nodes..."

# Wait for proxy to generate SSH keys
PROXY_KEY_FILE="$SSH_DIR/id_ed25519.pub"
for i in {1..10}; do
    if [[ -f "$PROXY_KEY_FILE" ]]; then
        break
    fi
    sleep 1
done

if [[ ! -f "$PROXY_KEY_FILE" ]]; then
    print_error "Proxy SSH key not generated after 10 seconds"
    print_info "Check service logs: journalctl -u pulse-sensor-proxy -n 50"
    exit 1
fi

PROXY_PUBLIC_KEY=$(cat "$PROXY_KEY_FILE")
print_info "Proxy public key: ${PROXY_PUBLIC_KEY:0:50}..."

# Discover cluster nodes using get_cluster_node_names helper (prefers pvesh API, falls back to pvecm CLI)
# This prefers management IPs over corosync ring IPs for multi-network clusters
CLUSTER_NODES=""
while IFS= read -r nodename; do
    [[ -z "$nodename" ]] && continue
    # Try to resolve hostname to IP via getent (uses /etc/hosts, then DNS)
    resolved_ip=$(getent hosts "$nodename" 2>/dev/null | awk '{print $1; exit}')
    if [[ -n "$resolved_ip" ]]; then
        CLUSTER_NODES="${CLUSTER_NODES:+$CLUSTER_NODES }$resolved_ip"
    else
        # Fallback: try extracting IP directly from pvecm status for this node
        if command -v pvecm >/dev/null 2>&1; then
            status_ip=$(pvecm status 2>/dev/null | grep -E "0x[0-9a-f]+.*$nodename" | awk '{for(i=1;i<=NF;i++) if($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) {print $i; exit}}')
            if [[ -n "$status_ip" ]]; then
                CLUSTER_NODES="${CLUSTER_NODES:+$CLUSTER_NODES }$status_ip"
            fi
        fi
    fi
done < <(get_cluster_node_names 2>/dev/null || true)

# Fallback to pvecm status if get_cluster_node_names didn't yield results
if [[ -z "$CLUSTER_NODES" ]] && command -v pvecm >/dev/null 2>&1; then
    CLUSTER_NODES=$(pvecm status 2>/dev/null | grep -vEi "qdevice" | awk '/0x[0-9a-f]+.*[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {for(i=1;i<=NF;i++) if($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) print $i}' || true)
fi

if [[ -n "$CLUSTER_NODES" ]]; then
    print_info "Discovered cluster nodes: $(echo $CLUSTER_NODES | tr '\n' ' ')"

    # Configure SSH key with forced command restriction
    FORCED_CMD='command="/opt/pulse/sensor-proxy/bin/pulse-sensor-wrapper.sh",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty'
    AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY} # pulse-managed-key"

    # Track SSH key push results
    SSH_SUCCESS_COUNT=0
    SSH_FAILURE_COUNT=0
    declare -a SSH_FAILED_NODES=()
    LOCAL_IPS=$(hostname -I 2>/dev/null || echo "")
    LOCAL_HOSTNAMES="$(hostname 2>/dev/null || echo "") $(hostname -f 2>/dev/null || echo "")"
    LOCAL_HANDLED=false

    # Push key to each cluster node
    for node_ip in $CLUSTER_NODES; do
        print_info "Authorizing proxy key on node $node_ip..."

        IS_LOCAL=false
        # Check if node_ip matches any of the local IPs (exact match with word boundaries)
        for local_ip in $LOCAL_IPS; do
            if [[ "$node_ip" == "$local_ip" ]]; then
                IS_LOCAL=true
                break
            fi
        done
        if [[ " $LOCAL_HOSTNAMES " == *" $node_ip "* ]]; then
            IS_LOCAL=true
        fi
        if [[ "$node_ip" == "127.0.0.1" || "$node_ip" == "localhost" ]]; then
            IS_LOCAL=true
        fi

        if [[ "$IS_LOCAL" = true ]]; then
            configure_local_authorized_key "$AUTH_LINE"
            LOCAL_HANDLED=true
            ((SSH_SUCCESS_COUNT+=1))
            continue
        fi

        # Ensure wrapper compatibility on remote node (supports old installations)
        # Create symlink if old wrapper exists but new path doesn't
        ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
            "if [[ -f /usr/local/bin/pulse-sensor-wrapper.sh && ! -f /opt/pulse/sensor-proxy/bin/pulse-sensor-wrapper.sh ]]; then \
                mkdir -p /opt/pulse/sensor-proxy/bin && \
                ln -sf /usr/local/bin/pulse-sensor-wrapper.sh /opt/pulse/sensor-proxy/bin/pulse-sensor-wrapper.sh; \
            fi" 2>/dev/null || true

        # Extract just the key portion for matching (ssh-TYPE KEY_DATA)
        KEY_DATA=$(echo "$AUTH_LINE" | grep -oE 'ssh-[a-z0-9-]+ [A-Za-z0-9+/=]+')

        # Check if key already exists on remote node
        KEY_EXISTS=$(ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
            "grep -qF '$KEY_DATA' /root/.ssh/authorized_keys 2>/dev/null && echo 'yes' || echo 'no'" 2>/dev/null)

        if [[ "$KEY_EXISTS" == "yes" ]]; then
            print_success "SSH key already configured on $node_ip"
            ((SSH_SUCCESS_COUNT+=1))
        else
            # Add new key with forced command (appends, does not remove existing keys)
            SSH_ERROR=$(ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=5 root@"$node_ip" \
                "echo '${AUTH_LINE}' >> /root/.ssh/authorized_keys" 2>&1)
            if [[ $? -eq 0 ]]; then
                print_success "SSH key configured on $node_ip"
                ((SSH_SUCCESS_COUNT+=1))
            else
                print_warn "Failed to configure SSH key on $node_ip"
                ((SSH_FAILURE_COUNT+=1))
                SSH_FAILED_NODES+=("$node_ip")
                # Log detailed error for debugging
                if [[ -n "$SSH_ERROR" ]]; then
                    print_info "  Error details: $(echo "$SSH_ERROR" | head -1)"
                fi
            fi
        fi
    done

    # Print summary
    print_info ""
    print_info "SSH key configuration summary:"
    print_info "  ✓ Success: $SSH_SUCCESS_COUNT node(s)"
    if [[ $SSH_FAILURE_COUNT -gt 0 ]]; then
        print_warn "  ✗ Failed: $SSH_FAILURE_COUNT node(s) - ${SSH_FAILED_NODES[*]}"
        print_info ""
        print_info "To retry failed nodes, re-run this script or manually run:"
        print_info "  ssh root@<node> 'echo \"${AUTH_LINE}\" >> /root/.ssh/authorized_keys'"
    fi
    if [[ "$LOCAL_HANDLED" = false ]]; then
        configure_local_authorized_key "$AUTH_LINE"
        ((SSH_SUCCESS_COUNT+=1))
    fi

    # Add discovered cluster nodes to config file for allowlist validation
    print_info "Updating proxy configuration with discovered cluster nodes..."
    # Collect only IPs (hostnames are not used for SSH temperature collection)
    all_nodes=()
    for node_ip in $CLUSTER_NODES; do
        all_nodes+=("$node_ip")
    done
    if [[ ${#CONTROL_PLANE_ALLOWED_NODE_LIST[@]} -gt 0 ]]; then
        all_nodes+=("${CONTROL_PLANE_ALLOWED_NODE_LIST[@]}")
    fi
    # Use helper function to safely update allowed_nodes (prevents duplicates on re-run)
    if ! update_allowed_nodes "Cluster nodes (auto-discovered during installation)" "${all_nodes[@]}"; then
        print_error "Failed to update allowed_nodes list"
        exit 1
    fi
else
    # No cluster found - configure standalone node
    print_info "No cluster detected, configuring standalone node..."

    # Configure SSH key with forced command restriction
    FORCED_CMD='command="/opt/pulse/sensor-proxy/bin/pulse-sensor-wrapper.sh",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty'
    AUTH_LINE="${FORCED_CMD} ${PROXY_PUBLIC_KEY} # pulse-managed-key"

    print_info "Authorizing proxy key on localhost..."
    configure_local_authorized_key "$AUTH_LINE"
    print_info ""
    print_info "Standalone node configuration complete"

    # Add localhost to config file for allowlist validation
    print_info "Updating proxy configuration for standalone mode..."
    LOCAL_IPS=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -v '^$' || echo "127.0.0.1")
    # Collect all local IPs and localhost variants into array
    all_nodes=()
    for local_ip in $LOCAL_IPS; do
        all_nodes+=("$local_ip")
    done
    # Always include localhost variants
    all_nodes+=("127.0.0.1" "localhost")
    if [[ ${#CONTROL_PLANE_ALLOWED_NODE_LIST[@]} -gt 0 ]]; then
        all_nodes+=("${CONTROL_PLANE_ALLOWED_NODE_LIST[@]}")
    fi
    # Use helper function to safely update allowed_nodes (prevents duplicates on re-run)
    if ! update_allowed_nodes "Standalone node configuration (auto-configured during installation)" "${all_nodes[@]}"; then
        print_error "Failed to update allowed_nodes list"
        exit 1
    fi
fi

cleanup_inline_allowed_nodes

# Container-specific configuration (skip for standalone mode or if container not on this node)
if [[ "$STANDALONE" == false && "$CONTAINER_ON_THIS_NODE" == true ]]; then
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Secure Container Communication Setup"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Setting up secure socket mount for temperature monitoring:"
    echo "  • Container communicates with host proxy via Unix socket"
    echo "  • No SSH keys exposed inside container (enhanced security)"
    echo "  • Proxy on host manages all temperature collection"
    echo ""

    print_info "Configuring socket bind mount..."
    MOUNT_TARGET="/mnt/pulse-proxy"
    HOST_SOCKET_SOURCE="/run/pulse-sensor-proxy"
    LXC_CONFIG="/etc/pve/lxc/${CTID}.conf"
    LOCAL_MOUNT_ENTRY="lxc.mount.entry: ${HOST_SOCKET_SOURCE} mnt/pulse-proxy none bind,create=dir 0 0"

    mkdir -p "$HOST_SOCKET_SOURCE"

    # Back up container config before modifying
    # Use timeout since /etc/pve is a FUSE filesystem that can hang
    LXC_CONFIG_BACKUP=$(mktemp)
    if ! timeout 10 cp "$LXC_CONFIG" "$LXC_CONFIG_BACKUP" 2>/dev/null; then
        print_warn "Could not back up container config (may not exist yet or pmxcfs slow)"
        LXC_CONFIG_BACKUP=""
    fi

    MOUNT_UPDATED=false
    CT_RUNNING=false
    SKIP_CONTAINER_POST_STEPS=false
    if timeout 5 pct status "$CTID" 2>/dev/null | grep -q "running"; then
        CT_RUNNING=true
    fi

    # /etc/pve is a FUSE filesystem (pmxcfs) - direct sed/echo don't work reliably
    # Must use temp file and copy back to trigger cluster sync
    # Also, config file contains snapshots sections - only modify main section (before first [)
    TEMP_CONFIG=$(mktemp)
    print_info "Reading container config from $LXC_CONFIG..."
    if ! timeout 10 cp "$LXC_CONFIG" "$TEMP_CONFIG" 2>/dev/null; then
        print_error "Timed out or failed reading container config from $LXC_CONFIG"
        print_error "The Proxmox cluster filesystem (pmxcfs) may be slow or unresponsive."
        print_error "Try: pvecm status  # to check cluster health"
        rm -f "$TEMP_CONFIG"
        exit 1
    fi

    # Extract line number where snapshots start (first line starting with [)
    # Note: grep returns 1 if no match, which fails with pipefail - use || true to suppress
    SNAPSHOT_START=$(grep -n '^\[' "$TEMP_CONFIG" | head -1 | cut -d: -f1 || true)

    if grep -Eq '^mp[0-9]+:.*pulse-sensor-proxy|^mp[0-9]+:.*mnt/pulse-proxy' "$TEMP_CONFIG" 2>/dev/null; then
        print_info "Removing mp mounts for pulse-sensor-proxy to keep snapshots and migrations working"
        if [ -n "$SNAPSHOT_START" ]; then
            # Only modify main section (before snapshots)
            sed -i "1,$((SNAPSHOT_START-1)) { /^mp[0-9]\+:.*pulse-sensor-proxy/d; /^mp[0-9]\+:.*mnt\/pulse-proxy/d }" "$TEMP_CONFIG"
        else
            sed -i '/^mp[0-9]\+:.*pulse-sensor-proxy/d; /^mp[0-9]\+:.*mnt\/pulse-proxy/d' "$TEMP_CONFIG"
        fi
        MOUNT_UPDATED=true
    fi

    if grep -q "^lxc.mount.entry: .*/pulse-sensor-proxy" "$TEMP_CONFIG" 2>/dev/null; then
        if ! grep -qxF "$LOCAL_MOUNT_ENTRY" "$TEMP_CONFIG"; then
            print_info "Updating existing lxc.mount.entry for pulse-sensor-proxy"
            if [ -n "$SNAPSHOT_START" ]; then
                sed -i "1,$((SNAPSHOT_START-1)) { s#^lxc.mount.entry: .*pulse-sensor-proxy.*#${LOCAL_MOUNT_ENTRY}# }" "$TEMP_CONFIG"
            else
                sed -i "s#^lxc.mount.entry: .*pulse-sensor-proxy.*#${LOCAL_MOUNT_ENTRY}#" "$TEMP_CONFIG"
            fi
            MOUNT_UPDATED=true
        else
            print_info "Container already has migration-safe lxc.mount.entry for proxy"
        fi
    else
        print_info "Adding lxc.mount.entry for pulse-sensor-proxy"
        # Insert before snapshot section if it exists, otherwise append
        if [ -n "$SNAPSHOT_START" ]; then
            sed -i "${SNAPSHOT_START}i ${LOCAL_MOUNT_ENTRY}" "$TEMP_CONFIG"
        else
            echo "$LOCAL_MOUNT_ENTRY" >> "$TEMP_CONFIG"
        fi
        MOUNT_UPDATED=true
    fi

    # Copy back to trigger pmxcfs sync
    if [[ "$MOUNT_UPDATED" = true ]]; then
        print_info "Writing updated config to $LXC_CONFIG..."
        if ! timeout 10 cp "$TEMP_CONFIG" "$LXC_CONFIG" 2>/dev/null; then
            print_error "Timed out or failed writing container config to $LXC_CONFIG"
            print_error "The Proxmox cluster filesystem (pmxcfs) may be slow or unresponsive."
            rm -f "$TEMP_CONFIG"
            exit 1
        fi
    fi
    rm -f "$TEMP_CONFIG"

    if ! timeout 10 pct config "$CTID" 2>/dev/null | grep -qxF "$LOCAL_MOUNT_ENTRY"; then
        print_error "Failed to persist migration-safe socket mount in container config"
        if [ -n "$LXC_CONFIG_BACKUP" ] && [ -f "$LXC_CONFIG_BACKUP" ]; then
            print_warn "Rolling back container configuration changes..."
            timeout 10 cp "$LXC_CONFIG_BACKUP" "$LXC_CONFIG" 2>/dev/null || true
            rm -f "$LXC_CONFIG_BACKUP"
        fi
        exit 1
    fi
    print_info "✓ Mount configuration recorded in container config"

    if [[ "$MOUNT_UPDATED" = true ]]; then
        if [[ "$SKIP_RESTART" = true ]]; then
            if [[ "$CT_RUNNING" = true ]]; then
                print_info "Skipping container restart (--skip-restart provided). Changes apply on next restart."
            else
                print_info "Skipping automatic container start (--skip-restart provided)."
            fi
        else
            print_info "Restarting container to activate secure communication..."
            if [[ "$CT_RUNNING" = true ]]; then
                pct stop "$CTID" && sleep 2 && pct start "$CTID"
            else
                pct start "$CTID"
            fi
            sleep 5
            CT_RUNNING=true
        fi
    fi

    # Verify socket directory and file inside container
    if [[ "$SKIP_RESTART" = true && "$CT_RUNNING" = true && "$MOUNT_UPDATED" = true ]]; then
        print_warn "Skipping socket verification until container $CTID is restarted."
        print_warn "Please restart container and verify socket manually:"
        print_warn "  pct stop $CTID && sleep 2 && pct start $CTID"
        print_warn "  pct exec $CTID -- test -S ${MOUNT_TARGET}/pulse-sensor-proxy.sock && echo 'Socket OK'"
    elif [[ "$SKIP_RESTART" = true && "$CT_RUNNING" = false ]]; then
        print_warn "Socket verification deferred. Start container $CTID and run:"
        print_warn "  pct exec $CTID -- test -S ${MOUNT_TARGET}/pulse-sensor-proxy.sock && echo 'Socket OK'"
        SKIP_CONTAINER_POST_STEPS=true
    elif [[ "$CT_RUNNING" = false ]]; then
        print_warn "Container $CTID is stopped. Start it to verify the pulse-sensor-proxy socket:"
        print_warn "  pct start $CTID && pct exec $CTID -- test -S ${MOUNT_TARGET}/pulse-sensor-proxy.sock && echo 'Socket OK'"
        SKIP_CONTAINER_POST_STEPS=true
    else
        if [[ ! -S "$SOCKET_PATH" ]]; then
            print_warn "Host proxy socket not available yet; deferring container verification until service starts."
            DEFER_SOCKET_VERIFICATION=true
        else
            print_info "Verifying secure communication channel..."
            if pct exec "$CTID" -- test -S "${MOUNT_TARGET}/pulse-sensor-proxy.sock"; then
                print_info "✓ Secure socket communication ready"
                # Clean up backup since verification succeeded
                [ -n "$LXC_CONFIG_BACKUP" ] && rm -f "$LXC_CONFIG_BACKUP"
            else
                print_error "Socket not visible at ${MOUNT_TARGET}/pulse-sensor-proxy.sock"
                print_error "Mount configuration verified but socket not accessible in container"
                print_error "This indicates a mount or restart issue"

                # Rollback container config changes
                if [ -n "$LXC_CONFIG_BACKUP" ] && [ -f "$LXC_CONFIG_BACKUP" ]; then
                    print_warn "Rolling back container configuration changes..."
                    cp "$LXC_CONFIG_BACKUP" "$LXC_CONFIG"
                    rm -f "$LXC_CONFIG_BACKUP"
                    print_info "Container configuration restored to previous state"
                fi
                exit 1
            fi
        fi
    fi

    if [[ "$SKIP_CONTAINER_POST_STEPS" != true ]]; then
        # Configure Pulse backend environment override inside container
        print_info "Configuring Pulse to use proxy..."

        # Always make sure the Pulse .env file contains the proxy socket override.
        configure_container_proxy_env

        if ! pct exec "$CTID" -- systemctl status pulse >/dev/null 2>&1; then
            print_warn "Pulse service not found in container $CTID; proxy socket configured but service restart deferred."
            print_info "Install or restart Pulse inside the container to enable temperature monitoring."
        else
            pct exec "$CTID" -- bash -lc "mkdir -p /etc/systemd/system/pulse.service.d"
            pct exec "$CTID" -- bash -lc "cat <<'EOF' >/etc/systemd/system/pulse.service.d/10-pulse-proxy.conf
[Service]
Environment=PULSE_SENSOR_PROXY_SOCKET=${MOUNT_TARGET}/pulse-sensor-proxy.sock
EOF"
            pct exec "$CTID" -- systemctl daemon-reload || true

            # Restart Pulse service to apply the new environment variable
            if pct exec "$CTID" -- systemctl is-active --quiet pulse 2>/dev/null; then
                print_info "Restarting Pulse service to apply configuration..."
                pct exec "$CTID" -- systemctl restart pulse
                sleep 2
                print_success "Pulse service restarted with proxy configuration"
            fi
        fi

        # Check for and remove legacy SSH keys from container
        print_info "Checking for legacy SSH keys in container..."
        LEGACY_KEYS_FOUND=false
        for key_type in id_rsa id_dsa id_ecdsa id_ed25519; do
            if pct exec "$CTID" -- test -f "/root/.ssh/$key_type" 2>/dev/null; then
                LEGACY_KEYS_FOUND=true
                if [ "$QUIET" != true ]; then
                    print_warn "Found legacy SSH key: /root/.ssh/$key_type"
                fi
                pct exec "$CTID" -- rm -f "/root/.ssh/$key_type" "/root/.ssh/${key_type}.pub"
            print_info "  Removed /root/.ssh/$key_type"
            fi
        done

        if [ "$LEGACY_KEYS_FOUND" = true ] && [ "$QUIET" != true ]; then
            print_info ""
            print_info "Legacy SSH keys removed from container for security"
            print_info ""
        fi
    else
        if [ -n "$LXC_CONFIG_BACKUP" ] && [ -f "$LXC_CONFIG_BACKUP" ]; then
            rm -f "$LXC_CONFIG_BACKUP"
        fi
        print_warn "Skipping container-side configuration until container $CTID is running."
    fi

    # Test proxy status
    print_info "Testing proxy status..."
    if systemctl is-active --quiet pulse-sensor-proxy; then
        print_info "${GREEN}✓${NC} pulse-sensor-proxy is running"
    else
        print_error "pulse-sensor-proxy is not running"
        print_info "Check logs: journalctl -u pulse-sensor-proxy -n 50"
        exit 1
    fi
fi  # End of container-specific configuration

if [[ "$SKIP_SELF_HEAL_SETUP" == "true" || "$SKIP_SELF_HEAL_SETUP" == "1" ]]; then
    print_info "Skipping self-heal safeguards during self-heal run"
else
    # Install self-heal safeguards to keep proxy available
    print_info "Configuring self-heal safeguards..."
    if ! cache_installer_for_self_heal; then
        if [[ -n "$INSTALLER_CACHE_REASON" ]]; then
            print_warn "Unable to cache installer script for self-heal (${INSTALLER_CACHE_REASON})"
        else
            print_warn "Unable to cache installer script for self-heal"
        fi
    fi

    cat > "$SELFHEAL_SCRIPT" <<'EOF'
#!/bin/bash
set -euo pipefail

SERVICE="pulse-sensor-proxy"
BINARY_PATH="/opt/pulse/sensor-proxy/bin/pulse-sensor-proxy"
INSTALLER="/opt/pulse/sensor-proxy/install-sensor-proxy.sh"
CTID_FILE="/etc/pulse-sensor-proxy/ctid"
PENDING_FILE="/etc/pulse-sensor-proxy/pending-control-plane.env"
TOKEN_FILE="/etc/pulse-sensor-proxy/.pulse-control-token"
CONFIG_FILE="/etc/pulse-sensor-proxy/config.yaml"
LOG_TAG="pulse-sensor-proxy-selfheal"

log() {
    logger -t "$LOG_TAG" "$1"
}

sanitize_allowed_nodes() {
    # Phase 2: Use config CLI instead of Python manipulation
    if [[ ! -f "$CONFIG_FILE" ]]; then
        return
    fi

    if [[ ! -x "$BINARY_PATH" ]]; then
        log "Binary not available; skipping sanitization"
        return
    fi

    # Use CLI to atomically migrate any inline blocks to file mode
    if "$BINARY_PATH" config migrate-to-file --config "$CONFIG_FILE" 2>&1 | grep -q "Migration complete"; then
        log "Migrated inline allowed_nodes to file mode"
    fi
}

attempt_control_plane_reconcile() {
    if [[ ! -f "$PENDING_FILE" ]]; then
        return
    fi
    if [[ -f "$TOKEN_FILE" ]]; then
        return
    fi
    if [[ ! -x "$INSTALLER" ]]; then
        return
    fi

    # shellcheck disable=SC1090
    source "$PENDING_FILE" || return
    if [[ -z "${PENDING_PULSE_SERVER:-}" ]]; then
        return
    fi

    cmd=("$INSTALLER" "--skip-restart" "--quiet" "--pulse-server" "${PENDING_PULSE_SERVER}")
    if [[ "${PENDING_STANDALONE:-false}" == "true" ]]; then
        cmd+=("--standalone")
        if [[ "${PENDING_HTTP_MODE:-false}" == "true" ]]; then
            cmd+=("--http-mode")
            if [[ -n "${PENDING_HTTP_ADDR:-}" ]]; then
                cmd+=("--http-addr" "${PENDING_HTTP_ADDR}")
            fi
        fi
    else
        if [[ -f "$CTID_FILE" ]]; then
            cmd+=("--ctid" "$(cat "$CTID_FILE")")
        else
            log "CTID file missing; cannot reconcile control plane"
            return
        fi
    fi

    if PULSE_SENSOR_PROXY_SELFHEAL=1 bash "$INSTALLER" "${cmd[@]}"; then
        rm -f "$PENDING_FILE"
        sanitize_allowed_nodes
    else
        log "Control-plane reconciliation failed"
    fi
}

sanitize_allowed_nodes

if ! command -v systemctl >/dev/null 2>&1; then
    exit 0
fi

# Early exit if service is healthy - no work needed
if systemctl is-active --quiet "${SERVICE}.service" 2>/dev/null; then
    # Service is running - only do control plane reconcile if needed, skip everything else
    attempt_control_plane_reconcile
    exit 0
fi

if ! systemctl list-unit-files 2>/dev/null | grep -q "^${SERVICE}\\.service"; then
    if [[ -x "$INSTALLER" && -f "$CTID_FILE" ]]; then
        log "Service unit missing; attempting reinstall"
        if PULSE_SENSOR_PROXY_SELFHEAL=1 bash "$INSTALLER" --ctid "$(cat "$CTID_FILE")" --skip-restart --quiet; then
            sanitize_allowed_nodes
        else
            log "Reinstall attempt failed"
        fi
    fi
    exit 0
fi

if ! systemctl is-active --quiet "${SERVICE}.service"; then
    systemctl start "${SERVICE}.service" || true
    sleep 2
fi

if ! systemctl is-active --quiet "${SERVICE}.service"; then
    if [[ -x "$INSTALLER" && -f "$CTID_FILE" ]]; then
        log "Service failed to start; attempting reinstall"
        if PULSE_SENSOR_PROXY_SELFHEAL=1 bash "$INSTALLER" --ctid "$(cat "$CTID_FILE")" --skip-restart --quiet; then
            sanitize_allowed_nodes
        else
            log "Reinstall attempt failed"
        fi
        systemctl start "${SERVICE}.service" || true
    fi
fi

attempt_control_plane_reconcile
EOF
chmod 0755 "$SELFHEAL_SCRIPT"

cat > "$SELFHEAL_SERVICE_UNIT" <<EOF
[Unit]
Description=Pulse Sensor Proxy Self-Heal
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=${SELFHEAL_SCRIPT}
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

cat > "$SELFHEAL_TIMER_UNIT" <<'EOF'
[Unit]
Description=Ensure pulse-sensor-proxy stays installed and running

[Timer]
OnBootSec=2min
OnUnitActiveSec=5min
Unit=pulse-sensor-proxy-selfheal.service

[Install]
WantedBy=timers.target
EOF

    systemctl daemon-reload
    systemctl enable --now pulse-sensor-proxy-selfheal.timer >/dev/null 2>&1 || true
    if [[ -f "$PENDING_CONTROL_PLANE_FILE" ]]; then
        if [[ "$QUIET" != true ]]; then
            print_info "Pending control-plane sync detected; will retry in background..."
        fi
        # Use --no-block to avoid hanging the installer if sync takes a long time
        systemctl start --no-block pulse-sensor-proxy-selfheal.service >/dev/null 2>&1 || true
    fi
fi

if [ "$QUIET" = true ]; then
    print_success "pulse-sensor-proxy installed and running"
else
    print_info "${GREEN}Installation complete!${NC}"
    print_info ""
    print_info "Temperature monitoring will use the secure host-side proxy"
    print_info ""

    # Only show Docker configuration instructions if Pulse is actually running in Docker on this host
    IS_PULSE_DOCKER=false
    if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^pulse$'; then
        IS_PULSE_DOCKER=true
    fi

    if [[ "$STANDALONE" == true ]] && [[ "$IS_PULSE_DOCKER" == true ]]; then
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "  Docker Container Configuration Required"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        print_info "${YELLOW}IMPORTANT:${NC} If Pulse is running in Docker, add this bind mount to your docker-compose.yml:"
        echo ""
        echo "  volumes:"
        echo "    - pulse-data:/data"
        echo "    - /run/pulse-sensor-proxy:/run/pulse-sensor-proxy:ro"
        echo ""
        print_info "Then restart your Pulse container:"
        echo "  docker-compose down && docker-compose up -d"
        echo ""
        print_info "Or if using Docker directly:"
        echo "  docker restart pulse"
        echo ""
    fi

    # Check if Pulse needs to be restarted to pick up the proxy registration
    PULSE_RESTART_CMD=""
    if systemctl is-active --quiet pulse 2>/dev/null; then
        PULSE_RESTART_CMD="systemctl restart pulse"
    elif systemctl is-active --quiet pulse-hot-dev 2>/dev/null; then
        PULSE_RESTART_CMD="systemctl restart pulse-hot-dev"
    elif command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^pulse$'; then
        PULSE_RESTART_CMD="docker restart pulse"
    fi

    if [[ -n "$PULSE_RESTART_CMD" ]]; then
        if [[ "$RESTART_PULSE" == true ]]; then
            echo ""
            print_info "Restarting Pulse to enable temperature monitoring..."
            if eval "$PULSE_RESTART_CMD"; then
                sleep 3
                print_success "Pulse restarted successfully - temperature monitoring is now active"
            else
                print_warn "Failed to restart Pulse automatically. Please restart manually:"
                echo "  $PULSE_RESTART_CMD"
            fi
            echo ""
        else
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "  Pulse Restart Required"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            print_info "${YELLOW}IMPORTANT:${NC} Restart Pulse to enable temperature monitoring:"
            echo "  sudo $PULSE_RESTART_CMD"
            echo ""
            print_info "Or add --restart-pulse flag to restart automatically:"
            echo "  curl ... | bash -s -- ... --restart-pulse"
            echo ""
        fi
    fi

    print_info "To check proxy status:"
    print_info "  systemctl status pulse-sensor-proxy"

    if [[ "$STANDALONE" == true ]]; then
        echo ""
        print_info "After restarting Pulse, verify the socket is accessible:"
        print_info "  docker exec pulse ls -l /run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
        echo ""
        print_info "Check Pulse logs for temperature proxy detection:"
        print_info "  docker logs pulse | grep -i 'temperature.*proxy'"
        echo ""
        print_info "For detailed documentation, see:"
        print_info "  https://github.com/rcourtman/Pulse/blob/main/docs/TEMPERATURE_MONITORING.md"
    fi
fi

exit 0
