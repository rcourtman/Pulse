#!/bin/bash

TOKEN="pulse-monitor@pam!test-token=a0c05119-0e04-4918-ac94-1fa604259bf1"
URL="https://192.168.0.5:8006/api2/json/nodes"

echo "Testing for 60 seconds..."
last_cpu=""
changes=()

for i in {1..60}; do
    response=$(curl -sk -H "Authorization: PVEAPIToken=$TOKEN" "$URL")
    cpu=$(echo "$response" | jq -r '.data[] | select(.node=="delly") | .cpu')
    
    if [ "$i" -eq 1 ]; then
        echo "Second $i: CPU=$cpu (initial)"
    elif [ "$cpu" != "$last_cpu" ]; then
        echo "Second $i: CPU changed"
        changes+=($i)
    fi
    
    last_cpu=$cpu
    sleep 1
done

echo ""
echo "Changes at seconds: ${changes[@]}"
echo "Total changes: ${#changes[@]} in 60 seconds"
if [ ${#changes[@]} -gt 0 ]; then
    echo "Average interval: $((60 / ${#changes[@]})) seconds"
fi