#!/bin/bash

# PBS API credentials from .env
PBS_HOST="192.168.0.16"
PBS_PORT="8007"
PBS_TOKEN_ID="root@pam!pulse-monitoring"
PBS_TOKEN_SECRET="9cc0b813-17e7-49e1-8d2d-d4e309c2db94"
DATASTORE="main"

# Function to extract node name from a backup
extract_node_from_backup() {
    local backup_type=$1
    local backup_id=$2
    local backup_time=$3
    
    echo "Checking $backup_type/$backup_id at timestamp $backup_time..."
    
    # Download the client.log.blob
    response=$(curl -sk -H "Authorization: PBSAPIToken=$PBS_TOKEN_ID:$PBS_TOKEN_SECRET" \
        "https://$PBS_HOST:$PBS_PORT/api2/json/admin/datastore/$DATASTORE/download-decoded?backup-type=$backup_type&backup-id=$backup_id&backup-time=$backup_time&file-name=client.log.blob")
    
    # Extract the client name (node name)
    node_name=$(echo "$response" | grep "Client name:" | sed 's/.*Client name: //' | head -1)
    
    if [ -n "$node_name" ]; then
        echo "  → Node: $node_name"
        return 0
    else
        echo "  → Could not extract node name"
        return 1
    fi
}

echo "=== PBS Node Information Extraction Test ==="
echo "PBS Server: $PBS_HOST:$PBS_PORT"
echo "Datastore: $DATASTORE"
echo ""

# Get list of backup groups
echo "Fetching backup groups..."
groups=$(curl -sk -H "Authorization: PBSAPIToken=$PBS_TOKEN_ID:$PBS_TOKEN_SECRET" \
    "https://$PBS_HOST:$PBS_PORT/api2/json/admin/datastore/$DATASTORE/groups")

# Process first 5 CT backups
echo ""
echo "=== Container (CT) Backups ==="
ct_backups=$(echo "$groups" | jq -r '.data[] | select(.["backup-type"] == "ct") | .["backup-id"]' | head -5)

for backup_id in $ct_backups; do
    # Get latest backup timestamp
    timestamp=$(curl -sk -H "Authorization: PBSAPIToken=$PBS_TOKEN_ID:$PBS_TOKEN_SECRET" \
        "https://$PBS_HOST:$PBS_PORT/api2/json/admin/datastore/$DATASTORE/snapshots?backup-type=ct&backup-id=$backup_id" | \
        jq -r '.data[0]["backup-time"]')
    
    if [ -n "$timestamp" ] && [ "$timestamp" != "null" ]; then
        extract_node_from_backup "ct" "$backup_id" "$timestamp"
    fi
done

# Process first 5 VM backups
echo ""
echo "=== Virtual Machine (VM) Backups ==="
vm_backups=$(echo "$groups" | jq -r '.data[] | select(.["backup-type"] == "vm") | .["backup-id"]' | head -5)

for backup_id in $vm_backups; do
    # Get latest backup timestamp
    timestamp=$(curl -sk -H "Authorization: PBSAPIToken=$PBS_TOKEN_ID:$PBS_TOKEN_SECRET" \
        "https://$PBS_HOST:$PBS_PORT/api2/json/admin/datastore/$DATASTORE/snapshots?backup-type=vm&backup-id=$backup_id" | \
        jq -r '.data[0]["backup-time"]')
    
    if [ -n "$timestamp" ] && [ "$timestamp" != "null" ]; then
        extract_node_from_backup "vm" "$backup_id" "$timestamp"
    fi
done

echo ""
echo "=== Summary ==="
echo "The 'Client name:' field in client.log.blob contains the source node name."
echo "This can be used to distinguish backups from different nodes with the same VMID."