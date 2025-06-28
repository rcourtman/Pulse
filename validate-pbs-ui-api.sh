#!/bin/bash

echo "=== PBS UI vs API Data Validation ==="
echo

# Function to format JSON nicely
pretty_json() {
    echo "$1" | jq '.' 2>/dev/null || echo "$1"
}

# 1. Get raw state data
echo "1. Fetching raw state PBS data..."
STATE_PBS=$(curl -s http://localhost:7655/api/state | jq '.pbs[0]')

if [ "$STATE_PBS" = "null" ]; then
    echo "❌ No PBS data in state endpoint"
    exit 1
fi

# Extract key metrics from state
echo "State data summary:"
echo "  - Instance name: $(echo "$STATE_PBS" | jq -r '.pbsInstanceName')"
echo "  - Status: $(echo "$STATE_PBS" | jq -r '.status')"
echo "  - Datastores: $(echo "$STATE_PBS" | jq '.datastores | length')"

# Count snapshots manually from state
TOTAL_SNAPSHOTS=$(echo "$STATE_PBS" | jq '[.datastores[].snapshots | length] | add')
echo "  - Total snapshots (manual count): $TOTAL_SNAPSHOTS"

# Check collision info in state
STATE_COLLISIONS=$(echo "$STATE_PBS" | jq '.vmidCollisions.totalCollisions // 0')
echo "  - Collisions in state: $STATE_COLLISIONS"

echo

# 2. Get PBS API data
echo "2. Fetching PBS API data..."
API_DATA=$(curl -s http://localhost:7655/api/pbs/backups)
API_INSTANCE=$(echo "$API_DATA" | jq '.data.instances[0]')

echo "API data summary:"
echo "  - Instance name: $(echo "$API_INSTANCE" | jq -r '.name')"
echo "  - Total backups: $(echo "$API_INSTANCE" | jq '.stats.totalBackups')"
echo "  - Total guests: $(echo "$API_INSTANCE" | jq '.stats.totalGuests')"
echo "  - Namespaces: $(echo "$API_INSTANCE" | jq '.namespaces | keys | join(", ")')"

# Count collisions in API
API_COLLISIONS=$(echo "$API_INSTANCE" | jq '[.collisions[].collisions | length] | add // 0')
echo "  - Collisions in API: $API_COLLISIONS"

echo

# 3. Compare namespace grouping
echo "3. Namespace grouping comparison:"
echo

echo "From manual state processing:"
echo "$STATE_PBS" | jq -r '
.datastores[].snapshots
| group_by(.namespace // "root")
| map({
    namespace: .[0].namespace // "root",
    count: length,
    vms: [.[] | select(.["backup-type"] == "vm") | .["backup-id"]] | unique | length,
    cts: [.[] | select(.["backup-type"] == "ct") | .["backup-id"]] | unique | length
})
| .[]
| "  \(.namespace): \(.count) backups, \(.vms) VMs, \(.cts) CTs"'

echo
echo "From PBS API:"
echo "$API_INSTANCE" | jq -r '
.namespaces
| to_entries
| map({
    namespace: .key,
    count: .value.totalBackups,
    vms: (.value.vms | length),
    cts: (.value.cts | length)
})
| .[]
| "  \(.namespace): \(.count) backups, \(.vms) VMs, \(.cts) CTs"'

echo

# 4. Test specific date
echo "4. Testing date query for June 27, 2025:"
DATE_DATA=$(curl -s http://localhost:7655/api/pbs/backups/date/2025-06-27)
echo "  - Total backups: $(echo "$DATE_DATA" | jq '.data.summary.totalBackups')"
echo "  - Unique guests: $(echo "$DATE_DATA" | jq '.data.summary.guests')"
echo "  - By namespace:"
echo "$DATE_DATA" | jq -r '.data.summary.byNamespace | to_entries[] | "    \(.key): \(.value.count) backups (\(.value.vms) VMs, \(.value.cts) CTs)"'

echo

# 5. Collision details
echo "5. Collision analysis:"
COLLISION_DATA=$(curl -s http://localhost:7655/api/pbs/collisions)
echo "  - Has collisions: $(echo "$COLLISION_DATA" | jq '.data.hasCollisions')"
echo "  - Total: $(echo "$COLLISION_DATA" | jq '.data.totalCollisions')"
echo "  - Critical: $(echo "$COLLISION_DATA" | jq '.data.criticalCollisions')"
echo "  - Warning: $(echo "$COLLISION_DATA" | jq '.data.warningCollisions')"

echo
echo "First 3 collisions:"
echo "$COLLISION_DATA" | jq -r '.data.byInstance[0].collisions.byDatastore.main.collisions[:3][] | "  - \(.vmid): \(.sources | length) sources"'

echo
echo "=== Validation Summary ==="
echo
echo "Check for discrepancies in:"
echo "1. Total snapshot/backup counts between state and API"
echo "2. Namespace grouping counts"
echo "3. Collision detection numbers"
echo "4. Guest counts per namespace"
echo
echo "The UI should display data from the PBS API endpoints, not raw state data."