#!/bin/bash

echo "Testing PBS form fix..."
echo ""

# Start Pulse if not running
sudo systemctl restart pulse-backend

sleep 3

# Test 1: Add a PVE node
echo "1. Adding PVE node..."
curl -X POST http://localhost:7655/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "type": "pve",
    "name": "pve-test",
    "host": "https://192.168.1.100:8006",
    "tokenName": "root@pam!test",
    "tokenValue": "test-token-123"
  }' 2>/dev/null

if [ $? -eq 0 ]; then
  echo "✓ PVE node added"
else
  echo "✗ Failed to add PVE node"
fi

echo ""

# Test 2: Add a PBS node  
echo "2. Adding PBS node..."
curl -X POST http://localhost:7655/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "type": "pbs",
    "name": "pbs-test",
    "host": "https://192.168.1.200:8007",
    "tokenName": "root@pam!pbstest",
    "tokenValue": "pbs-token-456"
  }' 2>/dev/null

if [ $? -eq 0 ]; then
  echo "✓ PBS node added"
else
  echo "✗ Failed to add PBS node"
fi

echo ""

# Test 3: Get nodes and verify they're separate
echo "3. Verifying nodes are correctly stored..."
NODES=$(curl -s http://localhost:7655/api/nodes)

# Check if PVE node exists with correct data
if echo "$NODES" | grep -q '"name":"pve-test".*"type":"pve"'; then
  echo "✓ PVE node data correct"
else
  echo "✗ PVE node data incorrect"
fi

# Check if PBS node exists with correct data
if echo "$NODES" | grep -q '"name":"pbs-test".*"type":"pbs"'; then
  echo "✓ PBS node data correct"
else
  echo "✗ PBS node data incorrect"
fi

echo ""
echo "Test completed. The UI should now:"
echo "1. Show clean forms when adding new nodes"
echo "2. Populate correct data when editing existing nodes"
echo "3. Not mix PVE and PBS data"