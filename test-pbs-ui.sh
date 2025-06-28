#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to test and report
test_endpoint() {
    local name=$1
    local endpoint=$2
    local jq_filter=$3
    local expected=$4
    
    printf "${BLUE}Testing:${NC} %s\n" "$name"
    result=$(curl -s http://localhost:7655$endpoint | jq -r "$jq_filter" 2>/dev/null)
    if [ $? -eq 0 ]; then
        printf "${GREEN}✅ Result:${NC} %s" "$result"
        if [ -n "$expected" ]; then
            if [ "$result" = "$expected" ]; then
                printf " ${GREEN}(matches expected)${NC}"
            else
                printf " ${RED}(expected: %s)${NC}" "$expected"
            fi
        fi
        echo
    else
        printf "${RED}❌ Failed to get data${NC}\n"
    fi
    echo
}

echo "=== PBS UI Data Validation ==="
echo "Run this after UI changes to ensure data consistency"
echo
date
echo

# Global stats
test_endpoint "Total Backups" "/api/pbs/backups" ".data.globalStats.totalBackups" "231"
test_endpoint "Total Guests" "/api/pbs/backups" ".data.globalStats.totalGuests" "21"
test_endpoint "Namespaces" "/api/pbs/backups" '.data.instances[0].namespaces | keys | join(", ")'

# Namespace details
echo -e "${YELLOW}Namespace Breakdown:${NC}"
curl -s http://localhost:7655/api/pbs/backups | jq -r '.data.instances[0].namespaces | to_entries[] | "  \(.key): \(.value.totalBackups) backups, \(.value.vms | length) VMs, \(.value.cts | length) CTs"'
echo

# Collisions
test_endpoint "Total Collisions" "/api/pbs/collisions" ".data.totalCollisions" "16"
test_endpoint "Collision Severity" "/api/pbs/collisions" ".data.byInstance[0].collisions.severity" "warning"

# Date-based queries
TODAY=$(date +%Y-%m-%d)
test_endpoint "Today's Backups" "/api/pbs/backups/date/$TODAY" ".data.summary.totalBackups"
test_endpoint "June 27 Backups" "/api/pbs/backups/date/2025-06-27" ".data.summary.totalBackups" "21"

# Specific guest
test_endpoint "Guest CT/106 Backups" "/api/pbs/backups/guest/ct/106" ".data.totalBackups" "11"

# Show collision details for CT/106
echo -e "${YELLOW}CT/106 Collision Info:${NC}"
curl -s http://localhost:7655/api/pbs/collisions | jq -r '.data.byInstance[0].collisions.byDatastore.main.collisions[] | select(.vmid == "ct/106") | "  Sources: \(.sources | length)\n  Latest owner: \(.latestOwner)\n  Date range: \(.oldestSnapshot[0:10]) to \(.newestSnapshot[0:10])"'

echo
echo "=== Summary ==="
echo "The UI should display:"
echo "1. Warning banner about 16 VMID collisions (warning severity)"
echo "2. Namespace summary showing root (187) and pimox (44) backups"
echo "3. Calendar showing 21 backups on June 27, 2025"
echo "4. Guest details showing collision warnings where applicable"