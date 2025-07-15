#!/bin/bash

# Pulse Permission Checker for Proxmox Backup Server
# This script helps diagnose permission issues for Pulse monitoring
# Run on your PBS server
#
# This script is provided "as is" without warranty of any kind.
# Use at your own risk. Always review changes before applying to production.

set -euo pipefail

# Check if running interactively
if [ -t 0 ]; then
    INTERACTIVE=true
else
    INTERACTIVE=false
fi

# Parse command line arguments
AUTO_FIX=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --fix|--auto-fix)
            AUTO_FIX=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--fix|--auto-fix]"
            echo "  --fix, --auto-fix  Automatically apply permission fixes without prompting"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Pulse Permission Troubleshooter for PBS ===${NC}"
echo "This script helps diagnose permission issues for Pulse monitoring"
echo "Version: 1.0"
echo ""
echo -e "${YELLOW}This diagnostic tool will:${NC}"
echo "  • Check your API token permissions"
echo "  • Identify what's missing for Pulse monitoring"
echo "  • Offer to fix issues (with your approval)"
echo ""
echo "By using this script, you acknowledge it's provided as-is without warranty."
echo ""

# Check if running on PBS
if ! command -v proxmox-backup-manager &> /dev/null; then
    echo -e "${RED}Error: This script must be run on a Proxmox Backup Server${NC}"
    exit 1
fi

# Check for jq (JSON processor)
if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW}Note: 'jq' not found. JSON parsing will be limited.${NC}"
    echo "Consider installing: apt-get install jq"
    echo ""
    JQ_AVAILABLE=false
else
    JQ_AVAILABLE=true
fi

# Find all tokens
echo -e "${YELLOW}Step 1: Finding API tokens...${NC}"
tokens=""

# Get all users first
all_users=$(proxmox-backup-manager user list 2>/dev/null | grep -E '^\\| [^|]+@[^|]+' | awk -F'|' '{print $2}' | tr -d ' ' | grep -v '^userid$' || true)

# Check each user for tokens
for user in $all_users; do
    user_tokens=$(proxmox-backup-manager user list-tokens "$user" 2>/dev/null | grep -E '^\\| [^|]+![^|]+' | awk -F'|' '{print $2}' | tr -d ' ' | grep -v '^tokenid$' || true)
    if [[ -n "$user_tokens" ]]; then
        for token in $user_tokens; do
            tokens="$tokens$token\n"
        done
    fi
done

tokens=$(echo -e "$tokens" | grep -v '^$' | sort -u)

if [[ -z "$tokens" ]]; then
    echo -e "${RED}No API tokens found!${NC}"
    echo "Please create an API token for Pulse first."
    echo ""
    echo "To create a token:"
    echo "  proxmox-backup-manager user create pulse@pbs"
    echo "  proxmox-backup-manager user create-token pulse@pbs pulse-token --privsep 0"
    exit 1
fi

echo "Found API tokens:"
echo "$tokens" | sed 's/^/  - /'
echo ""

# Check datastores
echo -e "${YELLOW}Step 2: Finding datastores...${NC}"
if [[ "$JQ_AVAILABLE" == "true" ]]; then
    datastores=$(proxmox-backup-manager datastore list --output-format json 2>/dev/null | jq -r '.[].store' | sort -u || true)
else
    # Fallback: Parse text output
    datastores=$(proxmox-backup-manager datastore list 2>/dev/null | grep -E '^\\| [^|]+' | awk -F'|' '{print $2}' | tr -d ' ' | grep -v '^store$' | grep -v '^name$' | grep -v '^===' | sort -u || true)
fi

if [[ -z "$datastores" ]]; then
    echo -e "${RED}No datastores found!${NC}"
    exit 1
fi

echo "Found datastores:"
echo "$datastores" | sed 's/^/  - /'
echo ""

# Check permissions
echo -e "${YELLOW}Step 3: Checking token permissions...${NC}"
issues_found=0
fixes_needed=()

for token in $tokens; do
    echo -e "\n${BLUE}Token: $token${NC}"
    
    # Get token permissions
    if [[ "$JQ_AVAILABLE" == "true" ]]; then
        perms=$(proxmox-backup-manager acl list --output-format json 2>/dev/null | jq -r --arg token "$token" '.[] | select(.ugid == $token) | "  Path: \(.path)\n  Role: \(.roleid)"' || true)
    else
        # Fallback: Parse text output for this token
        perms=""
        while IFS= read -r line; do
            # Match lines that start with | and contain the token in the first column
            if [[ "$line" =~ ^\|[[:space:]]*${token}[[:space:]]*\| ]]; then
                path=$(echo "$line" | awk -F'|' '{print $3}' | tr -d ' ')
                role=$(echo "$line" | awk -F'|' '{print $5}' | tr -d ' ')
                if [[ -n "$path" ]] && [[ -n "$role" ]] && [[ "$path" != "path" ]]; then
                    perms="${perms}  Path: $path\n  Role: $role\n"
                fi
            fi
        done < <(proxmox-backup-manager acl list 2>/dev/null || true)
        perms=$(echo -e "$perms" | grep -v '^$' || true)
    fi
    
    if [[ -z "$perms" ]]; then
        echo -e "  ${RED}✗${NC} No permissions found for this token"
        issues_found=$((issues_found + 1))
        
        # Add fixes for each datastore
        for ds in $datastores; do
            fixes_needed+=("proxmox-backup-manager acl update /datastore/$ds --auth-id '$token' --role DatastoreAudit")
        done
        
        # Also suggest root permission for general access
        fixes_needed+=("proxmox-backup-manager acl update / --auth-id '$token' --role DatastoreAudit")
    else
        echo "$perms"
        
        # Check if token has access to each datastore
        for ds in $datastores; do
            has_access=$(echo "$perms" | grep -E "(Path: /datastore/$ds|Path: /datastore$|Path: /$)" || true)
            if [[ -z "$has_access" ]]; then
                echo -e "  ${RED}✗${NC} No access to datastore: $ds"
                issues_found=$((issues_found + 1))
                fixes_needed+=("proxmox-backup-manager acl update /datastore/$ds --auth-id '$token' --role DatastoreAudit")
            else
                echo -e "  ${GREEN}✓${NC} Has access to datastore: $ds"
            fi
        done
    fi
done

echo ""
echo -e "${YELLOW}Step 4: Summary and Recommendations${NC}"
echo "================================================"

if [[ $issues_found -eq 0 ]]; then
    echo -e "${GREEN}✓ No permission issues found!${NC}"
    echo ""
    echo "Your PBS API tokens appear to have the correct permissions."
    echo "If you're still having issues, verify:"
    echo "  1. The token credentials are correctly configured in Pulse"
    echo "  2. The PBS server is reachable from Pulse"
    echo "  3. Check Pulse logs for connection errors"
else
    echo -e "${RED}✗ Found $issues_found permission issue(s)${NC}"
    echo ""
    echo -e "${YELLOW}Important:${NC} PBS tokens always require explicit permissions."
    echo "Unlike PVE, PBS tokens never inherit permissions from users, regardless of privsep setting."
    echo ""
    echo "The following commands will fix the permission issues:"
    echo ""
    
    # Remove duplicates and print fixes
    printf '%s\n' "${fixes_needed[@]}" | sort -u | while IFS= read -r fix; do
        echo "  $fix"
    done
    
    echo ""
    echo -e "${YELLOW}Note:${NC} DatastoreAudit role provides read-only access which is all Pulse needs."
    echo ""
    
    # Offer to apply fixes
    APPLY_FIXES=false
    
    if [[ "$AUTO_FIX" == "true" ]]; then
        echo -e "${YELLOW}Auto-fix mode enabled. Applying fixes...${NC}"
        APPLY_FIXES=true
    elif [[ "$INTERACTIVE" == "true" ]]; then
        echo -e "${YELLOW}Would you like to apply these fixes automatically?${NC}"
        echo "This will modify ACL permissions on your PBS server."
        echo ""
        echo -e "${RED}WARNING:${NC} Permission changes take effect immediately!"
        echo "Ensure you understand the implications before proceeding."
        echo ""
        read -p "Apply fixes? (y/N): " -n 1 -r
        echo ""
        
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            APPLY_FIXES=true
        fi
    else
        echo -e "${YELLOW}Run with --fix flag to automatically apply these fixes${NC}"
    fi
    
    if [[ "$APPLY_FIXES" == "true" ]]; then
        echo -e "\n${BLUE}Applying permission fixes...${NC}"
        
        # Apply each fix
        printf '%s\n' "${fixes_needed[@]}" | sort -u | while IFS= read -r fix; do
            echo -e "\nExecuting: ${YELLOW}$fix${NC}"
            # Use bash -c instead of eval for better safety
            if bash -c "$fix"; then
                echo -e "${GREEN}✓ Success${NC}"
            else
                echo -e "${RED}✗ Failed to apply fix${NC}"
            fi
        done
        
        echo -e "\n${GREEN}Permission fixes have been applied!${NC}"
        echo "Please restart Pulse to use the updated permissions."
    else
        echo -e "\n${BLUE}Skipping automatic fixes.${NC}"
        echo "You can run the commands above manually when ready."
    fi
fi

echo ""
echo "For more information, see: https://github.com/rcourtman/Pulse/blob/main/README.md"