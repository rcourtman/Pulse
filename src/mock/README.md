# Mock Data Implementation for Pulse

This directory contains the mock data implementation for the Pulse application. It provides a consistent set of mock data for development and testing purposes.

## Key Components

- **custom-data.ts**: The single source of truth for all mock data. This file defines the nodes and guests with their properties.
- **server.ts**: The mock server implementation that serves the mock data via REST and WebSocket APIs.
- **run-server.ts**: Script to run the mock server independently.
- **templates.ts**: Templates for VM and container names (used only for reference, not active data generation).

## Mock Data Guidelines

1. **Single Source of Truth**: All mock data comes from `custom-data.ts`. No other parts of the application should generate mock guests.

2. **10 Guests Per Node Limit**: Each node is limited to a maximum of 10 guests (a mix of VMs and containers) to prevent UI overload and ensure consistent performance. This includes shared guests from other nodes.

3. **Realistic Data**: The mock data is designed to mimic realistic Proxmox deployments with appropriate resource allocation and naming conventions.

4. **Shared Guests**: Some guests are marked as "shared" between nodes to demonstrate clustering features. These shared guests have a `primaryNode` property indicating which node is responsible for their metrics.

5. **Consistent Guest Display**: The mock server generates metrics for ALL guests, not just running ones. This ensures the UI displays a consistent list of guests without flickering or changing size, even as guest statuses change.

## Cluster Simulation

The mock data includes a simulation of Proxmox clustering with shared guests. Here's how it works:

1. **Shared Guest Definition**: In `custom-data.ts`, guests can be marked as shared by setting `shared: true` and specifying a `primaryNode` property.

2. **Visibility Across Nodes**: Shared guests appear on all nodes in the cluster, not just on their home node.

3. **Running Status**: A shared guest only runs on its primary node. On other nodes, it appears as "stopped".

4. **Resource Usage**: CPU and memory usage metrics are only generated for the primary node. Other nodes show zero resource usage.

5. **API Consistency**: The `/api/resources` endpoint shows shared guests on all nodes, matching how Proxmox presents cluster resources.

6. **Metrics Generation**: Metrics are generated for all guests (including shared ones) but with appropriate values based on primary status.

7. **Guest Distribution**: Shared guests are automatically distributed to all nodes during initialization, ensuring a consistent view across all parts of the application (REST API, WebSocket connections, etc.).

8. **Enforcing the 10 Guest Limit**: When distributing shared guests, we ensure that no node exceeds 10 total guests. If adding all shared guests would exceed this limit, we prioritize keeping guests that have the current node as their primary node.

## Preventing Guests from Appearing and Disappearing

To prevent guests from appearing and disappearing, causing UI flicker:

1. **One-time Distribution**: All shared guests are distributed across nodes during server initialization, before any clients connect.

2. **Consistent References**: We use the same guests map for both the REST API and WebSocket connections, ensuring both interfaces show the same guests.

3. **Fixed Node Sets**: Once guests are distributed during initialization, the set of guests on each node remains fixed - no guests are added or removed dynamically.

4. **Consistent Status**: A shared guest's status is determined by whether it's on its primary node, and this status remains consistent throughout the application.

5. **Metrics for All Guests**: Even stopped guests have metrics (with zeros for CPU/memory), ensuring they remain in the UI.

## How to Modify Mock Data

If you need to modify the mock data:

1. Edit the `customMockData` object in `custom-data.ts`.
2. Maintain the 10 guests per node limit.
3. Ensure each guest has consistent properties (id, name, type, status, cpu, memory, disk).
4. For shared guests, set the `shared` flag to true and specify the `primaryNode`.

## Running the Mock Server

The mock server is automatically started when running the application in development mode with mock data enabled:

```bash
npm run dev
```

The mock server runs on port 7656 by default. 