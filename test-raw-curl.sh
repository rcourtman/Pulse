#!/bin/bash

# Simple curl test to check Proxmox update frequency
TOKEN="pulse-monitor@pam!test-token=a0c05119-0e04-4918-ac94-1fa604259bf1"
URL="https://192.168.0.5:8006/api2/json/nodes"

echo "Testing Proxmox API update frequency with raw curl"
echo "Polling every 1 second for 30 seconds"
echo "================================================"

last_cpu=""
last_mem=""
count=0

for i in {1..30}; do
    # Get current time
    timestamp=$(date +"%H:%M:%S")
    
    # Make API call
    response=$(curl -sk -H "Authorization: PVEAPIToken=$TOKEN" "$URL")
    
    # Extract CPU and memory for delly node
    cpu=$(echo "$response" | jq -r '.data[] | select(.node=="delly") | .cpu')
    mem=$(echo "$response" | jq -r '.data[] | select(.node=="delly") | .mem')
    
    # Check if values changed
    if [ "$i" -eq 1 ]; then
        echo "$timestamp - Initial: CPU=$cpu, Mem=$((mem / 1024 / 1024 / 1024)) GB"
    else
        if [ "$cpu" != "$last_cpu" ]; then
            echo "$timestamp - CPU CHANGED: $last_cpu -> $cpu"
            ((count++))
        fi
        if [ "$mem" != "$last_mem" ]; then
            mem_diff=$(( (mem - last_mem) / 1024 / 1024 ))
            if [ "$mem_diff" -ne 0 ]; then
                echo "$timestamp - MEM CHANGED: ${mem_diff:+}${mem_diff} MB"
            fi
        fi
    fi
    
    last_cpu=$cpu
    last_mem=$mem
    
    sleep 1
done

echo ""
echo "Total CPU changes detected: $count in 30 seconds"