#!/bin/bash

TOKEN="pulse-monitor@pam!test-token=a0c05119-0e04-4918-ac94-1fa604259bf1"
AUTH="Authorization: PVEAPIToken=$TOKEN"
BASE="https://192.168.0.5:8006/api2/json"

echo "Testing different Proxmox endpoints for CPU data..."
echo "================================================"

# Test 1: /nodes endpoint
echo -e "\n1. Testing /nodes endpoint (10 samples):"
last=""
for i in {1..10}; do
    cpu=$(curl -sk -H "$AUTH" "$BASE/nodes" | jq -r '.data[] | select(.node=="delly") | .cpu')
    if [ "$cpu" != "$last" ]; then
        echo "  Sample $i: CPU=$cpu (changed)"
    else
        echo "  Sample $i: CPU=$cpu"
    fi
    last=$cpu
    sleep 1
done

# Test 2: /nodes/delly/status endpoint  
echo -e "\n2. Testing /nodes/delly/status endpoint (10 samples):"
last=""
for i in {1..10}; do
    cpu=$(curl -sk -H "$AUTH" "$BASE/nodes/delly/status" | jq -r '.data.cpu // 0')
    if [ "$cpu" != "$last" ]; then
        echo "  Sample $i: CPU=$cpu (changed)"
    else
        echo "  Sample $i: CPU=$cpu"
    fi
    last=$cpu
    sleep 1
done

# Test 3: /cluster/resources endpoint
echo -e "\n3. Testing /cluster/resources endpoint (10 samples):"
last=""
for i in {1..10}; do
    cpu=$(curl -sk -H "$AUTH" "$BASE/cluster/resources?type=node" | jq -r '.data[] | select(.node=="delly") | .cpu // 0')
    if [ "$cpu" != "$last" ]; then
        echo "  Sample $i: CPU=$cpu (changed)"
    else
        echo "  Sample $i: CPU=$cpu"
    fi
    last=$cpu
    sleep 1
done