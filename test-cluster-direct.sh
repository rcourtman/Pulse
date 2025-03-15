#!/bin/bash

echo "Running cluster resources test..."
export USE_MOCK_DATA=true
export MOCK_DATA_ENABLED=true
export MOCK_CLUSTER_ENABLED=true
export PROXMOX_USE_CLUSTER_RESOURCES=true
export LOG_LEVEL=info

node --require ts-node/register src/services/test-cluster.ts
