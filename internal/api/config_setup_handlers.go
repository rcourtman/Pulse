package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
	"github.com/rs/zerolog/log"
)

// HandleSetupScript serves the setup script for Proxmox/PBS nodes
func (h *ConfigHandlers) handleSetupScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	query := r.URL.Query()
	serverType := query.Get("type") // "pve" or "pbs"
	serverHost := strings.TrimSpace(query.Get("host"))
	pulseURL := strings.TrimSpace(query.Get("pulse_url"))   // URL of the Pulse server for auto-registration
	backupPerms := query.Get("backup_perms") == "true"      // Whether to add backup management permissions
	authToken := strings.TrimSpace(query.Get("auth_token")) // Temporary auth token for auto-registration

	if serverHost != "" {
		safeHost, err := sanitizeInstallerURL(serverHost)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid host parameter: %v", err), http.StatusBadRequest)
			return
		}
		serverHost = safeHost
	}

	if pulseURL != "" {
		safeURL, err := sanitizeInstallerURL(pulseURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid pulse_url parameter: %v", err), http.StatusBadRequest)
			return
		}
		pulseURL = safeURL
	}

	if sanitizedToken, err := sanitizeSetupAuthToken(authToken); err != nil {
		http.Error(w, fmt.Sprintf("Invalid auth_token parameter: %v", err), http.StatusBadRequest)
		return
	} else {
		authToken = sanitizedToken
	}

	// Validate required parameters
	if serverType == "" {
		http.Error(w, "Missing required parameter: type (must be 'pve' or 'pbs')", http.StatusBadRequest)
		return
	}

	// If host is not provided, try to use a sensible default
	if serverHost == "" {
		if serverType == "pve" {
			serverHost = "https://YOUR_PROXMOX_HOST:8006"
		} else {
			serverHost = "https://YOUR_PBS_HOST:8007"
		}
		log.Warn().
			Str("type", serverType).
			Msg("No host parameter provided, using placeholder. Auto-registration will fail.")
	}

	// If pulseURL is not provided, use the current request host
	if pulseURL == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		pulseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	} else {
		// Ensure derived pulseURL is still sanitized (should already be, but double check)
		if safeURL, err := sanitizeInstallerURL(pulseURL); err == nil {
			pulseURL = safeURL
		}
	}

	log.Info().
		Str("type", serverType).
		Str("host", serverHost).
		Bool("has_auth", h.getConfig(r.Context()).AuthUser != "" || h.getConfig(r.Context()).AuthPass != "" || h.getConfig(r.Context()).HasAPITokens()).
		Msg("HandleSetupScript called")

	// The setup script is now public - authentication happens via setup code
	// No need to check auth here since the script will prompt for a code

	// Default to PVE if not specified
	if serverType == "" {
		serverType = "pve"
	}

	// If pulse URL not provided, try to construct from request
	if pulseURL == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		pulseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	// Extract hostname/IP from the host URL if provided
	serverName := "your-server"
	if serverHost != "" {
		// Extract hostname/IP from URL
		if match := strings.Contains(serverHost, "://"); match {
			parts := strings.Split(serverHost, "://")
			if len(parts) > 1 {
				hostPart := strings.Split(parts[1], ":")[0]
				serverName = hostPart
			}
		} else {
			// Just a hostname/IP
			serverName = strings.Split(serverHost, ":")[0]
		}
	}

	// Extract Pulse IP from the pulse URL to make token name unique
	pulseIP := "pulse"
	if pulseURL != "" {
		// Extract IP/hostname from Pulse URL
		if match := strings.Contains(pulseURL, "://"); match {
			parts := strings.Split(pulseURL, "://")
			if len(parts) > 1 {
				hostPart := strings.Split(parts[1], ":")[0]
				// Replace dots with dashes for token name compatibility
				pulseIP = strings.ReplaceAll(hostPart, ".", "-")
			}
		}
	}

	// Create unique token name based on Pulse IP and timestamp
	// Adding timestamp ensures truly unique tokens even when running from same Pulse server
	timestamp := time.Now().Unix()
	tokenName := fmt.Sprintf("pulse-%s-%d", pulseIP, timestamp)

	// Log the token name for debugging
	log.Info().
		Str("pulseURL", pulseURL).
		Str("pulseIP", pulseIP).
		Str("tokenName", tokenName).
		Int64("timestamp", timestamp).
		Msg("Generated unique token name for setup script")

	// Get or generate SSH public key for temperature monitoring
	sshKeys := h.getOrGenerateSSHKeys()

	var script string

	if serverType == "pve" {
		// Build storage permissions command if needed
		storagePerms := ""
		if backupPerms {
			storagePerms = "\npveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin"
		}

		script = fmt.Sprintf(`#!/bin/bash
# Pulse Monitoring Setup Script for %s
# Generated: %s

echo "============================================"
echo "  Pulse Monitoring Setup for Proxmox VE"
echo "============================================"
echo ""

PULSE_URL="%s"
SERVER_HOST="%s"
TOKEN_NAME="%s"
PULSE_TOKEN_ID="pulse-monitor@pam!${TOKEN_NAME}"
SETUP_SCRIPT_URL="$PULSE_URL/api/setup-script?type=pve&host=$SERVER_HOST&pulse_url=$PULSE_URL"

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "Please run this script as root"
   exit 1
fi

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
        echo "  curl -sSL \"$SETUP_SCRIPT_URL\" | bash"
        echo ""
        exit 1
        ;;
    *)
        echo "This script requires Proxmox host tooling (pveum)."
        echo ""
        echo "Run on your Proxmox host:"
        echo "  curl -sSL \"$SETUP_SCRIPT_URL\" | bash"
        echo ""
        echo "Manual setup steps:"
        echo "  1. On Proxmox host, create API token:"
        echo "       pveum user add pulse-monitor@pam --comment \"Pulse monitoring service\""
        echo "       pveum aclmod / -user pulse-monitor@pam -role PVEAuditor"
        echo "       pveum user token add pulse-monitor@pam "$TOKEN_NAME" --privsep 0"
        echo ""
        echo "  2. In Pulse: Settings → Nodes → Add Node (enter token from above)"
        echo ""
        exit 1
        ;;
esac

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Main Menu
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "What would you like to do?"
echo ""
echo "  [1] Install/Configure - Set up Pulse monitoring"
echo "  [2] Remove All        - Uninstall everything Pulse has configured"
echo "  [3] Cancel            - Exit without changes"
echo ""
echo -n "Your choice [1/2/3]: "

MAIN_ACTION=""
if [ -t 0 ]; then
    read -n 1 -r MAIN_ACTION
else
    if read -n 1 -r MAIN_ACTION </dev/tty 2>/dev/null; then
        :
    else
        echo "(No terminal available - defaulting to Install)"
        MAIN_ACTION="1"
    fi
fi
echo ""
echo ""

# Handle Cancel
if [[ $MAIN_ACTION =~ ^[3Cc]$ ]]; then
    echo "Cancelled. No changes made."
    exit 0
fi

# Handle Remove All
if [[ $MAIN_ACTION =~ ^[2Rr]$ ]]; then
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


    # Always run manual removal for local services and files
    if true; then
        # Remove SSH keys from authorized_keys (only Pulse-managed entries)
        if [ -f /root/.ssh/authorized_keys ]; then
            echo "  • Removing SSH keys from authorized_keys..."
            TMP_AUTH_KEYS=$(mktemp)
            if [ -f "$TMP_AUTH_KEYS" ]; then
                grep -vF '# pulse-managed-key' /root/.ssh/authorized_keys > "$TMP_AUTH_KEYS" 2>/dev/null
                GREP_EXIT=$?
                if [ $GREP_EXIT -eq 0 ] || [ $GREP_EXIT -eq 1 ]; then
                    chmod --reference=/root/.ssh/authorized_keys "$TMP_AUTH_KEYS" 2>/dev/null || chmod 600 "$TMP_AUTH_KEYS"
                    chown --reference=/root/.ssh/authorized_keys "$TMP_AUTH_KEYS" 2>/dev/null || true
                    if mv "$TMP_AUTH_KEYS" /root/.ssh/authorized_keys; then
                        :
                    else
                        rm -f "$TMP_AUTH_KEYS"
                    fi
                else
                    rm -f "$TMP_AUTH_KEYS"
                fi
            fi
        fi

        # Remove Pulse monitoring API tokens and user
        echo "  • Removing Pulse monitoring API tokens and user..."
        if command -v pveum &> /dev/null; then
            TOKEN_LIST=$(pveum user token list pulse-monitor@pam 2>/dev/null | awk 'NR>3 {print $2}' | grep -v '^$' || printf '')
            if [ -n "$TOKEN_LIST" ]; then
                while IFS= read -r TOKEN; do
                    if [ -n "$TOKEN" ]; then
                        pveum user token remove pulse-monitor@pam "$TOKEN" 2>/dev/null || true
                    fi
                done <<< "$TOKEN_LIST"
            fi
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

# Extract Pulse server IP from the URL for token matching
PULSE_IP_PATTERN=$(echo "%s" | sed 's/\./\-/g')

# Check for old Pulse tokens from the same Pulse server and offer to clean them up
OLD_TOKENS=$(pveum user token list pulse-monitor@pam 2>/dev/null | grep -E "│ pulse-${PULSE_IP_PATTERN}-[0-9]+" | awk -F'│' '{print $2}' | sed 's/^ *//;s/ *$//' || true)
if [ ! -z "$OLD_TOKENS" ]; then
    echo "Checking for existing Pulse monitoring tokens from this Pulse server..."
    TOKEN_COUNT=$(echo "$OLD_TOKENS" | wc -l)
    echo ""
    echo "⚠️  Found $TOKEN_COUNT old Pulse monitoring token(s) from this Pulse server (${PULSE_IP_PATTERN}):"
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
                pveum user token remove pulse-monitor@pam "$TOKEN" 2>/dev/null && echo "   ✓ Removed token: $TOKEN" || echo "   ✗ Failed to remove: $TOKEN"
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
pveum user add pulse-monitor@pam --comment "Pulse monitoring service" 2>/dev/null || true

SETUP_AUTH_TOKEN="%s"
AUTO_REG_SUCCESS=false

attempt_auto_registration() {

    if [ -z "$SETUP_AUTH_TOKEN" ] && [ -n "$PULSE_SETUP_TOKEN" ]; then
        SETUP_AUTH_TOKEN="$PULSE_SETUP_TOKEN"
    fi

    if [ -z "$SETUP_AUTH_TOKEN" ]; then
        if [ -t 0 ]; then
            printf "Pulse setup token: "
            if command -v stty >/dev/null 2>&1; then stty -echo; fi
            IFS= read -r SETUP_AUTH_TOKEN
            if command -v stty >/dev/null 2>&1; then stty echo; fi
            printf "\n"
		elif [ -c /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]; then
			printf "Pulse setup token: " >/dev/tty
			if command -v stty >/dev/null 2>&1; then stty -echo </dev/tty 2>/dev/null || true; fi
			IFS= read -r SETUP_AUTH_TOKEN </dev/tty || true
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

    if [ -z "$SETUP_AUTH_TOKEN" ]; then
        echo "⚠️  Auto-registration skipped: no setup token provided"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
        return
    fi

    SERVER_HOSTNAME=$(hostname -s 2>/dev/null || hostname)
    SERVER_IP=$(hostname -I | awk '{print $1}')

    HOST_URL="$SERVER_HOST"
    if [ "$HOST_URL" = "https://YOUR_PROXMOX_HOST:8006" ] || [ -z "$HOST_URL" ]; then
        echo ""
        echo "❌ ERROR: No Proxmox host URL provided!"
        echo "   The setup script URL is missing the 'host' parameter."
        echo ""
        echo "   Please use the correct URL format:"
        echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pve&host=YOUR_PVE_URL&pulse_url=$PULSE_URL\" | bash"
        echo ""
        echo "   Example:"
        echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pve&host=https://192.168.0.5:8006&pulse_url=$PULSE_URL\" | bash"
        echo ""
        echo "📝 For manual setup, use the token created above with:"
        echo "   Token ID: $PULSE_TOKEN_ID"
        echo "   Token Value: [See above]"
        echo ""
        exit 1
    fi

    REGISTER_JSON='{"type":"pve","host":"'"$HOST_URL"'","serverName":"'"$SERVER_HOSTNAME"'","tokenId":"'"$PULSE_TOKEN_ID"'","tokenValue":"'"$TOKEN_VALUE"'","authToken":"'"$SETUP_AUTH_TOKEN"'"}'

    REGISTER_RESPONSE=$(echo "$REGISTER_JSON" | curl -s -X POST "$PULSE_URL/api/auto-register" \
        -H "Content-Type: application/json" \
        -d @- 2>&1)

    AUTO_REG_SUCCESS=false
    if echo "$REGISTER_RESPONSE" | grep -q "success"; then
        AUTO_REG_SUCCESS=true
        echo "Node registered successfully"
        echo ""
    else
        if echo "$REGISTER_RESPONSE" | grep -q "Authentication required"; then
            echo "Error: Auto-registration failed - authentication required"
            echo ""
            if [ -z "$PULSE_API_TOKEN" ]; then
                echo "To enable auto-registration, add your API token to the setup URL"
                echo "You can find your API token in Pulse Settings → Security"
            else
                echo "The provided API token was invalid"
            fi
        else
            echo "⚠️  Auto-registration failed. Manual configuration may be needed."
            echo "   Response: $REGISTER_RESPONSE"
        fi
        echo ""
        echo "📝 For manual setup:"
        echo "   1. Copy the token value shown above"
        echo "   2. Add this node manually in Pulse Settings"
    fi
}

# Generate API token
echo "Generating API token..."

# Check if token already exists
TOKEN_EXISTED=false
if pveum user token list pulse-monitor@pam 2>/dev/null | grep -q "$TOKEN_NAME"; then
    TOKEN_EXISTED=true
    echo ""
    echo "================================================================"
    echo "WARNING: Token '$TOKEN_NAME' already exists!"
    echo "================================================================"
    echo ""
    echo "To create a new token, first remove the existing one:"
    echo "  pveum user token remove pulse-monitor@pam $TOKEN_NAME"
    echo ""
    echo "Or create a token with a different name:"
    echo "  pveum user token add pulse-monitor@pam ${TOKEN_NAME}-$(date +%%s) --privsep 0"
    echo ""
    echo "Then use the new token ID in Pulse (e.g., ${PULSE_TOKEN_ID}-1234567890)"
    echo "================================================================"
    echo ""
else
    # Create token silently first
    TOKEN_OUTPUT=$(pveum user token add pulse-monitor@pam "$TOKEN_NAME" --privsep 0)
    
    # Extract the token value for auto-registration
    TOKEN_VALUE=$(echo "$TOKEN_OUTPUT" | grep "│ value" | awk -F'│' '{print $3}' | tr -d ' ' | tail -1)
    
    if [ -z "$TOKEN_VALUE" ]; then
        # If we can't extract the token, show it to the user
        echo ""
        echo "================================================================"
        echo "IMPORTANT: Copy the token value below - it's only shown once!"
        echo "================================================================"
        echo "$TOKEN_OUTPUT"
        echo "================================================================"
        echo ""
        echo "⚠️  Failed to extract token value from output."
        echo "   Manual registration may be required."
        echo ""
    else
        # Token created successfully
        echo "API token generated successfully"
        echo ""
    fi

fi

# Set up permissions
echo "Setting up permissions..."
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor%s

# Detect Proxmox version and apply appropriate permissions
# Method 1: Try to check if VM.Monitor exists (reliable for PVE 8 and below)
HAS_VM_MONITOR=false
if pveum role list 2>/dev/null | grep -q "VM.Monitor" || 
   pveum role add TestMonitor -privs VM.Monitor 2>/dev/null; then
    HAS_VM_MONITOR=true
    pveum role delete TestMonitor 2>/dev/null || true
fi

# Detect availability of newer guest agent privileges (PVE 9+)
HAS_VM_GUEST_AGENT_AUDIT=false
if pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then
    HAS_VM_GUEST_AGENT_AUDIT=true
else
    if pveum role add TestGuestAgentAudit -privs VM.GuestAgent.Audit 2>/dev/null; then
        HAS_VM_GUEST_AGENT_AUDIT=true
        pveum role delete TestGuestAgentAudit 2>/dev/null || true
    fi
fi

# Detect availability of Sys.Audit (needed for Ceph metrics)
HAS_SYS_AUDIT=false
if pveum role list 2>/dev/null | grep -q "Sys.Audit"; then
    HAS_SYS_AUDIT=true
else
    if pveum role add TestSysAudit -privs Sys.Audit 2>/dev/null; then
        HAS_SYS_AUDIT=true
        pveum role delete TestSysAudit 2>/dev/null || true
    fi
fi

# Method 2: Try to detect PVE version directly
PVE_VERSION=""
if command -v pveversion >/dev/null 2>&1; then
    # Extract major version (e.g., "9" from "pve-manager/9.0.5/...")
    PVE_VERSION=$(pveversion --verbose 2>/dev/null | grep "pve-manager" | awk -F'/' '{print $2}' | cut -d'.' -f1)
fi

EXTRA_PRIVS=()

if [ "$HAS_SYS_AUDIT" = true ]; then
    EXTRA_PRIVS+=("Sys.Audit")
fi

if [ "$HAS_VM_MONITOR" = true ]; then
    # PVE 8 or below - VM.Monitor exists
    EXTRA_PRIVS+=("VM.Monitor")
elif [ "$HAS_VM_GUEST_AGENT_AUDIT" = true ]; then
    # PVE 9+ - VM.Monitor removed, prefer VM.GuestAgent.Audit for guest data
    EXTRA_PRIVS+=("VM.GuestAgent.Audit")
fi

if [ ${#EXTRA_PRIVS[@]} -gt 0 ]; then
    PRIV_STRING="${EXTRA_PRIVS[*]}"
    pveum role delete PulseMonitor 2>/dev/null || true
    if pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null; then
        pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
        echo "  • Applied privileges: $PRIV_STRING"
    else
        echo "  • Failed to create PulseMonitor role with: $PRIV_STRING"
        echo "    Assign these privileges manually if Pulse reports permission errors."
    fi
else
    echo "  • No additional privileges detected. Pulse may show limited VM metrics."
fi

attempt_auto_registration

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Temperature Monitoring Setup (Optional)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

SSH_SENSORS_PUBLIC_KEY="%s"
SSH_SENSORS_KEY_ENTRY="command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $SSH_SENSORS_PUBLIC_KEY # pulse-sensors"
TEMPERATURE_ENABLED=false

if [ -n "$SSH_SENSORS_PUBLIC_KEY" ]; then
    echo "📊 Enable Temperature Monitoring?"
    echo ""
    echo "Collect CPU and drive temperatures via secure SSH connection."
    echo ""
    echo "Security:"
    echo "  • SSH key authentication with forced command (sensors -j only)"
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

        # Add key to root's authorized_keys
        mkdir -p /root/.ssh
        chmod 700 /root/.ssh

        # Remove any old pulse keys
        if [ -f /root/.ssh/authorized_keys ]; then
            grep -vF "# pulse-" /root/.ssh/authorized_keys > /root/.ssh/authorized_keys.tmp 2>/dev/null || touch /root/.ssh/authorized_keys.tmp
            mv /root/.ssh/authorized_keys.tmp /root/.ssh/authorized_keys
        fi

        echo "$SSH_SENSORS_KEY_ENTRY" >> /root/.ssh/authorized_keys
        chmod 600 /root/.ssh/authorized_keys
        echo "  ✓ Sensors key configured (restricted to sensors -j)"

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

        echo ""
        if [ "$TEMPERATURE_SETUP_SUCCESS" = true ]; then
            echo "✓ Temperature monitoring enabled"
            if [ "$IS_RPI" = true ]; then
                echo "  Using Raspberry Pi native temperature interface"
            fi
            echo "  Temperature data will appear in the dashboard within 10 seconds"
            TEMPERATURE_ENABLED=true
        else
            echo "⚠️  Temperature monitoring setup incomplete"
            echo "  You can re-run this script after installing lm-sensors"
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
echo "Node successfully registered with Pulse monitoring."
echo "Data will appear in your dashboard within 10 seconds."
echo ""

# Only show manual setup instructions if auto-registration failed
if [ "$AUTO_REG_SUCCESS" != true ]; then
    echo "Manual setup instructions:"
    echo "  Token ID: $PULSE_TOKEN_ID"
    if [ "$TOKEN_EXISTED" = true ]; then
        echo "  Token Value: [Use your existing token or create a new one as shown above]"
    elif [ -n "$TOKEN_VALUE" ]; then
        echo "  Token Value: $TOKEN_VALUE"
    else
        echo "  Token Value: [See token output above]"
    fi
    echo "  Host URL: YOUR_PROXMOX_HOST:8006"
echo ""
fi
`, serverName, time.Now().Format("2006-01-02 15:04:05"),
			pulseURL, serverHost, tokenName,
			pulseIP,
			authToken,
			storagePerms,
			sshKeys.SensorsPublicKey)

	} else { // PBS
		script = fmt.Sprintf(`#!/bin/bash
# Pulse Monitoring Setup Script for PBS %s
# Generated: %s

echo "============================================"
echo "  Pulse Monitoring Setup for PBS"
echo "============================================"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "Please run this script as root"
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

# Extract Pulse server IP from the URL for token matching
PULSE_IP_PATTERN=$(echo "%s" | sed 's/\./\-/g')

# Check for old Pulse tokens from the same Pulse server and offer to clean them up
echo "Checking for existing Pulse monitoring tokens from this Pulse server..."
# PBS outputs tokens differently than PVE - extract just the token names matching this Pulse server
OLD_TOKENS=$(proxmox-backup-manager user list-tokens pulse-monitor@pbs 2>/dev/null | grep -oE "pulse-${PULSE_IP_PATTERN}-[0-9]+" | sort -u || true)
if [ ! -z "$OLD_TOKENS" ]; then
    TOKEN_COUNT=$(echo "$OLD_TOKENS" | wc -l)
    echo ""
    echo "⚠️  Found $TOKEN_COUNT old Pulse monitoring token(s) from this Pulse server (${PULSE_IP_PATTERN}):"
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

# Check if token already exists (PBS tokens can be regenerated with same name)
echo ""
echo "================================================================"
echo "IMPORTANT: Copy the token value below - it's only shown once!"
echo "================================================================"
TOKEN_OUTPUT=$(proxmox-backup-manager user generate-token pulse-monitor@pbs %s 2>&1)
if echo "$TOKEN_OUTPUT" | grep -q "already exists"; then
    echo "WARNING: Token '%s' already exists!"
    echo ""
    echo "You can either:"
    echo "1. Delete the existing token first:"
    echo "   proxmox-backup-manager user delete-token pulse-monitor@pbs %s"
    echo ""
    echo "2. Or create a token with a different name:"
    echo "   proxmox-backup-manager user generate-token pulse-monitor@pbs %s-$(date +%%s)"
    echo ""
    echo "Then use the new token ID in Pulse (e.g., pulse-monitor@pbs!%s-1234567890)"
else
    echo "$TOKEN_OUTPUT"
    
    # Extract the token value for auto-registration
    TOKEN_VALUE=$(echo "$TOKEN_OUTPUT" | grep '"value"' | sed 's/.*"value"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    
    if [ -z "$TOKEN_VALUE" ]; then
        echo "⚠️  Failed to extract token value from output."
        echo "   Manual registration may be required."
        echo ""
    else
        echo "✅ Token created for Pulse monitoring"
        echo ""
    fi
    
    # Try auto-registration
    echo "🔄 Attempting auto-registration with Pulse..."
    echo ""
    
    # Use auth token from URL parameter when provided (automation workflows)
    AUTH_TOKEN="%s"

    # Allow non-interactive override via environment variable
    if [ -z "$AUTH_TOKEN" ] && [ -n "$PULSE_SETUP_TOKEN" ]; then
        AUTH_TOKEN="$PULSE_SETUP_TOKEN"
    fi

    # Prompt the operator if we still don't have a token and a TTY is available
    if [ -z "$AUTH_TOKEN" ]; then
        if [ -t 0 ]; then
            printf "Pulse setup token: "
            if command -v stty >/dev/null 2>&1; then stty -echo; fi
            IFS= read -r AUTH_TOKEN
            if command -v stty >/dev/null 2>&1; then stty echo; fi
            printf "\n"
		elif [ -c /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]; then
			printf "Pulse setup token: " >/dev/tty
			if command -v stty >/dev/null 2>&1; then stty -echo </dev/tty 2>/dev/null || true; fi
			IFS= read -r AUTH_TOKEN </dev/tty || true
			if command -v stty >/dev/null 2>&1; then stty echo </dev/tty 2>/dev/null || true; fi
			printf "\n" >/dev/tty
		fi
	fi

    # Only proceed with auto-registration if we have an auth token
    if [ -n "$AUTH_TOKEN" ]; then
        # Get the server's hostname (short form to match Pulse node names)
        SERVER_HOSTNAME=$(hostname -s 2>/dev/null || hostname)
        SERVER_IP=$(hostname -I | awk '{print $1}')
        
        # Send registration to Pulse
        PULSE_URL="%s"
        
        # Check if host URL was provided
        HOST_URL="%s"
        if [ "$HOST_URL" = "https://YOUR_PBS_HOST:8007" ] || [ -z "$HOST_URL" ]; then
            echo ""
            echo "❌ ERROR: No PBS host URL provided!"
            echo "   The setup script URL is missing the 'host' parameter."
            echo ""
            echo "   Please use the correct URL format:"
            echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pbs&host=YOUR_PBS_URL&pulse_url=$PULSE_URL\" | bash"
            echo ""
            echo "   Example:"
            echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pbs&host=https://192.168.0.8:8007&pulse_url=$PULSE_URL\" | bash"
            echo ""
            echo "📝 For manual setup, use the token created above with:"
            echo "   Token ID: pulse-monitor@pbs!%s"
            echo "   Token Value: [See above]"
            echo ""
            exit 1
        fi
        
        # Construct registration request with setup code
        REGISTER_JSON=$(cat <<EOF
{
  "type": "pbs",
  "host": "$HOST_URL",
  "serverName": "$SERVER_HOSTNAME",
  "tokenId": "pulse-monitor@pbs!%s",
  "tokenValue": "$TOKEN_VALUE",
  "authToken": "$AUTH_TOKEN"
}
EOF
        )
        # Remove newlines from JSON
        REGISTER_JSON=$(echo "$REGISTER_JSON" | tr -d '\n')
        
        # Send registration with setup code
        REGISTER_RESPONSE=$(curl -s -X POST "$PULSE_URL/api/auto-register" \
            -H "Content-Type: application/json" \
            -d "$REGISTER_JSON" 2>&1)
    else
        echo "⚠️  Auto-registration skipped: no setup token provided"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
    fi
    
    AUTO_REG_SUCCESS=false
    if echo "$REGISTER_RESPONSE" | grep -q "success"; then
        AUTO_REG_SUCCESS=true
        echo "✅ Successfully registered with Pulse!"
    else
        if echo "$REGISTER_RESPONSE" | grep -q "Authentication required"; then
            echo "Error: Auto-registration failed - authentication required"
            echo ""
            if [ -z "$PULSE_API_TOKEN" ]; then
                echo "To enable auto-registration, add your API token to the setup URL"
                echo "You can find your API token in Pulse Settings → Security"
            else
                echo "The provided API token was invalid"
            fi
        else
            echo "⚠️  Auto-registration failed. Manual configuration may be needed."
            echo "   Response: $REGISTER_RESPONSE"
        fi
        echo ""
        echo "📝 For manual setup:"
        echo "   1. Copy the token value shown above"
        echo "   2. Add this node manually in Pulse Settings"
    fi
    echo ""
fi
echo "================================================================"
echo ""

# Set up permissions
echo "Setting up permissions..."
proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs
proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!%s'

echo ""
echo "✅ Setup complete!"
echo ""

# Only show manual setup instructions if auto-registration failed
if [ "$AUTO_REG_SUCCESS" != true ]; then
    echo "Add this server to Pulse with:"
    echo "  Token ID: pulse-monitor@pbs!%s"
    echo "  Token Value: [Check the output above for the token or instructions]"
    echo "  Host URL: https://$SERVER_IP:8007"
    echo ""
    echo "If auto-registration is enabled but requires a token:"
    echo "  1. Generate a registration token in Pulse Settings → Security"
    echo "  2. Re-run this script with: PULSE_REG_TOKEN=your-token ./setup.sh"
    echo ""
fi
`, serverName, time.Now().Format("2006-01-02 15:04:05"), pulseIP,
			tokenName, tokenName, tokenName, tokenName, tokenName,
			authToken, pulseURL, serverHost, tokenName, tokenName, tokenName, tokenName)
	}

	// Set headers for script download
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=pulse-setup-%s.sh", serverType))
	w.Write([]byte(script))
}

// generateSetupCode generates a secure hex token that satisfies sanitizeSetupAuthToken.
func (h *ConfigHandlers) generateSetupCode() string {
	// 16 bytes => 32 hex characters which matches the sanitizer's lower bound.
	const tokenBytes = 16
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	// rand.Read should never fail, but if it does fall back to timestamp-based token.
	log.Warn().Msg("fallback setup token generator used due to entropy failure")
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// HandleSetupScriptURL generates a one-time setup code and URL for the setup script
func (h *ConfigHandlers) handleSetupScriptURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	// Parse request
	var req struct {
		Type        string `json:"type"`
		Host        string `json:"host"`
		BackupPerms bool   `json:"backupPerms"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Generate a temporary auth token (simpler than setup codes)
	token := h.generateSetupCode() // Reuse the generation function
	tokenHash := internalauth.HashAPIToken(token)

	// Store the token with expiry (5 minutes)
	expiry := time.Now().Add(5 * time.Minute)
	h.codeMutex.Lock()
	h.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: expiry,
		Used:      false,
		NodeType:  req.Type,
		Host:      req.Host,
		OrgID:     GetOrgID(r.Context()),
	}
	h.codeMutex.Unlock()

	log.Info().
		Str("token_hash", safePrefixForLog(tokenHash, 8)+"...").
		Time("expiry", expiry).
		Str("type", req.Type).
		Msg("Generated temporary auth token")

	// Build the URL with the token included
	host := r.Host

	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		if (parsedHost == "127.0.0.1" || parsedHost == "localhost") && parsedPort == strconv.Itoa(h.getConfig(r.Context()).FrontendPort) {
			// Prefer a user-configured public URL when we're running on loopback.
			if publicURL := strings.TrimSpace(h.getConfig(r.Context()).PublicURL); publicURL != "" {
				if parsedURL, err := url.Parse(publicURL); err == nil && parsedURL.Host != "" {
					host = parsedURL.Host
				}
			}
		}
	}

	// Detect protocol - check both TLS and proxy headers
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	pulseURL := fmt.Sprintf("%s://%s", scheme, host)

	encodedHost := ""
	if req.Host != "" {
		encodedHost = "&host=" + url.QueryEscape(req.Host)
	}

	backupPerms := ""
	if req.BackupPerms {
		backupPerms = "&backup_perms=true"
	}

	// Build script URL (setup token is passed via environment variable).
	scriptURL := fmt.Sprintf("%s/api/setup-script?type=%s%s&pulse_url=%s%s",
		pulseURL, req.Type, encodedHost, pulseURL, backupPerms)

	// Return a curl command; the setup token is passed via environment variable.
	// The setup token is returned separately so the script can prompt the user.
	tokenHint := token
	if len(token) > 6 {
		tokenHint = fmt.Sprintf("%s…%s", token[:3], token[len(token)-3:])
	}

	command := fmt.Sprintf(`curl -sSL "%s" | PULSE_SETUP_TOKEN=%s bash`, scriptURL, token)

	response := map[string]interface{}{
		"url":               scriptURL,
		"command":           command,
		"expires":           expiry.Unix(),
		"setupToken":        token,
		"tokenHint":         tokenHint,
		"commandWithEnv":    command,
		"commandWithoutEnv": fmt.Sprintf(`curl -sSL "%s" | bash`, scriptURL),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AutoRegisterRequest represents a request from the setup script or agent to auto-register a node
type AutoRegisterRequest struct {
	Type       string `json:"type"`                 // "pve" or "pbs"
	Host       string `json:"host"`                 // The host URL
	TokenID    string `json:"tokenId"`              // Full token ID like pulse-monitor@pam!pulse-token
	TokenValue string `json:"tokenValue,omitempty"` // The token value for the node
	ServerName string `json:"serverName"`           // Hostname or IP
	SetupCode  string `json:"setupCode,omitempty"`  // One-time setup code for authentication (deprecated)
	AuthToken  string `json:"authToken,omitempty"`  // Direct auth token from URL (new approach)
	Source     string `json:"source,omitempty"`     // "agent" or "script" - indicates how the node was registered
	// New secure fields
	RequestToken bool   `json:"requestToken,omitempty"` // If true, Pulse will generate and return a token
	Username     string `json:"username,omitempty"`     // Username for creating token (e.g., "root@pam")
	Password     string `json:"password,omitempty"`     // Password for authentication (never stored)
}

// HandleAutoRegister receives token details from the setup script and auto-configures the node
func (h *ConfigHandlers) handleAutoRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body first to get the setup code
	var req AutoRegisterRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse auto-register request")
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Check authentication - require either setup code or API token if auth is enabled
	authenticated := false

	// Support both setupCode (old) and authToken (new) fields
	authCode := req.SetupCode
	if req.AuthToken != "" {
		authCode = req.AuthToken
	}

	log.Debug().
		Bool("hasAuthToken", strings.TrimSpace(req.AuthToken) != "").
		Bool("hasSetupCode", strings.TrimSpace(authCode) != "").
		Bool("hasConfigToken", h.getConfig(r.Context()).HasAPITokens()).
		Msg("Checking authentication for auto-register")

	// First check for setup code/auth token in the request
	if authCode != "" {
		matchedAPIToken := false
		if h.getConfig(r.Context()).HasAPITokens() {
			if record, ok := h.getConfig(r.Context()).ValidateAPIToken(authCode); ok {
				// Accept settings:write (admin tokens) or host-agent:report (agent tokens)
				if record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeHostReport) {
					authenticated = true
					matchedAPIToken = true
					log.Info().
						Str("type", req.Type).
						Str("host", req.Host).
						Msg("Auto-register authenticated via direct API token")
				} else {
					log.Warn().
						Str("type", req.Type).
						Str("host", req.Host).
						Msg("Auto-register rejected: API token missing required scope")
				}
			}
		}

		if !matchedAPIToken {
			// Not the API token, check if it's a temporary setup code
			codeHash := internalauth.HashAPIToken(authCode)
			log.Debug().
				Bool("hasAuthCode", true).
				Str("codeHash", safePrefixForLog(codeHash, 8)+"...").
				Msg("Checking auth token as setup code")
			h.codeMutex.Lock()
			setupCode, exists := h.setupCodes[codeHash]
			log.Debug().
				Bool("exists", exists).
				Int("totalCodes", len(h.setupCodes)).
				Msg("Setup code lookup result")
			if exists && !setupCode.Used && time.Now().Before(setupCode.ExpiresAt) {
				// Validate that the code matches the node type
				// Note: We don't validate the host anymore as it may differ between
				// what's entered in the UI and what's provided in the setup script URL
				if setupCode.NodeType == req.Type {
					setupCode.Used = true // Mark as used immediately

					// Inject OrgID from setup code into context for subsequent processing
					if setupCode.OrgID != "" {
						ctx := context.WithValue(r.Context(), OrgIDContextKey, setupCode.OrgID)
						r = r.WithContext(ctx)
					}
					// Allow a short grace period for follow-up actions without keeping tokens alive too long
					graceExpiry := time.Now().Add(1 * time.Minute)
					if setupCode.ExpiresAt.Before(graceExpiry) {
						graceExpiry = setupCode.ExpiresAt
					}
					h.recentSetupTokens[codeHash] = graceExpiry
					authenticated = true
					log.Info().
						Str("type", req.Type).
						Str("host", req.Host).
						Bool("via_authToken", req.AuthToken != "").
						Msg("Auto-register authenticated via setup code/token")
				} else {
					log.Warn().
						Str("expected_type", setupCode.NodeType).
						Str("got_type", req.Type).
						Msg("Setup code validation failed - type mismatch")
				}
			} else if exists && setupCode.Used {
				log.Warn().Msg("Setup code already used")
			} else if exists {
				log.Warn().Msg("Setup code expired")
			} else {
				log.Warn().Msg("Invalid setup code/token - not in setup codes map")
			}
			h.codeMutex.Unlock()
		}
	}

	// If not authenticated via setup code, check API token if configured
	if !authenticated && h.getConfig(r.Context()).HasAPITokens() {
		apiToken := r.Header.Get("X-API-Token")
		if record, ok := h.getConfig(r.Context()).ValidateAPIToken(apiToken); ok {
			// Accept settings:write (admin tokens) or host-agent:report (agent tokens)
			if record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeHostReport) {
				authenticated = true
				log.Info().Msg("Auto-register authenticated via API token")
			} else {
				log.Warn().Msg("Auto-register rejected: API token missing required scope")
			}
		}
	}

	// Abort when no authentication succeeded. This applies even when API tokens
	// are not configured to ensure one-time setup tokens are always required.
	if !authenticated {
		log.Warn().
			Str("ip", r.RemoteAddr).
			Bool("has_auth_code", authCode != "").
			Msg("Unauthorized auto-register attempt rejected")

		if authCode == "" && r.Header.Get("X-API-Token") == "" {
			http.Error(w, "Pulse requires authentication", http.StatusUnauthorized)
		} else {
			http.Error(w, "Invalid or expired setup code", http.StatusUnauthorized)
		}
		return
	}

	// Log source IP for security auditing
	clientIP := r.RemoteAddr
	// Only trust X-Forwarded-For if request comes from a trusted proxy
	peerIP := extractRemoteIP(clientIP)
	if isTrustedProxyIP(peerIP) {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}
	}
	log.Info().Str("clientIP", clientIP).Msg("Auto-register request from")

	// Registration token validation removed - feature deprecated

	log.Info().
		Str("type", req.Type).
		Str("host", req.Host).
		Str("tokenId", req.TokenID).
		Bool("hasTokenValue", req.TokenValue != "").
		Str("serverName", req.ServerName).
		Msg("Processing auto-register request")

	// Check if this is a new secure registration request
	if req.RequestToken {
		// New secure mode - generate token on Pulse side
		if req.Type == "" || req.Host == "" || req.Username == "" || req.Password == "" {
			log.Error().
				Str("type", req.Type).
				Str("host", req.Host).
				Bool("hasUsername", req.Username != "").
				Bool("hasPassword", req.Password != "").
				Msg("Missing required fields for secure registration")
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}
		// Handle secure registration
		h.handleSecureAutoRegister(w, r, &req, clientIP)
		return
	}

	// Legacy mode - validate old required fields
	if req.Type == "" || req.Host == "" || req.TokenID == "" || req.TokenValue == "" {
		log.Error().
			Str("type", req.Type).
			Str("host", req.Host).
			Str("tokenId", req.TokenID).
			Bool("hasToken", req.TokenValue != "").
			Msg("Missing required fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	host, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fingerprint := ""
	if fp, err := tlsutil.FetchFingerprint(host); err != nil {
		log.Warn().Err(err).Str("host", host).Msg("Failed to fetch TLS fingerprint for auto-register")
	} else {
		fingerprint = fp
	}

	// Create a node configuration
	boolFalse := false
	boolTrue := true
	verifySSL := true
	nodeConfig := NodeConfigRequest{
		Type:               req.Type,
		Name:               req.ServerName,
		Host:               host, // Use normalized host
		TokenName:          req.TokenID,
		TokenValue:         req.TokenValue,
		Fingerprint:        fingerprint,
		VerifySSL:          &verifySSL,
		MonitorVMs:         &boolTrue,
		MonitorContainers:  &boolTrue,
		MonitorStorage:     &boolTrue,
		MonitorBackups:     &boolTrue,
		MonitorDatastores:  &boolTrue,
		MonitorSyncJobs:    &boolTrue,
		MonitorVerifyJobs:  &boolTrue,
		MonitorPruneJobs:   &boolTrue,
		MonitorGarbageJobs: &boolFalse,
	}

	// Check if a node with this host already exists
	// IMPORTANT: Match by Host URL primarily.
	// Also match by name+tokenID for DHCP scenarios where IP changed but it's the same host.
	// Different physical hosts can have the same hostname (e.g., "px1" on different networks)
	// but they'll have different tokens, so we only merge if BOTH name AND token match.
	// See: Issue #891, #104, #924, #940 and multiple fix attempts in Dec 2025.
	existingIndex := -1
	preserveHost := false // When true, keep user's configured hostname instead of overwriting with agent's IP

	// Extract IP from the new host URL for DNS comparison
	newHostIP := extractHostIP(host)

	if req.Type == "pve" {
		for i, node := range h.getConfig(r.Context()).PVEInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
			// DHCP case: same hostname AND same token = same physical host with new IP
			// This allows IP changes to update existing nodes without creating duplicates
			if req.ServerName != "" && strings.EqualFold(node.Name, req.ServerName) && node.TokenName == req.TokenID {
				existingIndex = i
				// Update the host to the new IP
				log.Info().
					Str("oldHost", node.Host).
					Str("newHost", host).
					Str("node", req.ServerName).
					Msg("Detected IP change for existing node - updating host")
				break
			}
			// Agent registration: check if existing hostname resolves to the new IP
			// This catches the case where a node was manually added by hostname and
			// then the agent registers using the IP address. (Issue #924)
			// We preserve the user's configured hostname instead of overwriting with IP. (Issue #940)
			if req.Source == "agent" && newHostIP != "" {
				existingHostIP := extractHostIP(node.Host)
				if existingHostIP == "" {
					// Existing config uses hostname, try to resolve it
					existingHostIP = resolveHostnameToIP(node.Host)
				}
				if existingHostIP == newHostIP {
					existingIndex = i
					preserveHost = true // Keep user's configured hostname
					log.Info().
						Str("existingHost", node.Host).
						Str("newHost", host).
						Str("resolvedIP", newHostIP).
						Str("node", node.Name).
						Msg("Agent registration detected existing node by IP resolution - preserving configured hostname")
					break
				}
			}
		}
	} else {
		for i, node := range h.getConfig(r.Context()).PBSInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
			// DHCP case: same hostname AND same token = same physical host with new IP
			if req.ServerName != "" && strings.EqualFold(node.Name, req.ServerName) && node.TokenName == req.TokenID {
				existingIndex = i
				log.Info().
					Str("oldHost", node.Host).
					Str("newHost", host).
					Str("node", req.ServerName).
					Msg("Detected IP change for existing node - updating host")
				break
			}
			// Agent registration: check if existing hostname resolves to the new IP
			// We preserve the user's configured hostname instead of overwriting with IP. (Issue #940)
			if req.Source == "agent" && newHostIP != "" {
				existingHostIP := extractHostIP(node.Host)
				if existingHostIP == "" {
					existingHostIP = resolveHostnameToIP(node.Host)
				}
				if existingHostIP == newHostIP {
					existingIndex = i
					preserveHost = true // Keep user's configured hostname
					log.Info().
						Str("existingHost", node.Host).
						Str("newHost", host).
						Str("resolvedIP", newHostIP).
						Str("node", node.Name).
						Msg("Agent registration detected existing node by IP resolution - preserving configured hostname")
					break
				}
			}
		}
	}

	// If node exists, update it; otherwise add new
	if existingIndex >= 0 {
		// Update existing node
		if req.Type == "pve" {
			instance := &h.getConfig(r.Context()).PVEInstances[existingIndex]
			// Update host in case IP changed (DHCP scenario)
			// But preserve user's configured hostname when matched by IP resolution (Issue #940)
			if !preserveHost {
				instance.Host = host
			}
			// Clear password auth when switching to token auth
			instance.User = ""
			instance.Password = ""
			instance.TokenName = nodeConfig.TokenName
			instance.TokenValue = nodeConfig.TokenValue
			// Update source if provided (allows upgrade from script to agent)
			if req.Source != "" {
				instance.Source = req.Source
			}

			// Check for cluster if not already detected
			if !instance.IsCluster {
				clientConfig := proxmox.ClientConfig{
					Host:       instance.Host,
					TokenName:  nodeConfig.TokenName,
					TokenValue: nodeConfig.TokenValue,
					VerifySSL:  instance.VerifySSL,
				}

				isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, instance.Name, instance.ClusterEndpoints)
				if isCluster {
					instance.IsCluster = true
					instance.ClusterName = clusterName
					instance.ClusterEndpoints = clusterEndpoints
					log.Info().
						Str("cluster", clusterName).
						Int("endpoints", len(clusterEndpoints)).
						Msg("Detected Proxmox cluster during auto-registration update")
				}
			}
			// Keep other settings as they were
		} else {
			instance := &h.getConfig(r.Context()).PBSInstances[existingIndex]
			// Update host in case IP changed (DHCP scenario)
			// But preserve user's configured hostname when matched by IP resolution (Issue #940)
			if !preserveHost {
				instance.Host = host
			}
			// Clear password auth when switching to token auth
			instance.User = ""
			instance.Password = ""
			instance.TokenName = nodeConfig.TokenName
			instance.TokenValue = nodeConfig.TokenValue
			// Update source if provided (allows upgrade from script to agent)
			if req.Source != "" {
				instance.Source = req.Source
			}
			// Keep other settings as they were
		}
		log.Info().
			Str("host", req.Host).
			Str("type", req.Type).
			Str("tokenName", nodeConfig.TokenName).
			Bool("hasTokenValue", nodeConfig.TokenValue != "").
			Msg("Updated existing node with new token")
	} else {
		// Add new node
		if req.Type == "pve" {
			// Check for cluster detection using helper
			verifySSL := false
			if nodeConfig.VerifySSL != nil {
				verifySSL = *nodeConfig.VerifySSL
			}
			clientConfig := proxmox.ClientConfig{
				Host:       nodeConfig.Host,
				TokenName:  nodeConfig.TokenName,
				TokenValue: nodeConfig.TokenValue,
				VerifySSL:  verifySSL,
			}

			isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, nodeConfig.Name, nil)

			// CLUSTER DEDUPLICATION: Check if we already have this cluster configured
			// If so, merge this node as an endpoint instead of creating a duplicate instance
			if isCluster && clusterName != "" {
				for i := range h.getConfig(r.Context()).PVEInstances {
					existingInstance := &h.getConfig(r.Context()).PVEInstances[i]
					if existingInstance.IsCluster && existingInstance.ClusterName == clusterName {
						// Found existing cluster with same name - merge endpoints!
						log.Info().
							Str("cluster", clusterName).
							Str("existingInstance", existingInstance.Name).
							Str("newNode", nodeConfig.Name).
							Msg("Auto-registered node belongs to already-configured cluster - merging endpoints")

						// Merge any new endpoints from the detected cluster
						existingEndpointMap := make(map[string]bool)
						for _, ep := range existingInstance.ClusterEndpoints {
							existingEndpointMap[ep.NodeName] = true
						}
						for _, newEp := range clusterEndpoints {
							if !existingEndpointMap[newEp.NodeName] {
								existingInstance.ClusterEndpoints = append(existingInstance.ClusterEndpoints, newEp)
								log.Info().
									Str("cluster", clusterName).
									Str("endpoint", newEp.NodeName).
									Msg("Added new endpoint to existing cluster via auto-registration")
							}
						}

						// Save and reload
						if h.getPersistence(r.Context()) != nil {
							if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
								log.Warn().Err(err).Msg("Failed to persist cluster endpoint merge during auto-registration")
							}
						}
						if h.reloadFunc != nil {
							if err := h.reloadFunc(); err != nil {
								log.Warn().Err(err).Msg("Failed to reload monitor after cluster merge during auto-registration")
							}
						}

						// Return success - merged into existing cluster
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success":        true,
							"merged":         true,
							"cluster":        clusterName,
							"existingNode":   existingInstance.Name,
							"message":        fmt.Sprintf("Agent merged into existing cluster '%s'", clusterName),
							"totalEndpoints": len(existingInstance.ClusterEndpoints),
						})
						return
					}
				}
			}

			monitorVMs := true
			if nodeConfig.MonitorVMs != nil {
				monitorVMs = *nodeConfig.MonitorVMs
			}
			monitorContainers := true
			if nodeConfig.MonitorContainers != nil {
				monitorContainers = *nodeConfig.MonitorContainers
			}
			monitorStorage := true
			if nodeConfig.MonitorStorage != nil {
				monitorStorage = *nodeConfig.MonitorStorage
			}
			monitorBackups := true
			if nodeConfig.MonitorBackups != nil {
				monitorBackups = *nodeConfig.MonitorBackups
			}

			// Disambiguate node name if duplicate hostnames exist
			displayName := h.disambiguateNodeName(r.Context(), nodeConfig.Name, nodeConfig.Host, "pve")

			newInstance := config.PVEInstance{
				Name:              displayName,
				Host:              nodeConfig.Host,
				TokenName:         nodeConfig.TokenName,
				TokenValue:        nodeConfig.TokenValue,
				VerifySSL:         verifySSL,
				MonitorVMs:        monitorVMs,
				MonitorContainers: monitorContainers,
				MonitorStorage:    monitorStorage,
				MonitorBackups:    monitorBackups,
				IsCluster:         isCluster,
				ClusterName:       clusterName,
				ClusterEndpoints:  clusterEndpoints,
				Source:            req.Source, // Track how this node was registered
			}
			h.getConfig(r.Context()).PVEInstances = append(h.getConfig(r.Context()).PVEInstances, newInstance)

			if isCluster {
				log.Info().
					Str("cluster", clusterName).
					Int("endpoints", len(clusterEndpoints)).
					Msg("Added new Proxmox cluster via auto-registration")
			}
		} else {
			verifySSL := false
			if nodeConfig.VerifySSL != nil {
				verifySSL = *nodeConfig.VerifySSL
			}
			monitorDatastores := false
			if nodeConfig.MonitorDatastores != nil {
				monitorDatastores = *nodeConfig.MonitorDatastores
			}
			monitorSyncJobs := false
			if nodeConfig.MonitorSyncJobs != nil {
				monitorSyncJobs = *nodeConfig.MonitorSyncJobs
			}
			monitorVerifyJobs := false
			if nodeConfig.MonitorVerifyJobs != nil {
				monitorVerifyJobs = *nodeConfig.MonitorVerifyJobs
			}
			monitorPruneJobs := false
			if nodeConfig.MonitorPruneJobs != nil {
				monitorPruneJobs = *nodeConfig.MonitorPruneJobs
			}
			monitorGarbageJobs := false
			if nodeConfig.MonitorGarbageJobs != nil {
				monitorGarbageJobs = *nodeConfig.MonitorGarbageJobs
			}

			// Disambiguate node name if duplicate hostnames exist
			pbsDisplayName := h.disambiguateNodeName(r.Context(), nodeConfig.Name, nodeConfig.Host, "pbs")

			newInstance := config.PBSInstance{
				Name:               pbsDisplayName,
				Host:               nodeConfig.Host,
				TokenName:          nodeConfig.TokenName,
				TokenValue:         nodeConfig.TokenValue,
				VerifySSL:          verifySSL,
				MonitorBackups:     true, // Enable by default for PBS
				MonitorDatastores:  monitorDatastores,
				MonitorSyncJobs:    monitorSyncJobs,
				MonitorVerifyJobs:  monitorVerifyJobs,
				MonitorPruneJobs:   monitorPruneJobs,
				MonitorGarbageJobs: monitorGarbageJobs,
				Source:             req.Source, // Track how this node was registered
			}
			h.getConfig(r.Context()).PBSInstances = append(h.getConfig(r.Context()).PBSInstances, newInstance)
		}
		log.Info().Str("host", req.Host).Str("type", req.Type).Msg("Added new node via auto-registration")
	}

	// Log what we're about to save
	if req.Type == "pve" && len(h.getConfig(r.Context()).PVEInstances) > 0 {
		lastNode := h.getConfig(r.Context()).PVEInstances[len(h.getConfig(r.Context()).PVEInstances)-1]
		log.Info().
			Str("name", lastNode.Name).
			Str("host", lastNode.Host).
			Str("tokenName", lastNode.TokenName).
			Bool("hasTokenValue", lastNode.TokenValue != "").
			Msg("About to save PVE node")
	}

	// Save configuration
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("Configuration saved successfully")

	actualName := h.findInstanceNameByHost(r.Context(), req.Type, host)
	if actualName == "" {
		actualName = strings.TrimSpace(req.ServerName)
	}
	if actualName == "" {
		actualName = strings.TrimSpace(nodeConfig.Name)
	}
	if actualName == "" {
		actualName = host
	}
	h.markAutoRegistered(req.Type, actualName)

	// Reload monitor to pick up new configuration
	if h.reloadFunc != nil {
		log.Info().Msg("Reloading monitor after auto-registration")
		go func() {
			// Run reload in background to avoid blocking the response
			if err := h.reloadFunc(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor after auto-registration")
			} else {
				log.Info().Msg("Monitor reloaded successfully after auto-registration")
			}
		}()
	}

	// Trigger a discovery refresh to remove the node from discovered list
	if h.getMonitor(r.Context()) != nil && h.getMonitor(r.Context()).GetDiscoveryService() != nil {
		log.Info().Msg("Triggering discovery refresh after auto-registration")
		h.getMonitor(r.Context()).GetDiscoveryService().ForceRefresh()
	}

	// Broadcast auto-registration success via WebSocket
	if h.wsHub != nil {
		nodeInfo := map[string]interface{}{
			"type":      req.Type,
			"host":      req.Host,
			"name":      req.ServerName,
			"tokenId":   req.TokenID,
			"hasToken":  true,
			"verifySSL": false,
			"status":    "connected",
		}

		// Broadcast the auto-registration success
		h.wsHub.BroadcastMessage(websocket.Message{
			Type:      "node_auto_registered",
			Data:      nodeInfo,
			Timestamp: time.Now().Format(time.RFC3339),
		})

		// Also broadcast a discovery update to refresh the UI
		if h.getMonitor(r.Context()) != nil && h.getMonitor(r.Context()).GetDiscoveryService() != nil {
			result, _ := h.getMonitor(r.Context()).GetDiscoveryService().GetCachedResult()
			if result != nil {
				h.wsHub.BroadcastMessage(websocket.Message{
					Type: "discovery_update",
					Data: map[string]interface{}{
						"servers":   result.Servers,
						"errors":    result.Errors,
						"timestamp": time.Now().Unix(),
					},
					Timestamp: time.Now().Format(time.RFC3339),
				})
				log.Info().Msg("Broadcasted discovery update after auto-registration")
			}
		}

		log.Info().
			Str("host", req.Host).
			Str("name", req.ServerName).
			Str("type", "node_auto_registered").
			Msg("Broadcasted auto-registration success via WebSocket")
	} else {
		log.Warn().Msg("WebSocket hub is nil, cannot broadcast auto-registration")
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Node %s auto-registered successfully", req.Host),
		"nodeId":  req.Host,
	})
}

// handleSecureAutoRegister handles the new secure registration flow where Pulse generates the token
func (h *ConfigHandlers) handleSecureAutoRegister(w http.ResponseWriter, r *http.Request, req *AutoRegisterRequest, clientIP string) {
	log.Info().
		Str("type", req.Type).
		Str("host", req.Host).
		Str("username", req.Username).
		Msg("Processing secure auto-register request")

	// Generate a unique token name based on Pulse's IP/hostname
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = strings.ReplaceAll(clientIP, ".", "-")
	}
	timestamp := time.Now().Unix()
	tokenName := fmt.Sprintf("pulse-%s-%d", hostname, timestamp)

	// Generate a secure random token value
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate secure token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	tokenValue := fmt.Sprintf("%x-%x-%x-%x-%x",
		tokenBytes[0:4], tokenBytes[4:6], tokenBytes[6:8], tokenBytes[8:10], tokenBytes[10:16])

	host, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fingerprint := ""
	if fp, err := tlsutil.FetchFingerprint(host); err != nil {
		log.Warn().Err(err).Str("host", host).Msg("Failed to fetch TLS fingerprint for auto-register")
	} else {
		fingerprint = fp
	}
	verifySSL := true

	// Create the token on the remote server
	var fullTokenID string
	var createErr error

	if req.Type == "pve" {
		// For PVE, create token via API
		fullTokenID = fmt.Sprintf("pulse-monitor@pam!%s", tokenName)
		// Note: This would require implementing token creation in the proxmox package
		// For now, we'll return the token for the script to create
		// TODO: Implement PVE token creation via API
	} else if req.Type == "pbs" {
		// For PBS, create token via API using the new client methods
		log.Info().
			Str("host", host).
			Str("username", req.Username).
			Msg("Creating PBS token via API")

		pbsClient, err := pbs.NewClient(pbs.ClientConfig{
			Host:        host,
			User:        req.Username,
			Password:    req.Password,
			Fingerprint: fingerprint,
			VerifySSL:   verifySSL,
		})
		if err != nil {
			log.Error().Err(err).Str("host", host).Msg("Failed to create PBS client")
			http.Error(w, fmt.Sprintf("Failed to connect to PBS: %v", err), http.StatusBadRequest)
			return
		}

		// Use the turnkey method to create user + token
		tokenID, tokenSecret, err := pbsClient.SetupMonitoringAccess(context.Background(), tokenName)
		if err != nil {
			log.Error().Err(err).Str("host", host).Msg("Failed to create PBS monitoring access")
			http.Error(w, fmt.Sprintf("Failed to create token: %v", err), http.StatusInternalServerError)
			return
		}

		fullTokenID = tokenID
		tokenValue = tokenSecret
		log.Info().
			Str("host", host).
			Str("tokenID", fullTokenID).
			Msg("Successfully created PBS token via API")
	}

	if createErr != nil {
		log.Error().Err(createErr).Msg("Failed to create token on remote server")
		http.Error(w, "Failed to create token on remote server", http.StatusInternalServerError)
		return
	}

	// Determine server name
	serverName := req.ServerName
	if serverName == "" {
		// Extract from host
		serverName = host
		serverName = strings.TrimPrefix(serverName, "https://")
		serverName = strings.TrimPrefix(serverName, "http://")
		if idx := strings.Index(serverName, ":"); idx > 0 {
			serverName = serverName[:idx]
		}
	}

	// Add the node to configuration
	if req.Type == "pve" {
		pveNode := config.PVEInstance{
			Name:              serverName,
			Host:              host,
			TokenName:         fullTokenID,
			TokenValue:        tokenValue,
			Fingerprint:       fingerprint,
			VerifySSL:         verifySSL,
			MonitorVMs:        true,
			MonitorContainers: true,
			MonitorStorage:    true,
			MonitorBackups:    true,
		}
		h.getConfig(r.Context()).PVEInstances = append(h.getConfig(r.Context()).PVEInstances, pveNode)
	} else if req.Type == "pbs" {
		pbsNode := config.PBSInstance{
			Name:              serverName,
			Host:              host,
			TokenName:         fullTokenID,
			TokenValue:        tokenValue,
			Fingerprint:       fingerprint,
			VerifySSL:         verifySSL,
			MonitorBackups:    true,
			MonitorDatastores: true,
			MonitorSyncJobs:   true,
			MonitorVerifyJobs: true,
			MonitorPruneJobs:  true,
		}
		h.getConfig(r.Context()).PBSInstances = append(h.getConfig(r.Context()).PBSInstances, pbsNode)
	}

	// Save configuration
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	actualName := h.findInstanceNameByHost(r.Context(), req.Type, host)
	if actualName == "" {
		actualName = serverName
	}
	h.markAutoRegistered(req.Type, actualName)

	// Reload monitor
	if h.reloadFunc != nil {
		go func() {
			if err := h.reloadFunc(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor after auto-registration")
			}
		}()
	}

	// Send success response with token details for script to create
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"message":    fmt.Sprintf("Node %s registered successfully", req.Host),
		"nodeId":     serverName,
		"tokenId":    fullTokenID,
		"tokenValue": tokenValue,
		"action":     "create_token", // Tells the script to create this token
	})

}

// SSHKeyPair holds the sensors SSH public key for temperature monitoring.
type SSHKeyPair struct {
	SensorsPublicKey string
}

// getOrGenerateSSHKeys returns the SSH public key for temperature monitoring
// If keys don't exist, they are generated automatically
// SECURITY: Blocks key generation when running in containers unless dev mode override is enabled
func (h *ConfigHandlers) getOrGenerateSSHKeys() SSHKeyPair {
	// CRITICAL SECURITY CHECK: Never generate SSH keys in containers (unless dev mode)
	// Container compromise = SSH key compromise = root access to Proxmox
	devModeAllowSSH := os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true"
	isContainer := os.Getenv("PULSE_DOCKER") == "true" || system.InContainer()

	if isContainer && !devModeAllowSSH {
		log.Error().Msg("SECURITY BLOCK: SSH key generation disabled in containerized deployments")
		log.Error().Msg("Temperature monitoring via SSH is disabled in containerized deployments")
		log.Error().Msg("See: https://github.com/rcourtman/Pulse/blob/main/SECURITY.md#critical-security-notice-for-container-deployments")
		log.Error().Msg("To test SSH keys in dev/lab only: PULSE_DEV_ALLOW_CONTAINER_SSH=true (NEVER in production!)")
		return SSHKeyPair{}
	}

	if devModeAllowSSH && isContainer {
		log.Warn().Msg("⚠️  DEV MODE: SSH key generation ENABLED in container - FOR TESTING ONLY")
		log.Warn().Msg("⚠️  This grants root SSH access from container - NEVER use in production!")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Warn().Err(err).Msg("Could not determine home directory for SSH keys")
		return SSHKeyPair{}
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Generate/load sensors key (for temperature collection)
	sensorsPrivPath := filepath.Join(sshDir, "id_ed25519_sensors")
	sensorsPubPath := filepath.Join(sshDir, "id_ed25519_sensors.pub")
	sensorsKey := h.generateOrLoadSSHKey(sshDir, sensorsPrivPath, sensorsPubPath, "sensors")

	return SSHKeyPair{
		SensorsPublicKey: sensorsKey,
	}
}

// generateOrLoadSSHKey generates or loads a single SSH keypair
func (h *ConfigHandlers) generateOrLoadSSHKey(sshDir, privateKeyPath, publicKeyPath, keyType string) string {
	// Check if public key already exists
	if pubKeyBytes, err := os.ReadFile(publicKeyPath); err == nil {
		publicKey := strings.TrimSpace(string(pubKeyBytes))
		log.Info().Str("keyPath", publicKeyPath).Str("type", keyType).Msg("Using existing SSH public key")
		return publicKey
	}

	// Key doesn't exist - generate one
	log.Info().Str("sshDir", sshDir).Str("type", keyType).Msg("Generating new SSH keypair for temperature monitoring")

	// Create .ssh directory if it doesn't exist
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		log.Error().Err(err).Str("sshDir", sshDir).Msg("Failed to create .ssh directory")
		return ""
	}

	// Generate Ed25519 key pair (more secure and faster than RSA)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate Ed25519 key")
		return ""
	}

	// Save private key in OpenSSH format
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Error().Err(err).Str("path", privateKeyPath).Msg("Failed to create private key file")
		return ""
	}
	defer privateKeyFile.Close()

	// Marshal Ed25519 private key to OpenSSH format
	privKeyBytes, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal private key")
		return ""
	}
	if err := pem.Encode(privateKeyFile, privKeyBytes); err != nil {
		log.Error().Err(err).Msg("Failed to write private key")
		return ""
	}

	// Generate public key in OpenSSH format
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate public key")
		return ""
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	publicKeyString := strings.TrimSpace(string(publicKeyBytes))

	// Save public key
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0644); err != nil {
		log.Error().Err(err).Str("path", publicKeyPath).Msg("Failed to write public key")
		return ""
	}

	log.Info().
		Str("privateKey", privateKeyPath).
		Str("publicKey", publicKeyPath).
		Msg("Successfully generated SSH keypair")

	return publicKeyString
}

// AgentInstallCommandRequest represents a request for an agent install command
type AgentInstallCommandRequest struct {
	Type string `json:"type"` // "pve" or "pbs"
}

// AgentInstallCommandResponse contains the generated install command
type AgentInstallCommandResponse struct {
	Command string `json:"command"`
	Token   string `json:"token"`
}

// HandleAgentInstallCommand generates an API token and install command for agent-based Proxmox setup
func (h *ConfigHandlers) handleAgentInstallCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentInstallCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate type
	if req.Type != "pve" && req.Type != "pbs" {
		http.Error(w, "Type must be 'pve' or 'pbs'", http.StatusBadRequest)
		return
	}

	// Generate a new API token with host report and host manage scopes
	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate API token for agent install")
		http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
		return
	}

	tokenName := fmt.Sprintf("proxmox-agent-%s-%d", req.Type, time.Now().Unix())
	scopes := []string{
		config.ScopeHostReport,
		config.ScopeHostConfigRead,
		config.ScopeHostManage,
		config.ScopeAgentExec,
	}

	record, err := config.NewAPITokenRecord(rawToken, tokenName, scopes)
	if err != nil {
		log.Error().Err(err).Str("token_name", tokenName).Msg("Failed to construct API token record")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Persist the token
	config.Mu.Lock()
	h.getConfig(r.Context()).APITokens = append(h.getConfig(r.Context()).APITokens, *record)
	h.getConfig(r.Context()).SortAPITokens()

	if h.getPersistence(r.Context()) != nil {
		if err := h.getPersistence(r.Context()).SaveAPITokens(h.getConfig(r.Context()).APITokens); err != nil {
			// Rollback the in-memory addition
			h.getConfig(r.Context()).APITokens = h.getConfig(r.Context()).APITokens[:len(h.getConfig(r.Context()).APITokens)-1]
			config.Mu.Unlock()
			log.Error().Err(err).Msg("Failed to persist API tokens after creation")
			http.Error(w, "Failed to save token to disk: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	config.Mu.Unlock()

	// Derive Pulse URL from the request
	host := r.Host
	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		if (parsedHost == "127.0.0.1" || parsedHost == "localhost") && parsedPort == strconv.Itoa(h.getConfig(r.Context()).FrontendPort) {
			// Prefer a user-configured public URL when we're running on loopback
			if publicURL := strings.TrimSpace(h.getConfig(r.Context()).PublicURL); publicURL != "" {
				if parsedURL, err := url.Parse(publicURL); err == nil && parsedURL.Host != "" {
					host = parsedURL.Host
				}
			}
		}
	}

	// Detect protocol - check both TLS and proxy headers
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	pulseURL := fmt.Sprintf("%s://%s", scheme, host)

	// Generate the install command
	command := fmt.Sprintf(`curl -fsSL %s/install.sh | bash -s -- \
  --url %s \
  --token %s \
  --enable-proxmox`,
		pulseURL, pulseURL, rawToken)

	log.Info().
		Str("token_name", tokenName).
		Str("type", req.Type).
		Msg("Generated agent install command with API token")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentInstallCommandResponse{
		Command: command,
		Token:   rawToken,
	})
}
