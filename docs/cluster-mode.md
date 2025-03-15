# Proxmox Cluster Mode in Pulse

## Overview

Pulse now supports a more efficient method for monitoring Proxmox VE clusters by using the `/cluster/resources` API endpoint. This feature allows you to configure just one node from your cluster and automatically get information about all VMs and containers across all nodes.

## Configuration

To use the cluster resources endpoint, add the following to your `.env` file:

```
# Enable use of cluster resources endpoint
PROXMOX_USE_CLUSTER_RESOURCES=true

# When using cluster resources, cluster mode should also be enabled
PROXMOX_CLUSTER_MODE=true

# Optionally, you can specify a custom cluster name
PROXMOX_CLUSTER_NAME=my-cluster
```

## How It Works

When `PROXMOX_USE_CLUSTER_RESOURCES` is enabled, Pulse will:

1. Fetch all VMs and containers from all nodes in the cluster with a single API call
2. Properly organize and display them according to their actual host node
3. Eliminate duplicates that might arise from querying each node individually
4. Update metrics and statuses more efficiently

## Requirements

To use this feature:

1. Configure **only one node** from your Proxmox cluster in your `.env` file
2. The configured node must have API access to query the cluster-wide endpoints
3. Ensure the API token has sufficient permissions to access cluster resources

## Troubleshooting

If you encounter issues with the cluster resources endpoint:

1. Pulse will automatically fall back to the standard node-by-node approach if there's an error using the cluster resources endpoint
2. Check the logs for any errors related to the cluster resources endpoint
3. Verify that your API token has the correct permissions

## Example Configuration

Minimal configuration for cluster mode:

```
# Proxmox connection details - only need one node from the cluster
PROXMOX_NODE_1_NAME=pve01
PROXMOX_NODE_1_HOST=https://pve01.domain.local:8006
PROXMOX_NODE_1_TOKEN_ID=pulse-monitor@pve!pulse
PROXMOX_NODE_1_TOKEN_SECRET=your-token-secret-here

# Enable cluster mode features
PROXMOX_CLUSTER_MODE=true
PROXMOX_USE_CLUSTER_RESOURCES=true
PROXMOX_CLUSTER_NAME=my-cluster
``` 