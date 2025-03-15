# Testing the Cluster Resources Feature

This document provides instructions for testing the new Proxmox cluster resources endpoint feature, which allows Pulse to fetch all VMs and containers from a Proxmox cluster in a single API call.

## Testing with Mock Data

The easiest way to test the cluster resources feature is to use the mock data server, which simulates a Proxmox cluster with multiple nodes and guests.

### Prerequisites

- Node.js 20 or later
- npm

### Setup

1. Clone the repository and navigate to the project directory:
   ```bash
   git clone https://github.com/rcourtman/pulse.git
   cd pulse
   ```

2. Install dependencies:
   ```bash
   npm install
   ```

3. Run the test script:
   ```bash
   ./test-cluster-resources.sh
   ```

   This script will:
   - Copy the test environment file to `.env`
   - Start the development server with mock data and the cluster resources endpoint enabled

4. Open your browser and navigate to `http://localhost:7654` to see the dashboard.

### What to Look For

When testing with mock data, you should observe:

1. All VMs and containers from all nodes in the mock cluster are displayed correctly.
2. The console logs should show messages indicating that the cluster resources endpoint is being used.
3. Resources should be properly organized by node.
4. There should be no duplicate VMs or containers.

## Testing with a Real Proxmox Cluster

If you have access to a real Proxmox cluster, you can test the feature with actual data.

### Configuration

1. Copy the `.env.example` file to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Edit the `.env` file to configure your Proxmox cluster:
   - Set `PROXMOX_NODE_1_NAME`, `PROXMOX_NODE_1_HOST`, `PROXMOX_NODE_1_TOKEN_ID`, and `PROXMOX_NODE_1_TOKEN_SECRET` to connect to your Proxmox node.
   - Set `PROXMOX_USE_CLUSTER_RESOURCES=true` to enable the cluster resources endpoint.

3. Start the server:
   ```bash
   npm run dev
   ```

4. Open your browser and navigate to `http://localhost:7654` to see the dashboard.

## Troubleshooting

If you encounter issues with the cluster resources feature:

1. Check the console logs for error messages.
2. Verify that your Proxmox cluster is properly configured.
3. Ensure that the API token has sufficient permissions to access the cluster resources endpoint.
4. If the cluster resources endpoint fails, Pulse will automatically fall back to the standard approach of querying each node separately.

## Fallback Mechanism

The implementation includes a fallback mechanism that will revert to the standard approach if the cluster resources endpoint fails. This ensures backward compatibility and robustness.

To test the fallback mechanism:

1. Set `PROXMOX_USE_CLUSTER_RESOURCES=true` in your `.env` file.
2. Intentionally cause the cluster resources endpoint to fail (e.g., by using an invalid API token).
3. Observe that Pulse falls back to querying each node separately.

## Reporting Issues

If you encounter any issues with the cluster resources feature, please report them on the GitHub repository:
https://github.com/rcourtman/pulse/issues 