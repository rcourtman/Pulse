#!/bin/bash

# Pulse Permission Checker for Proxmox VE
# This script helps diagnose permission issues for Pulse monitoring
# Run on any Proxmox VE node in your cluster

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

echo -e "${BLUE}=== Pulse Permission Troubleshooter for Proxmox VE ===${NC}"
echo "This script helps diagnose storage permission issues for Pulse monitoring"
echo "Version: 1.0"
echo ""

# Check if running on PVE
if ! command -v pveum &> /dev/null; then
    echo -e "${RED}Error: This script must be run on a Proxmox VE node${NC}"
    exit 1
fi

# Function to check if a user has specific permissions on a path
check_permission() {
    local user=$1
    local path=$2
    local perm_pattern=$3
    local perms=$(pveum user permissions $user --path $path 2>/dev/null | grep -E "$perm_pattern" || true)
    if [[ -n "$perms" ]]; then
        echo "true"
    else
        echo "false"
    fi
}

# Function to check if user has PVEAuditor permissions
check_auditor_perms() {
    local user=$1
    local path=$2
    # PVEAuditor provides: VM.Audit, Sys.Audit, Datastore.Audit, SDN.Audit, etc.
    local has_vm_audit=$(pveum user permissions $user --path $path 2>/dev/null | grep -E "VM\.Audit" || true)
    local has_sys_audit=$(pveum user permissions $user --path $path 2>/dev/null | grep -E "Sys\.Audit" || true)
    if [[ -n "$has_vm_audit" ]] && [[ -n "$has_sys_audit" ]]; then
        echo "true"
    else
        echo "false"
    fi
}

# Find all users with API tokens
echo -e "${YELLOW}Step 1: Finding users with API tokens...${NC}"
# First get all users, then check each for tokens
all_users=$(pveum user list --output-format json 2>/dev/null | jq -r '.[].userid' | sort -u)
users_with_tokens=""

for user in $all_users; do
    # Try to list tokens for this user - will fail if no tokens exist
    if pveum user token list "$user" --output-format json &>/dev/null; then
        token_count=$(pveum user token list "$user" --output-format json 2>/dev/null | jq 'length' 2>/dev/null || echo "0")
        if [[ "$token_count" -gt 0 ]]; then
            users_with_tokens="$users_with_tokens$user\n"
        fi
    fi
done

users_with_tokens=$(echo -e "$users_with_tokens" | grep -v '^$' | sort -u)

if [[ -z "$users_with_tokens" ]]; then
    echo -e "${RED}No users with API tokens found!${NC}"
    echo "Please create an API token for Pulse first."
    exit 1
fi

echo "Found users with tokens:"
echo "$users_with_tokens" | sed 's/^/  - /'
echo ""

# Check each user's tokens
echo -e "${YELLOW}Step 2: Checking API tokens and their privilege separation...${NC}"
declare -A token_info

for user in $users_with_tokens; do
    echo -e "\n${BLUE}User: $user${NC}"
    
    # Get token details
    tokens=$(pveum user token list $user --output-format json 2>/dev/null || echo '[]')
    
    if [[ "$tokens" == "[]" ]]; then
        echo "  No tokens found (may lack permission to view)"
        continue
    fi
    
    echo "$tokens" | jq -r '.[] | "  Token: \(.tokenid // "unknown")\n    Privilege Separation: \(if .privsep == 1 then "Yes (permissions on USER required)" else "No (permissions on TOKEN required)" end)\n    Expire: \(.expire // "never")\n    Comment: \(.comment // "none")"'
    
    # Store token info for later
    while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            tokenid=$(echo "$line" | jq -r '.tokenid // empty')
            privsep=$(echo "$line" | jq -r '.privsep // 1')
            if [[ -n "$tokenid" ]]; then
                token_info["$user!$tokenid"]=$privsep
            fi
        fi
    done < <(echo "$tokens" | jq -c '.[]' 2>/dev/null || true)
done

echo ""

# Find backup-enabled storages
echo -e "${YELLOW}Step 3: Finding backup-enabled storages...${NC}"
backup_storages=$(pvesh get /storage --output-format json 2>/dev/null | jq -r '.[] | select(.content and (.content | contains("backup")) and .type != "pbs") | .storage' | sort -u)

if [[ -z "$backup_storages" ]]; then
    echo -e "${YELLOW}No local backup storages found (excluding PBS).${NC}"
    echo "If you're using PBS exclusively, this is normal and the warning can be ignored."
    exit 0
fi

echo "Found backup-enabled storages (excluding PBS):"
echo "$backup_storages" | sed 's/^/  - /'
echo ""

# Check permissions for each user/token on each storage
echo -e "${YELLOW}Step 4: Checking storage permissions...${NC}"
issues_found=0
fixes_needed=()

for user in $users_with_tokens; do
    echo -e "\n${BLUE}Checking permissions for user: $user${NC}"
    
    # CRITICAL: Check if user has PVEAuditor permissions on root (/)
    user_has_root_perm=$(check_auditor_perms "$user" "/")
    
    if [[ "$user_has_root_perm" == "true" ]]; then
        echo -e "  ${GREEN}✓${NC} User has PVEAuditor permissions on / (can monitor VMs/containers)"
    else
        echo -e "  ${RED}✗${NC} User lacks PVEAuditor permissions on / - CRITICAL: Cannot monitor VMs/containers!"
        issues_found=$((issues_found + 1))
        
        # Get tokens for this user to generate fix commands
        user_tokens=$(pveum user token list $user --output-format json 2>/dev/null | jq -r '.[].tokenid' 2>/dev/null || true)
        
        for token in $user_tokens; do
            if [[ -n "$token" ]]; then
                privsep=${token_info["$user!$token"]:-1}
                
                if [[ "$privsep" == "1" ]]; then
                    fixes_needed+=("pveum acl modify / --users $user --roles PVEAuditor")
                else
                    fixes_needed+=("pveum acl modify / --tokens $user!$token --roles PVEAuditor")
                fi
            fi
        done
    fi
    
    # Check user permissions on /storage
    user_has_storage_perm=$(check_permission "$user" "/storage" "Datastore\.(Audit|Allocate|AllocateSpace)")
    
    if [[ "$user_has_storage_perm" == "true" ]]; then
        echo -e "  ${GREEN}✓${NC} User has Datastore permissions on /storage"
    else
        echo -e "  ${RED}✗${NC} User lacks Datastore permissions on /storage"
    fi
    
    # Check permissions on each storage
    for storage in $backup_storages; do
        echo -e "\n  Storage: ${BLUE}$storage${NC}"
        
        # Check if user has permissions on this specific storage
        user_has_perm=$(check_permission "$user" "/storage/$storage" "Datastore\.(Audit|Allocate|AllocateSpace)")
        
        # Test actual access by trying to list content
        can_list="false"
        error_msg=""
        
        # Get first node to test on
        node=$(pvesh get /nodes --output-format json 2>/dev/null | jq -r '.[0].node' 2>/dev/null || hostname)
        
        # Try to list backup content as the user would
        test_output=$(pvesh get /nodes/$node/storage/$storage/content --content backup 2>&1 || true)
        
        if echo "$test_output" | grep -q "Permission check failed"; then
            can_list="false"
            error_msg="Permission denied"
        elif echo "$test_output" | grep -q "volid"; then
            can_list="true"
        fi
        
        if [[ "$user_has_perm" == "true" ]] || [[ "$can_list" == "true" ]]; then
            echo -e "    ${GREEN}✓${NC} User has access to storage"
        else
            echo -e "    ${RED}✗${NC} User cannot access storage"
            issues_found=$((issues_found + 1))
            
            # Check tokens for this user
            user_tokens=$(pveum user token list $user --output-format json 2>/dev/null | jq -r '.[].tokenid' 2>/dev/null || true)
            
            for token in $user_tokens; do
                if [[ -n "$token" ]]; then
                    privsep=${token_info["$user!$token"]:-1}
                    
                    if [[ "$privsep" == "1" ]]; then
                        fixes_needed+=("pveum acl modify /storage/$storage --users $user --roles PVEDatastoreAdmin")
                    else
                        fixes_needed+=("pveum acl modify /storage/$storage --tokens $user!$token --roles PVEDatastoreAdmin")
                    fi
                fi
            done
        fi
    done
done

echo ""
echo -e "${YELLOW}Step 5: Summary and Recommendations${NC}"
echo "================================================"

if [[ $issues_found -eq 0 ]]; then
    echo -e "${GREEN}✓ No permission issues found!${NC}"
    echo ""
    echo "Your Proxmox API tokens appear to have the correct permissions for accessing backup storage."
    echo "If you're still seeing warnings in Pulse, try:"
    echo "  1. Restart Pulse to refresh the data"
    echo "  2. Check if backups actually exist in the listed storages"
    echo "  3. Verify the token credentials in Pulse configuration"
else
    echo -e "${RED}✗ Found $issues_found permission issue(s)${NC}"
    echo ""
    echo "The following commands will fix the permission issues:"
    echo ""
    
    # Remove duplicates and print fixes
    printf '%s\n' "${fixes_needed[@]}" | sort -u | while IFS= read -r fix; do
        echo "  $fix"
    done
    
    echo ""
    echo "Run these commands on any node in your cluster to grant the necessary permissions."
    echo ""
    echo -e "${YELLOW}Important Notes:${NC}"
    echo "- PVEAuditor on / is REQUIRED for basic monitoring (VMs, containers, nodes)"
    echo "- PVEDatastoreAdmin on /storage is needed to view backup information"
    echo "- With privsep=1 (Yes), set permissions on the USER"
    echo "- With privsep=0 (No), set permissions on the TOKEN"
    echo ""
    
    # Offer to apply fixes
    APPLY_FIXES=false
    
    if [[ "$AUTO_FIX" == "true" ]]; then
        echo -e "${YELLOW}Auto-fix mode enabled. Applying fixes...${NC}"
        APPLY_FIXES=true
    elif [[ "$INTERACTIVE" == "true" ]]; then
        echo -e "${YELLOW}Would you like to apply these fixes automatically?${NC}"
        echo "This will modify permissions on your Proxmox cluster."
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
            if eval "$fix"; then
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