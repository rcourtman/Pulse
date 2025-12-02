# Pulse Architecture

Pulse is a real-time monitoring system designed for Proxmox VE, Proxmox Backup Server, and Docker/Host infrastructure. It is built with a **Go** backend and a **SolidJS** frontend, focusing on low latency, high concurrency, and a premium user experience.

## 🏗 High-Level Overview

The system operates as a single binary that serves both the API and the static frontend assets. It connects to Proxmox infrastructure via their REST APIs (using API tokens or password auth), while Docker/Host metrics are collected by lightweight agents that push data to Pulse.

```mermaid
graph TD
    User[User Browser] <-->|WebSocket / HTTP| Pulse[Pulse Server]

    subgraph "Pulse Server (Go)"
        API[REST API]
        WS[WebSocket Hub]
        Monitor[Monitoring Engine]
        Config[Config Manager]
    end

    Pulse -->|HTTPS API :8006| PVE[Proxmox VE Node]
    Pulse -->|HTTPS API :8007| PBS[Proxmox Backup Server]
    Agent[Pulse Agent] -->|HTTPS POST| API
    Agent -.->|Collects from| DockerHost[Docker / Host]

    Monitor --> WS
    Monitor --> API
```

## 🔌 Backend Architecture (Go)

The backend is a high-performance Go application designed for concurrent monitoring.

### Core Components

1.  **Entry Point (`cmd/pulse/main.go`)**:
    *   Initializes the configuration, logger, and persistence layer.
    *   Starts the `ReloadableMonitor` which manages the lifecycle of monitoring routines.
    *   Launches the HTTP server and WebSocket hub.

2.  **Monitoring Engine (`internal/monitoring`)**:
    *   **Polymorphic Monitors**: Uses interfaces to treat PVE and PBS hosts uniformly where possible.
    *   **Goroutines**: Each host is monitored in its own lightweight goroutine to ensure non-blocking operations.
    *   **API Clients**: Communicates with Proxmox VE/PBS via their REST APIs using API tokens or password-based tickets.

3.  **Agent Receivers (`internal/api/agents`)**:
    *   Receives metrics from `pulse-agent` instances via HTTP POST.
    *   Agents collect Docker container stats, host metrics, and temperatures locally.
    *   Push-based model: agents initiate connections to Pulse, not vice versa.

4.  **WebSocket Hub (`internal/websocket`)**:
    *   Manages active client connections.
    *   Broadcasts metric updates in real-time.
    *   Handles "commands" from the frontend (e.g., requesting immediate updates).

5.  **API Layer (`internal/api`)**:
    *   RESTful endpoints for configuration (adding nodes, setting thresholds).
    *   Handles authentication and secure token management.

### Data Flow

1.  **Collection**:
    *   **Proxmox**: The Monitoring Engine polls PVE/PBS REST APIs (default: 2s interval).
    *   **Agents**: Docker/Host agents push metrics to Pulse at their configured interval (default: 30s).
2.  **Normalization**: API responses and agent reports are parsed into standardized Go structs (`HostMetrics`, `ContainerMetrics`).
3.  **Broadcast**: Normalized data is sent to the `WebSocket Hub`.
4.  **Delivery**: The Hub serializes the data to JSON and pushes it to all subscribed frontend clients.

## 🎨 Frontend Architecture (SolidJS)

The frontend is a modern Single Page Application (SPA) built with **SolidJS** and **TypeScript**. It prioritizes performance by using fine-grained reactivity instead of a Virtual DOM.

### Key Technologies
*   **SolidJS**: For reactive UI components.
*   **TailwindCSS**: For styling and theming (Dark/Light mode).
*   **Vite**: For fast development and optimized builds.

### State Management
*   **Stores (`frontend-modern/src/stores`)**:
    *   `websocket.ts`: The central nervous system. It maintains the WS connection, handles reconnection logic, and updates reactive signals when new data arrives.
    *   `metricsHistory.ts`: Buffers incoming metrics to drive historical charts (Sparklines) without needing a time-series database backend.

### Component Design
*   **Atomic Design**: Small, reusable components (`MetricBar`, `StatusBadge`) compose into larger views (`NodeSummaryTable`).
*   **Visualizations**: Custom SVG-based charts (Sparklines) are used instead of heavy charting libraries to keep the bundle size small and rendering fast.

## 🔒 Security

*   **Encryption at Rest**: Sensitive configuration (passwords, API keys) is encrypted on disk using `AES-GCM` with a user-provided passphrase.
*   **Transport Security**: All communications can be secured via TLS.
*   **Authentication**: Session-based auth for API access.

## 🚀 Deployment

Pulse is distributed as:
1.  **Docker Container**: Multi-stage build resulting in a scratch-based or alpine-based image containing just the binary and frontend assets.
2.  **Single Binary**: The frontend is embedded into the Go binary using `embed`, allowing for a single-file deployment.
