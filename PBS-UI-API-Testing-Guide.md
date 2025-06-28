# PBS UI/API Testing & Feedback Guide

## Overview
This guide helps validate that the PBS UI displays data correctly by cross-referencing with the API endpoints.

## Quick Test Commands

### 1. Check What's in the Summary Card
The namespace-grouped summary should match this API call:
```bash
# Get namespace summary
curl -s http://localhost:7655/api/pbs/backups | jq '.data.instances[0].namespaces'
```

Expected in UI:
- Root namespace: 187 backups, 3 VMs, 17 CTs
- Pimox namespace: 44 backups, 0 VMs, 4 CTs

### 2. Check Collision Warnings
```bash
# Get collision info
curl -s http://localhost:7655/api/pbs/collisions | jq '.data | {hasCollisions, totalCollisions, severity: .byInstance[0].collisions.severity}'
```

Expected in UI:
- Warning banner showing 16 VMID collisions
- Severity: "warning" (not critical because namespaces are used)

### 3. Check Calendar Day View (June 27, 2025)
```bash
# Get backups for specific date
curl -s http://localhost:7655/api/pbs/backups/date/2025-06-27 | jq '.data.summary'
```

Expected when clicking June 27:
- 21 total backups
- 19 unique guests
- Hourly distribution showing peaks at 4h, 13h, and 23h

### 4. Check Individual Guest Backups
```bash
# Pick a guest with collisions (e.g., CT 106)
curl -s http://localhost:7655/api/pbs/backups/guest/ct/106 | jq '.data | {guestId, totalBackups, instances: .instances | length}'
```

Expected in guest detail view:
- 11 total backups for CT 106
- Shows collision warning (3 different sources)

## Automated Validation Script

```bash
#!/bin/bash
# validate-pbs-display.sh

echo "=== PBS Display Validation Checklist ==="
echo

# Test 1: Summary totals
API_TOTALS=$(curl -s http://localhost:7655/api/pbs/backups | jq '.data.globalStats')
echo "1. Global Summary Should Show:"
echo "$API_TOTALS" | jq -r '"   Total Backups: \(.totalBackups)\n   Total Size: \(.totalSize | . / 1e12 | tostring[0:4]) TB\n   Unique Guests: \(.totalGuests)\n   Namespaces: \(.namespaces | join(", "))"'

# Test 2: Namespace breakdown
echo -e "\n2. Namespace Cards Should Show:"
curl -s http://localhost:7655/api/pbs/backups | jq -r '.data.instances[0].namespaces | to_entries[] | "   \(.key): \(.value.totalBackups) backups, \(.value.vms | length) VMs, \(.value.cts | length) CTs"'

# Test 3: Collision warnings
COLLISIONS=$(curl -s http://localhost:7655/api/pbs/collisions | jq '.data')
echo -e "\n3. Collision Warning Banner:"
if [ "$(echo "$COLLISIONS" | jq '.hasCollisions')" = "true" ]; then
    echo "$COLLISIONS" | jq -r '"   ⚠️  \(.totalCollisions) VMID collisions detected\n   Severity: \(.byInstance[0].collisions.severity)\n   First affected: \(.byInstance[0].collisions.byDatastore.main.collisions[0].vmid)"'
else
    echo "   ✅ No collisions - no warning should be shown"
fi

# Test 4: Today's backups
TODAY=$(date +%Y-%m-%d)
TODAY_BACKUPS=$(curl -s http://localhost:7655/api/pbs/backups/date/$TODAY | jq '.data.summary')
echo -e "\n4. Today's Backups ($TODAY):"
echo "$TODAY_BACKUPS" | jq -r '"   Total: \(.totalBackups)\n   Guests: \(.guests)\n   Size: \(.totalSize | . / 1e9 | tostring[0:5]) GB"'

echo -e "\n=== Visual Elements to Verify ==="
echo "□ Collision warning banner (red/yellow based on severity)"
echo "□ Namespace summary cards with guest counts"
echo "□ Backup timeline in calendar view"
echo "□ Guest list showing backup counts"
echo "□ Hourly distribution chart for selected date"
```

## Manual Testing Checklist

### PBS Tab - Main View
- [ ] Collision warning banner appears at top (if collisions exist)
- [ ] "Backup Summary by Namespace" section shows correct counts
- [ ] Each namespace card displays VM/CT breakdown
- [ ] Date ranges shown for each namespace

### Calendar View
- [ ] Click on June 27, 2025
- [ ] Should show 21 backups
- [ ] Timeline shows concentration at 22:00-23:00
- [ ] Can drill down to see individual backups

### Namespace Filtering
- [ ] Click namespace tabs
- [ ] Counts update correctly
- [ ] Tasks filter by namespace

### Guest Detail View
- [ ] Click on a guest with collisions (e.g., CT 106)
- [ ] Shows warning about multiple sources
- [ ] Lists all backup instances

## Continuous Validation

Run this every time you make UI changes:
```bash
# Save as /opt/pulse/test-pbs-ui.sh
#!/bin/bash

# Function to test and report
test_endpoint() {
    local name=$1
    local endpoint=$2
    local jq_filter=$3
    
    echo "Testing: $name"
    result=$(curl -s http://localhost:7655$endpoint | jq "$jq_filter" 2>/dev/null)
    if [ $? -eq 0 ]; then
        echo "✅ $result"
    else
        echo "❌ Failed to get data"
    fi
    echo
}

echo "=== PBS UI Data Validation ==="
echo "Run this after UI changes to ensure data consistency"
echo

test_endpoint "Total Backups" "/api/pbs/backups" ".data.globalStats.totalBackups"
test_endpoint "Namespaces" "/api/pbs/backups" ".data.instances[0].namespaces | keys"
test_endpoint "Collisions" "/api/pbs/collisions" ".data.totalCollisions"
test_endpoint "Today's Backups" "/api/pbs/backups/date/$(date +%Y-%m-%d)" ".data.summary.totalBackups"
test_endpoint "Guest CT/106" "/api/pbs/backups/guest/ct/106" ".data.totalBackups"
```

## Troubleshooting Discrepancies

If UI doesn't match API:

1. **Check Console Errors**
   ```bash
   # In browser DevTools
   console.log(PulseApp.state.pbs)  # Raw state
   ```

2. **Verify API is Used**
   - UI should call `/api/pbs/backups` not use raw state
   - Check Network tab for API calls

3. **Clear Cache**
   - Hard refresh: Ctrl+Shift+R
   - Clear localStorage if needed

4. **Check Data Flow**
   ```
   PBS Server → dataFetcher.js → state.js → API endpoints → UI
   ```

## Report Issues

When reporting discrepancies:
1. Screenshot of UI element
2. API response for same data
3. Expected vs Actual values
4. Browser console errors