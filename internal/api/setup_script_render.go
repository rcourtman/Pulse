package api

import (
	"fmt"
	"strings"
	"time"
)

type setupScriptRenderContext struct {
	ServerName         string
	PulseURL           string
	ServerHost         string
	SetupToken         string
	TokenName          string
	TokenMatchPrefix   string
	StoragePerms       string
	StorageRepairPerms string
	SensorsPublicKey   string
	Artifact           setupScriptInstallArtifact
}

func deriveSetupScriptServerName(serverHost string) string {
	trimmedHost := strings.TrimSpace(serverHost)
	if trimmedHost == "" {
		return "your-server"
	}
	if strings.Contains(trimmedHost, "://") {
		parts := strings.Split(trimmedHost, "://")
		if len(parts) > 1 {
			return strings.Split(parts[1], ":")[0]
		}
	}
	return strings.Split(trimmedHost, ":")[0]
}

func renderSetupScript(serverType string, ctx setupScriptRenderContext) string {
	if strings.TrimSpace(serverType) == "pve" {
		return renderPVESetupScript(ctx)
	}
	return renderPBSSetupScript(ctx)
}

func renderPVESetupScript(ctx setupScriptRenderContext) string {
	return fmt.Sprintf(`#!/bin/bash
# Pulse Monitoring Setup Script for %s
# Generated: %s

echo "============================================"
echo "  Pulse Monitoring Setup for Proxmox VE"
echo "============================================"
echo ""

PULSE_URL="%s"
SERVER_HOST="%s"
HOST_URL="$SERVER_HOST"
PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-%s}"
SETUP_TOKEN_INVALID=false
TOKEN_NAME="%s"
TOKEN_MATCH_PREFIX="%s"
PULSE_TOKEN_ID="pulse-monitor@pve!${TOKEN_NAME}"
SETUP_SCRIPT_URL="%s"
PULSE_BOOTSTRAP_COMMAND_WITH_ENV=%s

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "Root privileges required. Run as root (su -) and retry."
   exit 1
fi

resolve_authorized_keys_path() {
    local auth_keys="/root/.ssh/authorized_keys"
    local resolved=""

    if [ -L "$auth_keys" ]; then
        resolved="$(readlink -f "$auth_keys" 2>/dev/null || true)"
        if [ -n "$resolved" ]; then
            auth_keys="$resolved"
        fi
    fi

    printf '%%s\n' "$auth_keys"
}

install_authorized_keys_file() {
    local tmp_auth_keys="$1"
    local auth_keys="$2"

    if [ ! -f "$tmp_auth_keys" ]; then
        return 1
    fi

    chmod --reference="$auth_keys" "$tmp_auth_keys" 2>/dev/null || chmod 600 "$tmp_auth_keys"
    chown --reference="$auth_keys" "$tmp_auth_keys" 2>/dev/null || true

    if mv -f "$tmp_auth_keys" "$auth_keys" 2>/dev/null; then
        return 0
    fi

    if cp -f "$tmp_auth_keys" "$auth_keys" 2>/dev/null; then
        rm -f "$tmp_auth_keys"
        return 0
    fi

    rm -f "$tmp_auth_keys"
    return 1
}

# Detect environment (Proxmox host vs LXC guest)
detect_environment() {
    if command -v pveum >/dev/null 2>&1 && command -v pveversion >/dev/null 2>&1; then
        echo "pve_host"
        return
    fi

    if [ -f /proc/1/cgroup ] && grep -qE '/(lxc|machine\.slice/machine-lxc)' /proc/1/cgroup 2>/dev/null; then
        echo "lxc_guest"
        return
    fi

    if command -v systemd-detect-virt >/dev/null 2>&1; then
        if systemd-detect-virt -q -c 2>/dev/null; then
            local virt_type
            virt_type=$(systemd-detect-virt -c 2>/dev/null | tr '[:upper:]' '[:lower:]')
            if echo "$virt_type" | grep -q "lxc"; then
                echo "lxc_guest"
                return
            fi
        fi
    fi

    echo "unknown"
}

detect_lxc_ctid() {
    local ctid=""
    if [ -f /proc/1/cgroup ]; then
        ctid=$(sed 's/\\x2d/-/g' /proc/1/cgroup 2>/dev/null | grep -Eo '(lxc|machine-lxc)-[0-9]+' | tail -n1 | grep -Eo '[0-9]+' | tail -n1)
        if [ -n "$ctid" ]; then
            echo "$ctid"
            return
        fi
    fi

    if command -v hostname >/dev/null 2>&1; then
        ctid=$(hostname 2>/dev/null)
        if echo "$ctid" | grep -qE '^[0-9]+$'; then
            echo "$ctid"
            return
        fi
    fi

    echo ""
}

ENVIRONMENT=$(detect_environment)

case "$ENVIRONMENT" in
    pve_host)
        echo "Detected Proxmox VE host environment."
        echo ""
        ;;
    lxc_guest)
        echo "Detected Proxmox LXC container environment."
        echo ""
        echo "Run this script on the Proxmox host:"
        echo "  $PULSE_BOOTSTRAP_COMMAND_WITH_ENV"
        echo ""
        exit 1
        ;;
    *)
        echo "This script requires Proxmox host tooling (pveum)."
        echo ""
        echo "Run on your Proxmox host:"
        echo "  $PULSE_BOOTSTRAP_COMMAND_WITH_ENV"
        echo ""
        echo "This setup flow must run on the Proxmox host so Pulse can create"
        echo "the monitoring token and continue registration on the canonical path."
        echo ""
        exit 1
        ;;
esac

trim_field() {
    sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

pulse_pve_user_exists() {
    pveum user list 2>/dev/null | grep -F "pulse-monitor@pve" >/dev/null 2>&1
}

pulse_pve_matching_tokens() {
    local user_id="$1"
    pveum user token list "$user_id" 2>/dev/null |
        awk -F'│' 'NR>3 {print $2}' |
        trim_field |
        grep -E "^${TOKEN_MATCH_PREFIX}(-[0-9]+)?$" || true
}

pulse_pve_token_exists() {
    pulse_pve_matching_tokens pulse-monitor@pve | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1
}

pulse_pve_token_expire_label() {
    local expire
    expire=$(pveum user token list pulse-monitor@pve 2>/dev/null |
        awk -F'│' -v token="$TOKEN_NAME" '
            NR>3 {
                name=$2
                expire_field=$3
                gsub(/^[ \t]+|[ \t]+$/, "", name)
                gsub(/^[ \t]+|[ \t]+$/, "", expire_field)
                if (name == token) {
                    print expire_field
                    exit
                }
            }')
    if [ -z "$expire" ]; then
        echo "unknown"
    else
        echo "$expire"
    fi
}

pulse_pve_acl_has() {
    local acl_path="$1"
    local subject="$2"
    local role="$3"
    if pveum acl list 2>/dev/null |
        awk -F'│' -v acl_path="$acl_path" -v subject="$subject" -v role="$role" '
            NR>3 {
                path_field=$2
                subject_field=$3
                role_field=$4
                gsub(/^[ \t]+|[ \t]+$/, "", path_field)
                gsub(/^[ \t]+|[ \t]+$/, "", subject_field)
                gsub(/^[ \t]+|[ \t]+$/, "", role_field)
                if (path_field == acl_path && subject_field == subject && role_field == role) {
                    found=1
                }
            }
            END { exit found ? 0 : 1 }'; then
        return 0
    fi

    pveum acl list 2>/dev/null | grep -F "$acl_path" | grep -F "$subject" | grep -F "$role" >/dev/null 2>&1
}

print_pve_acl_status() {
    local acl_path="$1"
    local subject="$2"
    local role="$3"
    local label="$4"
    if pulse_pve_acl_has "$acl_path" "$subject" "$role"; then
        echo "  ✓ $label"
    else
        echo "  ⚠ Missing: $label"
    fi
}

configure_pve_pulse_monitor_role() {
    local apply_token_acl="$1"
    local HAS_VM_MONITOR=false
    local HAS_VM_GUEST_AGENT_AUDIT=false
    local HAS_VM_GUEST_AGENT_FILE_READ=false
    local HAS_SYS_AUDIT=false
    local PVE_VERSION=""
    local EXTRA_PRIVS=()

    if pveum role list 2>/dev/null | grep -q "VM.Monitor" ||
       pveum role add TestMonitor -privs VM.Monitor 2>/dev/null; then
        HAS_VM_MONITOR=true
        pveum role delete TestMonitor 2>/dev/null || true
    fi

    if pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then
        HAS_VM_GUEST_AGENT_AUDIT=true
    elif pveum role add TestGuestAgentAudit -privs VM.GuestAgent.Audit 2>/dev/null; then
        HAS_VM_GUEST_AGENT_AUDIT=true
        pveum role delete TestGuestAgentAudit 2>/dev/null || true
    fi

    if pveum role list 2>/dev/null | grep -q "VM.GuestAgent.FileRead"; then
        HAS_VM_GUEST_AGENT_FILE_READ=true
    elif pveum role add TestGuestAgentFileRead -privs VM.GuestAgent.FileRead 2>/dev/null; then
        HAS_VM_GUEST_AGENT_FILE_READ=true
        pveum role delete TestGuestAgentFileRead 2>/dev/null || true
    fi

    if pveum role list 2>/dev/null | grep -q "Sys.Audit"; then
        HAS_SYS_AUDIT=true
    elif pveum role add TestSysAudit -privs Sys.Audit 2>/dev/null; then
        HAS_SYS_AUDIT=true
        pveum role delete TestSysAudit 2>/dev/null || true
    fi

    if command -v pveversion >/dev/null 2>&1; then
        PVE_VERSION=$(pveversion --verbose 2>/dev/null | grep "pve-manager" | awk -F'/' '{print $2}' | cut -d'.' -f1)
    fi

    if [ "$HAS_SYS_AUDIT" = true ]; then
        EXTRA_PRIVS+=("Sys.Audit")
    fi

    if [ "$HAS_VM_GUEST_AGENT_AUDIT" = true ]; then
        EXTRA_PRIVS+=("VM.GuestAgent.Audit")
        if [ "$HAS_VM_GUEST_AGENT_FILE_READ" = true ]; then
            EXTRA_PRIVS+=("VM.GuestAgent.FileRead")
        fi
    elif [ "$HAS_VM_MONITOR" = true ]; then
        EXTRA_PRIVS+=("VM.Monitor")
    fi

    if [ ${#EXTRA_PRIVS[@]} -gt 0 ]; then
        local PRIV_STRING
        PRIV_STRING="$(IFS=,; echo "${EXTRA_PRIVS[*]}")"

        if pveum role modify PulseMonitor -privs "$PRIV_STRING" 2>/dev/null || pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null; then
            pveum aclmod / -user pulse-monitor@pve -role PulseMonitor
            if [ "$apply_token_acl" = true ]; then
                pveum aclmod / -token "$PULSE_TOKEN_ID" -role PulseMonitor
            fi
            echo "  • Applied privileges: $PRIV_STRING"
        else
            echo "  • Failed to configure PulseMonitor role with: $PRIV_STRING"
            echo "    Assign these privileges manually if Pulse reports permission errors."
        fi
    else
        echo "  • No additional privileges detected. Pulse may show limited VM metrics."
        if [ -n "$PVE_VERSION" ]; then
            echo "    Detected PVE major version: $PVE_VERSION"
        fi
    fi
}

run_pve_setup_doctor() {
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Pulse Proxmox Setup Doctor"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Checking Pulse-managed Proxmox user, token, ACLs, and legacy token drift."
    echo "This mode does not rotate the API token."
    echo ""

    local token_exists=false
    if pulse_pve_user_exists; then
        echo "  ✓ Monitoring user exists: pulse-monitor@pve"
    else
        echo "  ⚠ Missing monitoring user: pulse-monitor@pve"
        echo "    Creating monitoring user..."
        pveum user add pulse-monitor@pve --comment "Pulse monitoring service" 2>/dev/null || true
    fi

    if pulse_pve_token_exists; then
        token_exists=true
        echo "  ✓ Current token exists: $PULSE_TOKEN_ID"
        echo "    Expire: $(pulse_pve_token_expire_label)"
    else
        echo "  ⚠ Current token was not found: $PULSE_TOKEN_ID"
        echo "    Choose Install/Configure to rotate the token and re-register Pulse with the new value."
    fi

    local old_tokens_pve old_tokens_pam old_count_pve old_count_pam
    old_tokens_pve=$(pulse_pve_matching_tokens pulse-monitor@pve)
    old_tokens_pam=$(pulse_pve_matching_tokens pulse-monitor@pam)
    old_count_pve=$(echo "$old_tokens_pve" | grep -c "pulse" || true)
    old_count_pam=$(echo "$old_tokens_pam" | grep -c "pulse" || true)
    echo ""
    echo "Pulse-managed token inventory:"
    if [ "$old_count_pve" -eq 0 ] && [ "$old_count_pam" -eq 0 ]; then
        echo "  ✓ No matching Pulse-managed tokens found for this Pulse server scope."
    else
        [ -n "$old_tokens_pve" ] && echo "$old_tokens_pve" | sed 's/^/  • pulse-monitor@pve!/g'
        [ -n "$old_tokens_pam" ] && echo "$old_tokens_pam" | sed 's/^/  • pulse-monitor@pam!/g'
        echo "  The current token is expected. Timestamp-suffixed entries are older Pulse-managed tokens."
        echo "  ACL rows for both the user and current token are also expected."
    fi

    echo ""
    echo "Permission audit:"
    print_pve_acl_status "/" "pulse-monitor@pve" "PVEAuditor" "User has PVEAuditor at /"
    if [ "$token_exists" = true ]; then
        print_pve_acl_status "/" "$PULSE_TOKEN_ID" "PVEAuditor" "Token has PVEAuditor at /"
        print_pve_acl_status "/" "$PULSE_TOKEN_ID" "PulseMonitor" "Token has PulseMonitor at /"
    fi
    print_pve_acl_status "/" "pulse-monitor@pve" "PulseMonitor" "User has PulseMonitor at /"

    echo ""
    echo "Reapplying safe Pulse-managed permissions..."
    pveum aclmod / -user pulse-monitor@pve -role PVEAuditor
    if [ "$token_exists" = true ]; then
        pveum aclmod / -token "$PULSE_TOKEN_ID" -role PVEAuditor
    fi%s
    configure_pve_pulse_monitor_role "$token_exists"

    echo ""
    echo "Setup doctor complete."
    echo "If Pulse still shows 401 after this, the stored token value in Pulse no longer matches Proxmox."
    echo "Choose Install/Configure to rotate the token and let Pulse re-register it."
    echo ""
    exit 0
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Main Menu
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "What would you like to do?"
echo ""
echo "  [1] Install/Configure - Set up Pulse monitoring"
echo "  [2] Audit/Repair      - Check and repair Pulse-managed Proxmox setup"
echo "  [3] Remove All        - Uninstall everything Pulse has configured"
echo "  [4] Cancel            - Exit without changes"
echo ""
echo -n "Your choice [1/2/3/4]: "

MAIN_ACTION="${PULSE_SETUP_ACTION:-}"
if [ -t 0 ]; then
    if [ -z "$MAIN_ACTION" ]; then
        read -n 1 -r MAIN_ACTION
    fi
else
    if [ -z "$MAIN_ACTION" ]; then
        if read -n 1 -r MAIN_ACTION </dev/tty 2>/dev/null; then
            :
        else
            echo "(No terminal available - defaulting to Install)"
            MAIN_ACTION="1"
        fi
    fi
fi
echo ""
echo ""

# Handle Cancel
if [[ $MAIN_ACTION =~ ^(4|[Cc]|cancel)$ ]]; then
    echo "Cancelled. No changes made."
    exit 0
fi

if [[ $MAIN_ACTION =~ ^(2|[Aa]|audit|repair)$ ]]; then
    run_pve_setup_doctor
fi

# Handle Remove All
if [[ $MAIN_ACTION =~ ^(3|[Rr]|remove)$ ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🗑️  Complete Removal"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "This will remove:"
    echo "  • SSH keys from authorized_keys (Pulse-managed entries)"
    echo "  • Pulse monitoring API tokens and user"
    echo ""
    echo "⚠️  WARNING: This is a destructive operation!"
    echo ""
    echo -n "Are you sure? [y/N]: "

    CONFIRM_REMOVE=""
    if [ -t 0 ]; then
        read -n 1 -r CONFIRM_REMOVE
    else
        if read -n 1 -r CONFIRM_REMOVE </dev/tty 2>/dev/null; then
            :
        else
            echo "(No terminal available - cancelling removal)"
            CONFIRM_REMOVE="n"
        fi
    fi
    echo ""
    echo ""

    if [[ ! $CONFIRM_REMOVE =~ ^[Yy]$ ]]; then
        echo "Removal cancelled. No changes made."
        exit 0
    fi

    echo "Removing Pulse monitoring components..."
    echo ""

    SERVER_HOSTNAME=$(hostname -s 2>/dev/null || hostname)

    if [ -n "$PULSE_SETUP_TOKEN" ]; then
        echo "  • Removing Pulse connection from server..."
        UNREGISTER_JSON='{"type":"pve","host":"'"$HOST_URL"'","serverName":"'"$SERVER_HOSTNAME"'","tokenId":"'"$PULSE_TOKEN_ID"'","authToken":"'"$PULSE_SETUP_TOKEN"'","source":"script"}'
        UNREGISTER_RESPONSE=$(echo "$UNREGISTER_JSON" | curl -fsS -X POST "$PULSE_URL/api/auto-unregister" \
            -H "Content-Type: application/json" \
            -d @- 2>&1)
        UNREGISTER_RC=$?
        if [ "$UNREGISTER_RC" -ne 0 ]; then
            echo "    ⚠️  Pulse server teardown request failed."
            echo "       Response: $UNREGISTER_RESPONSE"
            echo "       Remove the node from Pulse manually if it remains listed."
        elif echo "$UNREGISTER_RESPONSE" | grep -Eq '"status"[[:space:]]*:[[:space:]]*"success"'; then
            if echo "$UNREGISTER_RESPONSE" | grep -Eq '"removed"[[:space:]]*:[[:space:]]*true'; then
                echo "    ✓ Removed matching Proxmox VE connection from Pulse"
            else
                echo "    ✓ Pulse confirmed there is no matching Proxmox VE connection to remove"
            fi
        else
            echo "    ⚠️  Pulse server teardown did not confirm success."
            echo "       Response: $UNREGISTER_RESPONSE"
            echo "       Remove the node from Pulse manually if it remains listed."
        fi
    else
        echo "  • No Pulse setup token available for server-side teardown"
        echo "    Remove the node from Pulse manually if it remains listed."
    fi


    # Always run manual removal for local services and files
    if true; then
        # Remove SSH keys from authorized_keys (only Pulse-managed entries)
        AUTH_KEYS="$(resolve_authorized_keys_path)"
        if [ -f "$AUTH_KEYS" ]; then
            echo "  • Removing SSH keys from authorized_keys..."
            TMP_AUTH_KEYS="$(mktemp /tmp/.pulse-authorized-keys.XXXXXX 2>/dev/null)" || TMP_AUTH_KEYS=""
            if [ -n "$TMP_AUTH_KEYS" ] && [ -f "$TMP_AUTH_KEYS" ]; then
                grep -vF '# pulse-' "$AUTH_KEYS" > "$TMP_AUTH_KEYS" 2>/dev/null
                GREP_EXIT=$?
                if [ $GREP_EXIT -eq 0 ] || [ $GREP_EXIT -eq 1 ]; then
                    if install_authorized_keys_file "$TMP_AUTH_KEYS" "$AUTH_KEYS"; then
                        :
                    else
                        rm -f "$TMP_AUTH_KEYS"
                        echo "    ⚠️  Failed to update $AUTH_KEYS"
                    fi
                else
                    rm -f "$TMP_AUTH_KEYS"
                fi
            else
                echo "    ⚠️  Failed to create temporary authorized_keys file"
            fi
        fi

        # Remove Pulse monitoring API tokens and user
        echo "  • Removing Pulse monitoring API tokens and user..."
        if command -v pveum &> /dev/null; then
            TOKEN_LIST=$(pveum user token list pulse-monitor@pve 2>/dev/null | awk 'NR>3 {print $2}' | grep -v '^$' || printf '')
            if [ -n "$TOKEN_LIST" ]; then
                while IFS= read -r TOKEN; do
                    if [ -n "$TOKEN" ]; then
                        pveum user token remove pulse-monitor@pve "$TOKEN" 2>/dev/null || true
                    fi
                done <<< "$TOKEN_LIST"
            fi
            pveum user delete pulse-monitor@pve 2>/dev/null || true
            pveum user delete pulse-monitor@pam 2>/dev/null || true
            pveum role delete PulseMonitor 2>/dev/null || true
        fi

        if command -v proxmox-backup-manager &> /dev/null; then
            proxmox-backup-manager user delete pulse-monitor@pbs 2>/dev/null || true
        fi
    fi

    echo ""
    echo "✓ Complete removal finished"
    echo ""
    echo "All Pulse monitoring components have been removed from this host."
    exit 0
fi

# If we get here, user chose Install (or default)
echo "Proceeding with installation..."
echo ""

# Check for old Pulse tokens from the same Pulse server and offer to clean them up
OLD_TOKENS_PVE=$(pulse_pve_matching_tokens pulse-monitor@pve)
OLD_TOKENS_PAM=$(pulse_pve_matching_tokens pulse-monitor@pam)
if [ ! -z "$OLD_TOKENS_PVE" ] || [ ! -z "$OLD_TOKENS_PAM" ]; then
    echo "Checking for existing Pulse monitoring tokens from this Pulse server..."
    TOKEN_COUNT_PVE=$(echo "$OLD_TOKENS_PVE" | grep -c "pulse" || true)
    TOKEN_COUNT_PAM=$(echo "$OLD_TOKENS_PAM" | grep -c "pulse" || true)
    TOKEN_COUNT=$((TOKEN_COUNT_PVE + TOKEN_COUNT_PAM))
    echo ""
    echo "⚠️  Found $TOKEN_COUNT old Pulse monitoring token(s) from this Pulse server (${TOKEN_MATCH_PREFIX}):"
    [ ! -z "$OLD_TOKENS_PVE" ] && echo "$OLD_TOKENS_PVE" | sed 's/^/   - /' | sed 's/$/ (pve)/'
    [ ! -z "$OLD_TOKENS_PAM" ] && echo "$OLD_TOKENS_PAM" | sed 's/^/   - /' | sed 's/$/ (pam)/'
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🗑️  CLEANUP OPTION"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Would you like to remove these old tokens? Type 'y' for yes, 'n' for no: "
    # Read from terminal, not from stdin (which is the piped script)
    if [ -t 0 ]; then
        # Running interactively
        read -p "> " -n 1 -r REPLY
    else
        # Being piped - try to read from terminal if available
        if read -p "> " -n 1 -r REPLY </dev/tty 2>/dev/null; then
            # Successfully read from terminal
            :
        else
            # No terminal available (e.g., in Docker without -t flag)
            echo "(No terminal available for input - keeping existing tokens)"
            REPLY="n"
        fi
    fi
    echo ""
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing old tokens..."
        while IFS= read -r TOKEN; do
            if [ ! -z "$TOKEN" ]; then
                pveum user token remove pulse-monitor@pve "$TOKEN" 2>/dev/null && echo "   ✓ Removed token: $TOKEN (pve)" || echo "   ✗ Failed to remove: $TOKEN (pve)"
            fi
        done <<< "$OLD_TOKENS_PVE"
        while IFS= read -r TOKEN; do
            if [ ! -z "$TOKEN" ]; then
                pveum user token remove pulse-monitor@pam "$TOKEN" 2>/dev/null && echo "   ✓ Removed token: $TOKEN (pam)" || echo "   ✗ Failed to remove: $TOKEN (pam)"
            fi
        done <<< "$OLD_TOKENS_PAM"
        echo ""
    else
        echo "Keeping existing tokens."
    fi
    echo ""
fi

# Create monitoring user
echo "Creating monitoring user..."
pveum user add pulse-monitor@pve --comment "Pulse monitoring service" 2>/dev/null || true

AUTO_REG_SUCCESS=false

extract_json_string_field() {
    local json_input="$1"
    local field_name="$2"

    printf '%%s\n' "$json_input" | sed -n 's/.*"'"$field_name"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1
}

extract_pve_token_value() {
    local token_output="$1"
    local token_value=""

    token_value=$(extract_json_string_field "$token_output" "value")
    if [ -n "$token_value" ]; then
        printf '%%s\n' "$token_value"
        return 0
    fi

    printf '%%s\n' "$token_output" | awk -F'[|│]' '
        function trim(value) {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
            return value
        }

        {
            key = trim($2)
            value = trim($3)
            if (key == "value" && value != "") {
                print value
                exit
            }
        }
    '
}

create_pve_token() {
    if TOKEN_OUTPUT=$(pveum user token add pulse-monitor@pve "$TOKEN_NAME" --privsep 1 --output-format json 2>&1); then
        TOKEN_CREATE_RC=0
    else
        TOKEN_CREATE_RC=$?
    fi

    if [ "$TOKEN_CREATE_RC" -eq 0 ]; then
        TOKEN_VALUE=$(extract_pve_token_value "$TOKEN_OUTPUT")
        return 0
    fi

    if echo "$TOKEN_OUTPUT" | grep -Eqi 'unknown option|unknown command|no such option|unable to parse option|output-format'; then
        if TOKEN_OUTPUT=$(pveum user token add pulse-monitor@pve "$TOKEN_NAME" --privsep 1 2>&1); then
            TOKEN_CREATE_RC=0
        else
            TOKEN_CREATE_RC=$?
        fi
        if [ "$TOKEN_CREATE_RC" -eq 0 ]; then
            TOKEN_VALUE=$(extract_pve_token_value "$TOKEN_OUTPUT")
        fi
    fi

    return "$TOKEN_CREATE_RC"
}

smoke_test_pve_token() {
    if SMOKE_OUTPUT=$(curl -kfsS --retry 2 --retry-delay 1 \
        -H "Authorization: PVEAPIToken=$PULSE_TOKEN_ID=$TOKEN_VALUE" \
        "${HOST_URL%%/}/api2/json/nodes" 2>&1); then
        SMOKE_RC=0
    else
        SMOKE_RC=$?
    fi

    if [ "$SMOKE_RC" -ne 0 ]; then
        echo "⚠️  Created API token, but the local Proxmox API smoke check failed."
        echo "   Response: $SMOKE_OUTPUT"
        echo ""
        echo "📝 Use the token details below in Pulse Settings → Nodes after resolving the API connectivity issue."
        TOKEN_READY=false
        AUTO_REG_SUCCESS=false
        return 1
    fi

    return 0
}

attempt_auto_registration() {
    if [ -z "$PULSE_SETUP_TOKEN" ]; then
        if [ -t 0 ]; then
            printf "Pulse setup token: "
            if command -v stty >/dev/null 2>&1; then stty -echo; fi
            IFS= read -r PULSE_SETUP_TOKEN
            if command -v stty >/dev/null 2>&1; then stty echo; fi
            printf "\n"
		elif [ -c /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]; then
			printf "Pulse setup token: " >/dev/tty
			if command -v stty >/dev/null 2>&1; then stty -echo </dev/tty 2>/dev/null || true; fi
			IFS= read -r PULSE_SETUP_TOKEN </dev/tty || true
			if command -v stty >/dev/null 2>&1; then stty echo </dev/tty 2>/dev/null || true; fi
			printf "\n" >/dev/tty
		fi
	fi

    if [ -z "$TOKEN_VALUE" ]; then
        echo "⚠️  Auto-registration skipped: token value unavailable"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
        return
    fi

    if [ -z "$PULSE_SETUP_TOKEN" ]; then
        echo "⚠️  Auto-registration skipped: no setup token provided"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
        return
    fi

    SERVER_HOSTNAME=$(hostname -s 2>/dev/null || hostname)
    SERVER_IP=$(hostname -I | awk '{print $1}')

    REGISTER_JSON='{"type":"pve","host":"'"$HOST_URL"'","serverName":"'"$SERVER_HOSTNAME"'","tokenId":"'"$PULSE_TOKEN_ID"'","tokenValue":"'"$TOKEN_VALUE"'","authToken":"'"$PULSE_SETUP_TOKEN"'","source":"script"}'

    REGISTER_RESPONSE=$(echo "$REGISTER_JSON" | curl -fsS -X POST "$PULSE_URL/api/auto-register" \
        -H "Content-Type: application/json" \
        -d @- 2>&1)
    REGISTER_RC=$?

    AUTO_REG_SUCCESS=false
    if [ "$REGISTER_RC" -ne 0 ]; then
        echo "⚠️  Auto-registration request failed before success confirmation."
        echo "   Response: $REGISTER_RESPONSE"
        echo ""
        echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."
    elif echo "$REGISTER_RESPONSE" | grep -Eq '"status"[[:space:]]*:[[:space:]]*"success"'; then
        AUTO_REG_SUCCESS=true
        echo "Successfully registered with Pulse monitoring."
        echo ""
    else
        if echo "$REGISTER_RESPONSE" | grep -q "Authentication required"; then
            SETUP_TOKEN_INVALID=true
            echo "Error: Auto-registration failed - authentication required"
            echo ""
            echo "The provided Pulse setup token was invalid or expired"
            echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."
        else
            echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."
            echo "   Response: $REGISTER_RESPONSE"
            echo ""
            echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."
        fi
    fi
}

# Generate API token
echo "Generating API token..."

# Rotate token if it already exists so reruns remain idempotent
TOKEN_EXISTED=false
TOKEN_OUTPUT=""
TOKEN_VALUE=""
TOKEN_CREATED=false
TOKEN_READY=false
if pulse_pve_token_exists; then
    TOKEN_EXISTED=true
    echo "Existing token '$TOKEN_NAME' found. Rotating in place..."
    if ! pveum user token remove pulse-monitor@pve "$TOKEN_NAME" >/dev/null 2>&1; then
        echo "⚠️  Failed to remove existing token '$TOKEN_NAME'. Attempting create anyway..."
    fi
fi

# Create a privilege-separated token and capture value (shown once by Proxmox)
create_pve_token
if [ "$TOKEN_CREATE_RC" -ne 0 ]; then
    echo "❌ Failed to create token '$TOKEN_NAME'"
    echo "$TOKEN_OUTPUT"
    echo ""
    echo "Fix the token creation error above and rerun this script on the node."
    echo ""
else
    TOKEN_CREATED=true

    if [ -z "$TOKEN_VALUE" ]; then
        echo ""
        echo "================================================================"
        echo "IMPORTANT: Copy the token value below - it's only shown once!"
        echo "================================================================"
        echo "$TOKEN_OUTPUT"
        echo "================================================================"
        echo ""
        echo "⚠️  Failed to extract token value from output."
        echo "   Resolve the token output issue above and rerun this script on the node."
        echo ""
    else
        if [ "$TOKEN_EXISTED" = true ]; then
            echo "API token rotated successfully"
        else
            echo "API token generated successfully"
        fi
        TOKEN_READY=true
        echo ""
    fi

fi

# Set up permissions
echo "Setting up permissions..."
pveum aclmod / -user pulse-monitor@pve -role PVEAuditor
if [ "$TOKEN_CREATED" = true ]; then
    pveum aclmod / -token "$PULSE_TOKEN_ID" -role PVEAuditor
fi%s

configure_pve_pulse_monitor_role "$TOKEN_CREATED"

if [ "$TOKEN_READY" = true ]; then
    if smoke_test_pve_token; then
        attempt_auto_registration
    else
        AUTO_REG_SUCCESS=false
    fi
else
    AUTO_REG_SUCCESS=false
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Temperature Monitoring Setup (Optional)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

SSH_SENSORS_PUBLIC_KEY="%s"
PULSE_SENSORS_WRAPPER="/usr/local/sbin/pulse-sensors"
SSH_SENSORS_KEY_ENTRY="command=\"$PULSE_SENSORS_WRAPPER\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $SSH_SENSORS_PUBLIC_KEY # pulse-sensors"
TEMPERATURE_ENABLED=false

install_pulse_sensors_wrapper() {
    mkdir -p "$(dirname "$PULSE_SENSORS_WRAPPER")"
    cat > "$PULSE_SENSORS_WRAPPER" <<'PULSE_SENSORS_WRAPPER_EOF'
#!/bin/sh
set -eu

if command -v python3 >/dev/null 2>&1; then
    python3 - <<'PY'
import datetime
import json
import os
import re
import shlex
import shutil
import subprocess
import sys


def run_command(args, timeout=8):
    try:
        return subprocess.run(
            args,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            timeout=timeout,
            check=False,
        )
    except Exception:
        return None


def load_sensors():
    if shutil.which("sensors"):
        result = run_command(["sensors", "-j"])
        if result and result.stdout.strip():
            try:
                return json.loads(result.stdout)
            except Exception:
                pass

    thermal_path = "/sys/class/thermal/thermal_zone0/temp"
    try:
        with open(thermal_path, "r", encoding="utf-8") as handle:
            raw = handle.read().strip()
        value = float(raw)
        if value > 1000:
            value = value / 1000
        return {"rpitemp": {"temp1": {"temp1_input": value}}}
    except Exception:
        return {}


def smartctl_path():
    return shutil.which("smartctl")


def is_multiplexed_device_type(device_type):
    return bool(device_type and re.search(r",\d", device_type))


def nvme_namespace_for_controller(name):
    if not re.fullmatch(r"nvme\d+", name or ""):
        return ""
    try:
        for candidate in sorted(os.listdir("/sys/block")):
            if candidate.startswith(name + "n"):
                return candidate
    except Exception:
        return ""
    return ""


def canonical_block_for_device(device):
    name = os.path.basename(str(device or "").strip())
    if not name:
        return ""
    if re.fullmatch(r"nvme\d+", name):
        return nvme_namespace_for_controller(name) or name
    return name


def is_non_rotational_block_device(device):
    # Positively confirmed SSD via sysfs; anything uncertain returns False so
    # the caller keeps the conservative standby guard.
    block = canonical_block_for_device(device)
    if not block:
        return False
    try:
        with open("/sys/block/%%s/queue/rotational" %% block) as handle:
            return handle.read().strip() == "0"
    except Exception:
        return False


def inferred_smart_device_types(device):
    name = os.path.basename(str(device or "").strip()).lower()
    if re.fullmatch(r"(sd|hd)[a-z]+", name or ""):
        return ["sat", "scsi"]
    return []


def is_physical_block_device(device):
    name = str(device.get("name") or "").strip()
    dtype = str(device.get("type") or "").strip().lower()
    if not name or dtype != "disk":
        return False

    lowered_name = name.lower()
    virtual_prefixes = (
        "loop", "ram", "zram", "dm-", "md", "nbd", "rbd", "sr", "fd",
        "zd", "vd", "xvd",
    )
    if lowered_name.startswith(virtual_prefixes):
        return False

    metadata = " ".join(
        str(device.get(key) or "").lower()
        for key in ("tran", "model", "vendor", "subsystems")
    )
    virtual_tokens = ("virtual", "virtio", "qemu", "vmware", "/virtual/")
    return not any(token in metadata for token in virtual_tokens)


def block_device_targets():
    result = run_command(["lsblk", "-J", "-d", "-o", "NAME,TYPE,TRAN,MODEL,VENDOR,SUBSYSTEMS"])
    if not result or not result.stdout.strip():
        return []
    try:
        data = json.loads(result.stdout)
    except Exception:
        return []

    devices = []
    seen = set()
    for device in data.get("blockdevices") or []:
        if not isinstance(device, dict) or not is_physical_block_device(device):
            continue
        name = str(device.get("name") or "").strip()
        if not name or name in seen:
            continue
        seen.add(name)
        devices.append("/dev/" + name)
    return devices


def union_smart_targets(scan_targets, block_devices):
    targets = []
    seen_targets = set()
    covered_blocks = set()

    for device, device_type in scan_targets:
        key = (device, device_type)
        if key in seen_targets:
            continue
        seen_targets.add(key)
        targets.append(key)
        if not is_multiplexed_device_type(device_type):
            block = canonical_block_for_device(device)
            if block:
                covered_blocks.add(block)

    for device in block_devices:
        block = canonical_block_for_device(device)
        if not block or block in covered_blocks:
            continue
        key = ("/dev/" + block, "")
        if key in seen_targets:
            continue
        seen_targets.add(key)
        targets.append(key)
        covered_blocks.add(block)

    return targets


def smart_targets(path):
    result = run_command([path, "--scan-open"])

    targets = []
    seen = set()
    typed_by_path = set()
    if result and result.stdout.strip():
        for raw_line in result.stdout.splitlines():
            line = raw_line.split("#", 1)[0].strip()
            if not line:
                continue
            try:
                fields = shlex.split(line)
            except ValueError:
                fields = line.split()
            if not fields:
                continue

            device = fields[0].strip()
            if not device.startswith("/"):
                continue

            device_type = ""
            for idx, field in enumerate(fields[:-1]):
                if field == "-d":
                    device_type = fields[idx + 1].strip()
                    break

            key = (device, device_type)
            if key in seen:
                continue
            seen.add(key)
            if device_type:
                typed_by_path.add(device)
            targets.append(key)

    scan_targets = [(device, dtype) for device, dtype in targets if dtype or device not in typed_by_path]
    return union_smart_targets(scan_targets, block_device_targets())


def smart_probe_attempts(device, device_type):
    attempts = []
    seen = set()

    def add(dtype):
        key = (device, dtype)
        if key in seen:
            return
        seen.add(key)
        attempts.append(key)

    if device_type:
        add(device_type)
    else:
        add("")

    if device_type and not is_multiplexed_device_type(device_type):
        add("")
    if not is_multiplexed_device_type(device_type):
        for dtype in inferred_smart_device_types(device):
            add(dtype)
    return attempts


def disk_type(data):
    device = data.get("device") if isinstance(data.get("device"), dict) else {}
    protocol = str(device.get("protocol") or "").lower()
    dtype = str(device.get("type") or "").lower()
    if "nvme" in protocol or "nvme" in dtype:
        return "nvme"
    if "sas" in protocol or "scsi" in protocol or "sas" in dtype:
        return "sas"
    if "ata" in protocol or "sata" in protocol or "sat" in dtype:
        return "sata"
    return dtype


def smart_health(data):
    status = data.get("smart_status")
    if isinstance(status, dict) and "passed" in status:
        return "PASSED" if status.get("passed") else "FAILED"
    return "UNKNOWN"


def format_wwn(data):
    wwn = data.get("wwn")
    if not isinstance(wwn, dict) or not wwn.get("naa"):
        return ""
    try:
        naa = int(wwn.get("naa") or 0)
        oui = int(wwn.get("oui") or 0)
        ident = int(wwn.get("id") or 0)
    except Exception:
        return ""
    return f"{naa:x}-{oui:x}-{ident:x}"


def first_valid_temperature(candidates):
    for value in candidates:
        try:
            temp = int(float(value))
        except Exception:
            continue
        if 0 < temp < 150:
            return temp
    return 0


def raw_attribute_temperature(data):
    attrs = data.get("ata_smart_attributes")
    if not isinstance(attrs, dict):
        return 0
    table = attrs.get("table")
    if not isinstance(table, list):
        return 0

    for attr in table:
        if not isinstance(attr, dict) or attr.get("id") not in (190, 194):
            continue
        raw = attr.get("raw") if isinstance(attr.get("raw"), dict) else {}
        candidates = []
        raw_string = raw.get("string")
        if isinstance(raw_string, str):
            match = re.search(r"(-?\d{1,3})", raw_string)
            if match:
                candidates.append(match.group(1))
        if "value" in raw:
            candidates.append(raw.get("value"))
        temperature = first_valid_temperature(candidates)
        if temperature > 0:
            return temperature
    return 0


def smart_temperature(data):
    temperatures = []
    temp = data.get("temperature")
    if isinstance(temp, dict):
        temperatures.append(temp.get("current"))
    nvme = data.get("nvme_smart_health_information_log")
    if isinstance(nvme, dict):
        temperatures.append(nvme.get("temperature"))
    sct = data.get("ata_sct_status")
    if isinstance(sct, dict):
        current = sct.get("current")
        if isinstance(current, dict):
            temperatures.append(current.get("value"))

    temperature = first_valid_temperature(temperatures)
    if temperature > 0:
        return temperature
    return raw_attribute_temperature(data)


def collect_smart():
    path = smartctl_path()
    if not path:
        return []

    entries = []
    observed_at = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    for device, device_type in smart_targets(path):
        best_entry = None
        attempts = smart_probe_attempts(device, device_type)
        for attempt_index, (attempt_device, attempt_type) in enumerate(attempts):
            args = [path]
            if attempt_type:
                args.extend(["-d", attempt_type])
            # The standby guard avoids spinning up sleeping rotational disks.
            # SSDs have nothing to spin up, and some SATA SSDs answer CHECK
            # POWER MODE with a bogus standby state that permanently hides
            # their SMART data (#1516), so confirmed SSDs are probed without
            # it. Multiplexed members keep the guard: the shared /dev path's
            # rotational flag describes the array, not the member.
            if is_multiplexed_device_type(attempt_type) or not is_non_rotational_block_device(attempt_device):
                args.extend(["-n", "standby,3"])
            args.extend(["-i", "-A", "-H", "--json=o", attempt_device])

            result = run_command(args)
            if not result or not result.stdout.strip():
                continue
            try:
                data = json.loads(result.stdout)
            except Exception:
                continue

            power_mode = str(data.get("power_mode") or "").lower()
            reported_device = attempt_device
            if is_multiplexed_device_type(attempt_type):
                reported_device = f"{attempt_device} [{attempt_type}]"
            else:
                block = canonical_block_for_device(attempt_device)
                if block:
                    reported_device = "/dev/" + block

            entry = {
                "device": reported_device,
                "model": str(data.get("model_name") or data.get("model_family") or "").strip(),
                "serial": str(data.get("serial_number") or "").strip(),
                "wwn": format_wwn(data),
                "type": disk_type(data),
                "temperature": smart_temperature(data),
                "health": smart_health(data),
                "standbySkipped": "standby" in power_mode or "sleep" in power_mode,
                "lastUpdated": observed_at,
            }
            best_entry = entry
            if entry["temperature"] > 0 or entry["standbySkipped"] or attempt_index == len(attempts) - 1:
                break

        if best_entry:
            entries.append(best_entry)
    return entries


payload = {
    "sensors": load_sensors(),
    "smart": collect_smart(),
}
json.dump(payload, sys.stdout, separators=(",", ":"))
sys.stdout.write("\n")
PY
    exit 0
fi

sensors_json="{}"
if command -v sensors >/dev/null 2>&1; then
    sensors_json="$(sensors -j 2>/dev/null || printf '{}')"
fi
if [ -z "$sensors_json" ]; then
    sensors_json="{}"
fi
printf '{"sensors":%%s,"smart":[]}\n' "$sensors_json"
PULSE_SENSORS_WRAPPER_EOF
    chown root:root "$PULSE_SENSORS_WRAPPER" 2>/dev/null || true
    chmod 755 "$PULSE_SENSORS_WRAPPER"
}

if [ -n "$SSH_SENSORS_PUBLIC_KEY" ]; then
    echo "📊 Enable Temperature Monitoring?"
    echo ""
    echo "Collect CPU and drive temperatures via secure SSH connection."
    echo ""
    echo "Security:"
    echo "  • SSH key authentication with forced command (Pulse sensor wrapper only)"
    echo "  • No shell access, port forwarding, or other SSH features"
    echo "  • Keys stored in Pulse service user's home directory"
    echo ""
    echo "Enable temperature monitoring? [y/N]"
    echo -n "> "

    if [ -t 0 ]; then
        read -n 1 -r SSH_REPLY
    else
        # When stdin is not a terminal (e.g., curl | bash), try /dev/tty first, then stdin for piped input
        if read -n 1 -r SSH_REPLY </dev/tty 2>/dev/null; then
            :
        elif read -t 2 -n 1 -r SSH_REPLY 2>/dev/null && [ -n "$SSH_REPLY" ]; then
            echo "$SSH_REPLY"
        else
            echo "(No terminal available - skipping temperature monitoring)"
            SSH_REPLY="n"
        fi
    fi
    echo ""
    echo ""

    if [[ $SSH_REPLY =~ ^[Yy]$ ]]; then
        echo "Configuring temperature monitoring..."

        SENSORS_WRAPPER_OK=false
        if install_pulse_sensors_wrapper; then
            SENSORS_WRAPPER_OK=true
            echo "  ✓ Pulse sensor wrapper installed"
        else
            echo "  ⚠️  Failed to install Pulse sensor wrapper"
        fi

        # Add key to root's authorized_keys
        AUTH_KEYS="$(resolve_authorized_keys_path)"
        mkdir -p "$(dirname "$AUTH_KEYS")"
        mkdir -p /root/.ssh
        chmod 700 /root/.ssh 2>/dev/null || true

        # Remove any old pulse keys
        SENSORS_KEY_OK=false
        TMP_AUTH_KEYS="$(mktemp /tmp/.pulse-authorized-keys.XXXXXX 2>/dev/null)" || TMP_AUTH_KEYS=""
        if [ "$SENSORS_WRAPPER_OK" = true ] && [ -n "$TMP_AUTH_KEYS" ] && [ -f "$TMP_AUTH_KEYS" ]; then
            if [ -f "$AUTH_KEYS" ]; then
                grep -vF "# pulse-" "$AUTH_KEYS" > "$TMP_AUTH_KEYS" 2>/dev/null
                GREP_EXIT=$?
                if [ $GREP_EXIT -ne 0 ] && [ $GREP_EXIT -ne 1 ]; then
                    echo "  ⚠️  Failed to read $AUTH_KEYS"
                    rm -f "$TMP_AUTH_KEYS"
                    TMP_AUTH_KEYS=""
                fi
            fi

            if [ -n "$TMP_AUTH_KEYS" ]; then
                printf '%%s\n' "$SSH_SENSORS_KEY_ENTRY" >> "$TMP_AUTH_KEYS"
                if install_authorized_keys_file "$TMP_AUTH_KEYS" "$AUTH_KEYS"; then
                    SENSORS_KEY_OK=true
                else
                    echo "  ⚠️  Failed to update $AUTH_KEYS"
                    rm -f "$TMP_AUTH_KEYS" 2>/dev/null || true
                fi
            fi
        else
            if [ "$SENSORS_WRAPPER_OK" = true ]; then
                echo "  ⚠️  Failed to create temp file - cannot update authorized_keys"
            elif [ -n "$TMP_AUTH_KEYS" ]; then
                rm -f "$TMP_AUTH_KEYS" 2>/dev/null || true
            fi
        fi
        if [ "$SENSORS_KEY_OK" = true ]; then
            echo "  ✓ Sensors key configured (restricted to Pulse sensor wrapper)"
        else
            echo "  ⚠️  Sensors key was not configured"
        fi

        # Check if this is a Raspberry Pi
        IS_RPI=false
        if [ -f /proc/device-tree/model ] && grep -qi "raspberry pi" /proc/device-tree/model 2>/dev/null; then
            IS_RPI=true
        fi

        TEMPERATURE_SETUP_SUCCESS=false

        # Install lm-sensors if not present (skip on Raspberry Pi)
        if ! command -v sensors &> /dev/null; then
            if [ "$IS_RPI" = true ]; then
                echo "  ℹ️  Raspberry Pi detected - using native RPi temperature interface"
                echo "    Pulse will read temperature from /sys/class/thermal/thermal_zone0/temp"
                TEMPERATURE_SETUP_SUCCESS=true
            else
                echo "  ✓ Installing lm-sensors..."

                # Try to update and install, but provide helpful errors if it fails
                UPDATE_OUTPUT=$(apt-get update -qq 2>&1)
                if echo "$UPDATE_OUTPUT" | grep -q "Could not create temporary file\|/tmp"; then
                    echo ""
                    echo "    ⚠️  APT cannot write to /tmp directory"
                    echo "    This may be a permissions issue. To fix:"
                    echo "      sudo chown root:root /tmp"
                    echo "      sudo chmod 1777 /tmp"
                    echo ""
                    echo "    Attempting installation anyway..."
                elif echo "$UPDATE_OUTPUT" | grep -q "Failed to fetch\|GPG error\|no longer has a Release file"; then
                    echo "    ⚠️  Some repository errors detected, attempting installation anyway..."
                fi

                if apt-get install -y lm-sensors > /dev/null 2>&1; then
                    sensors-detect --auto > /dev/null 2>&1 || true
                    echo "    ✓ lm-sensors installed successfully"
                    TEMPERATURE_SETUP_SUCCESS=true
                else
                    echo ""
                    echo "    ⚠️  Could not install lm-sensors"
                    echo "    Possible causes:"
                    echo "      - Repository configuration errors"
                    echo "      - /tmp directory permission issues"
                    echo "      - Network connectivity problems"
                    echo ""
                    echo "    To fix manually:"
                    echo "      1. Check /tmp permissions: ls -ld /tmp"
                    echo "         (should be: drwxrwxrwt owned by root:root)"
                    echo "      2. Fix if needed: sudo chown root:root /tmp && sudo chmod 1777 /tmp"
                    echo "      3. Install: sudo apt-get update && sudo apt-get install -y lm-sensors"
                    echo ""
                fi
            fi
        else
            echo "  ✓ lm-sensors package verified"
            TEMPERATURE_SETUP_SUCCESS=true
        fi

        if ! command -v smartctl &> /dev/null; then
            echo "  ✓ Installing smartmontools for disk SMART temperatures..."
            apt-get update -qq > /dev/null 2>&1 || true
            if apt-get install -y smartmontools > /dev/null 2>&1; then
                echo "    ✓ smartmontools installed successfully"
            else
                echo "    ⚠️  Could not install smartmontools"
                echo "    HDD/SATA/SAS disk temperatures require smartctl or the unified agent."
            fi
        else
            echo "  ✓ smartmontools package verified"
        fi

        echo ""
        if [ "$TEMPERATURE_SETUP_SUCCESS" = true ] && [ "$SENSORS_KEY_OK" = true ]; then
            echo "✓ Temperature monitoring enabled"
            if [ "$IS_RPI" = true ]; then
                echo "  Using Raspberry Pi native temperature interface"
            fi
            echo "  Temperature data will appear in the dashboard within 10 seconds"
            TEMPERATURE_ENABLED=true
        else
            echo "⚠️  Temperature monitoring setup incomplete"
            echo "  You can re-run this script after installing lm-sensors and smartmontools"
        fi
    else
        echo "Skipping temperature monitoring."
    fi
else
    echo "Temperature monitoring keys are not available from Pulse."
fi

echo ""
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Setup Complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
if [ "$AUTO_REG_SUCCESS" = true ]; then
    echo "Successfully registered with Pulse monitoring."
    echo "Data will appear in your dashboard within 10 seconds."
    echo ""
elif [ "$TOKEN_CREATED" != true ]; then
    echo "Pulse monitoring token setup failed."
    echo "Resolve the token creation error shown above and rerun this script on the node."
    echo ""
elif [ "$SETUP_TOKEN_INVALID" = true ]; then
    echo "Pulse setup token authentication failed."
    echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."
    echo ""
elif [ "$TOKEN_READY" != true ]; then
    echo "Pulse monitoring token setup could not be completed."
    echo "Resolve the token output issue shown above and rerun this script on the node."
    echo ""
else
    echo "Pulse monitoring token setup completed."
    echo "Finish registration in Pulse using the manual setup details below."
    echo ""
fi

# Only show manual setup instructions if auto-registration failed
if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then
    if [ "$TOKEN_READY" = true ]; then
        echo "Add this server to Pulse with:"
        echo "  Token ID: $PULSE_TOKEN_ID"
        if [ -n "$TOKEN_VALUE" ]; then
            echo "  Token Value: $TOKEN_VALUE"
        else
            echo "  Token Value: [See token output above]"
        fi
        echo "  Host URL: $SERVER_HOST"
        echo ""
        echo "Use these details in Pulse Settings → Nodes to finish registration."
        echo ""
    fi
fi
`, ctx.ServerName, time.Now().Format("2006-01-02 15:04:05"),
		ctx.PulseURL, ctx.ServerHost, ctx.SetupToken, ctx.TokenName, ctx.TokenMatchPrefix, ctx.Artifact.URL, posixShellQuote(ctx.Artifact.CommandWithEnv),
		ctx.StorageRepairPerms,
		ctx.StoragePerms,
		ctx.SensorsPublicKey)
}

func renderPBSSetupScript(ctx setupScriptRenderContext) string {
	return fmt.Sprintf(`#!/bin/bash
# Pulse Monitoring Setup Script for PBS %s
# Generated: %s

echo "============================================"
echo "  Pulse Monitoring Setup for PBS"
echo "============================================"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "Root privileges required. Run as root (su -) and retry."
   exit 1
fi

# Check if proxmox-backup-manager command exists
if ! command -v proxmox-backup-manager &> /dev/null; then
   echo ""
   echo "❌ ERROR: 'proxmox-backup-manager' command not found!"
   echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
   echo ""
   echo "This script must be run on a Proxmox Backup Server."
   echo "The 'proxmox-backup-manager' command is required to create users and tokens."
   echo ""
   echo "If you're seeing this error, you might be:"
   echo "  • Running on a non-PBS system"
   echo "  • On a PVE server (use the PVE setup script instead)"
   echo "  • Missing PBS installation or in wrong environment"
   echo ""
   echo "If PBS is running in Docker, ensure you're inside the PBS container."
   echo ""
   exit 1
fi

# Match canonical Pulse-managed token names for this Pulse server scope.
TOKEN_MATCH_PREFIX="%s"
TOKEN_NAME="%s"
PULSE_TOKEN_ID="pulse-monitor@pbs!${TOKEN_NAME}"
HOST_URL="%s"
SETUP_TOKEN_INVALID=false

# Check for old Pulse tokens from the same Pulse server and offer to clean them up
echo "Checking for existing Pulse monitoring tokens from this Pulse server..."
# PBS outputs tokens differently than PVE - extract just the token names matching this Pulse server
OLD_TOKENS=$(proxmox-backup-manager user list-tokens pulse-monitor@pbs 2>/dev/null | grep -oE "${TOKEN_MATCH_PREFIX}(-[0-9]+)?" | sort -u || true)
if [ ! -z "$OLD_TOKENS" ]; then
    TOKEN_COUNT=$(echo "$OLD_TOKENS" | wc -l)
    echo ""
    echo "⚠️  Found $TOKEN_COUNT old Pulse monitoring token(s) from this Pulse server (${TOKEN_MATCH_PREFIX}):"
    echo "$OLD_TOKENS" | sed 's/^/   - /'
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🗑️  CLEANUP OPTION"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Would you like to remove these old tokens? Type 'y' for yes, 'n' for no: "
    # Read from terminal, not from stdin (which is the piped script)
    if [ -t 0 ]; then
        # Running interactively
        read -p "> " -n 1 -r REPLY
    else
        # Being piped - try to read from terminal if available
        if read -p "> " -n 1 -r REPLY </dev/tty 2>/dev/null; then
            # Successfully read from terminal
            :
        else
            # No terminal available (e.g., in Docker without -t flag)
            echo "(No terminal available for input - keeping existing tokens)"
            REPLY="n"
        fi
    fi
    echo ""
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing old tokens..."
        while IFS= read -r TOKEN; do
            if [ ! -z "$TOKEN" ]; then
                proxmox-backup-manager user delete-token pulse-monitor@pbs "$TOKEN" 2>/dev/null && echo "   ✓ Removed token: $TOKEN" || echo "   ✗ Failed to remove: $TOKEN"
            fi
        done <<< "$OLD_TOKENS"
        echo ""
    else
        echo "Keeping existing tokens."
    fi
    echo ""
fi

# Create monitoring user
echo "Creating monitoring user..."
proxmox-backup-manager user create pulse-monitor@pbs 2>/dev/null || echo "User already exists"

# Generate API token
echo "Generating API token..."

# Rotate token if it already exists so reruns remain idempotent
TOKEN_EXISTED=false
TOKEN_OUTPUT=""
TOKEN_VALUE=""
TOKEN_CREATED=false
TOKEN_READY=false
if proxmox-backup-manager user list-tokens pulse-monitor@pbs 2>/dev/null | awk '{print $1}' | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1; then
    TOKEN_EXISTED=true
    echo "Existing token '$TOKEN_NAME' found. Rotating in place..."
    if ! proxmox-backup-manager user delete-token pulse-monitor@pbs "$TOKEN_NAME" >/dev/null 2>&1; then
        echo "⚠️  Failed to remove existing token '$TOKEN_NAME'. Attempting create anyway..."
    fi
fi

TOKEN_OUTPUT=$(proxmox-backup-manager user generate-token pulse-monitor@pbs "$TOKEN_NAME" 2>&1)
TOKEN_CREATE_RC=$?
if [ "$TOKEN_CREATE_RC" -ne 0 ]; then
    echo "❌ Failed to create token '$TOKEN_NAME'"
    echo "$TOKEN_OUTPUT"
    echo ""
    echo "Fix the token creation error above and rerun this script on the node."
    echo ""
else
    TOKEN_CREATED=true
    echo ""
    echo "================================================================"
    echo "IMPORTANT: Copy the token value below - it's only shown once!"
    echo "================================================================"
    echo "$TOKEN_OUTPUT"

    # Extract the token value for auto-registration
    TOKEN_VALUE=$(echo "$TOKEN_OUTPUT" | grep '"value"' | sed 's/.*"value"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

    if [ -z "$TOKEN_VALUE" ]; then
        echo "⚠️  Failed to extract token value from output."
        echo "   Resolve the token output issue above and rerun this script on the node."
        echo ""
    else
        if [ "$TOKEN_EXISTED" = true ]; then
            echo "✅ Token rotated for Pulse monitoring"
        else
            echo "✅ Token created for Pulse monitoring"
        fi
        TOKEN_READY=true
        echo ""
    fi

    # Use setup token from query parameter when provided (automation workflows)
    PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-%s}"
    REGISTER_ATTEMPTED=false

    if [ -z "$TOKEN_VALUE" ]; then
        echo "⚠️  Auto-registration skipped: token value unavailable"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
        echo ""
    else
    # Prompt the operator if we still don't have a token and a TTY is available
        if [ -z "$PULSE_SETUP_TOKEN" ]; then
            if [ -t 0 ]; then
                printf "Pulse setup token: "
                if command -v stty >/dev/null 2>&1; then stty -echo; fi
                IFS= read -r PULSE_SETUP_TOKEN
                if command -v stty >/dev/null 2>&1; then stty echo; fi
                printf "\n"
		    elif [ -c /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]; then
			    printf "Pulse setup token: " >/dev/tty
			    if command -v stty >/dev/null 2>&1; then stty -echo </dev/tty 2>/dev/null || true; fi
			    IFS= read -r PULSE_SETUP_TOKEN </dev/tty || true
			    if command -v stty >/dev/null 2>&1; then stty echo </dev/tty 2>/dev/null || true; fi
			    printf "\n" >/dev/tty
		    fi
	    fi

        # Only proceed with auto-registration if we have a setup token
        if [ -n "$PULSE_SETUP_TOKEN" ]; then
            # Get the server's hostname (short form to match Pulse node names)
            SERVER_HOSTNAME=$(hostname -s 2>/dev/null || hostname)
            SERVER_IP=$(hostname -I | awk '{print $1}')
            
            # Send registration to Pulse
            PULSE_URL="%s"

            echo "🔄 Attempting auto-registration with Pulse..."
            echo ""
            
            # Construct registration request with setup token
            REGISTER_JSON='{"type":"pbs","host":"'"$HOST_URL"'","serverName":"'"$SERVER_HOSTNAME"'","tokenId":"'"$PULSE_TOKEN_ID"'","tokenValue":"'"$TOKEN_VALUE"'","authToken":"'"$PULSE_SETUP_TOKEN"'","source":"script"}'
            
            # Send registration with setup token
            REGISTER_ATTEMPTED=true
            REGISTER_RESPONSE=$(echo "$REGISTER_JSON" | curl -fsS -X POST "$PULSE_URL/api/auto-register" \
                -H "Content-Type: application/json" \
                -d @- 2>&1)
            REGISTER_RC=$?
        else
            echo "⚠️  Auto-registration skipped: no setup token provided"
            AUTO_REG_SUCCESS=false
            REGISTER_RESPONSE=""
        fi
    fi
    
    AUTO_REG_SUCCESS=false
    if [ "$REGISTER_ATTEMPTED" != true ]; then
        :
    elif [ "$REGISTER_RC" -ne 0 ]; then
        echo "⚠️  Auto-registration request failed before success confirmation."
        echo "   Response: $REGISTER_RESPONSE"
        echo ""
        echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."
    elif echo "$REGISTER_RESPONSE" | grep -Eq '"status"[[:space:]]*:[[:space:]]*"success"'; then
        AUTO_REG_SUCCESS=true
        echo "Successfully registered with Pulse monitoring."
    else
        if echo "$REGISTER_RESPONSE" | grep -q "Authentication required"; then
            SETUP_TOKEN_INVALID=true
            echo "Error: Auto-registration failed - authentication required"
            echo ""
            echo "The provided Pulse setup token was invalid or expired"
            echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."
        else
            echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."
            echo "   Response: $REGISTER_RESPONSE"
            echo ""
            echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."
        fi
    fi
    echo ""
fi
echo "================================================================"
echo ""

# Set up permissions
echo "Setting up permissions..."
proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs
proxmox-backup-manager acl update / Audit --auth-id "$PULSE_TOKEN_ID"

echo ""
echo "✅ Setup complete!"
if [ "$AUTO_REG_SUCCESS" = true ]; then
    echo "Successfully registered with Pulse monitoring."
    echo "Data will appear in your dashboard within 10 seconds."
    echo ""
elif [ "$TOKEN_CREATED" != true ]; then
    echo "Pulse monitoring token setup failed."
    echo "Resolve the token creation error shown above and rerun this script on the node."
    echo ""
elif [ "$SETUP_TOKEN_INVALID" = true ]; then
    echo "Pulse setup token authentication failed."
    echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."
    echo ""
elif [ "$TOKEN_READY" != true ]; then
    echo "Pulse monitoring token setup could not be completed."
    echo "Resolve the token output issue shown above and rerun this script on the node."
    echo ""
else
    echo "Pulse monitoring token setup completed."
    echo "Finish registration in Pulse using the manual setup details below."
    echo ""
fi

# Only show manual setup instructions if auto-registration failed
if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then
    if [ "$TOKEN_READY" = true ]; then
        echo "Add this server to Pulse with:"
        echo "  Token ID: $PULSE_TOKEN_ID"
        if [ -n "$TOKEN_VALUE" ]; then
            echo "  Token Value: $TOKEN_VALUE"
        else
            echo "  Token Value: [See token output above]"
        fi
        echo "  Host URL: $HOST_URL"
        echo ""
        echo "Use these details in Pulse Settings → Nodes to finish registration."
        echo ""
    fi
fi
`, ctx.ServerName, time.Now().Format("2006-01-02 15:04:05"), ctx.TokenMatchPrefix,
		ctx.TokenName, ctx.ServerHost, ctx.SetupToken, ctx.PulseURL)
}
