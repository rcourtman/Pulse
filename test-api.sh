#!/bin/bash
for i in {1..10}; do
  timestamp=$(date +"%H:%M:%S")
  cpu=$(curl -s -H "X-API-Token: 0999c3bdf6d98647da81c00643ea5c4fe4560aaefed9519e" http://localhost:7655/api/state | jq -r '.nodes[] | select(.name=="delly") | .cpu')
  echo "$timestamp: $cpu"
  sleep 2
done