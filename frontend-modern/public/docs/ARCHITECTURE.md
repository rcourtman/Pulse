# Pulse Architecture

Pulse is a real-time infrastructure monitoring platform for **Proxmox VE**,
**Proxmox Backup Server**, **Proxmox Mail Gateway**, **Docker**, **machine
agents**, **Kubernetes**, **TrueNAS**, and early-access **VMware vSphere**
environments. It uses a **Go 1.26** backend and a **SolidJS / TypeScript**
frontend.

## 🏗 High-Level Overview

The system runs as a single binary that serves both the API and the embedded frontend assets. It connects to infrastructure via platform-supported HTTP and WebSocket APIs and lightweight push-based agents, normalises everything into a **Unified Resource model**, and delivers real-time updates to clients over WebSocket.

```mermaid
flowchart TD
    User[Browser / Mobile] <-->|WebSocket + HTTP| Pulse[Pulse Server]
    Mobile[Mobile App] <-->|E2E Encrypted| Relay[Relay Server]
    Relay <-->|WebSocket| Pulse

    subgraph "Pulse Server (Go)"
        API[Decomposed API Router]
        WS[WebSocket Hub]
        Monitor[Reloadable Monitor]
        UR[Unified Resource Registry]
        AI[AI Subsystem]
        Config[Config Manager]
        License[Entitlements Engine]
        Audit[Audit Logger]
    end

    Pulse -->|HTTPS :8006| PVE[Proxmox VE]
    Pulse -->|HTTPS :8007| PBS[Proxmox Backup Server]
    Pulse -->|HTTPS| PMG[Proxmox Mail Gateway]
    Pulse -->|HTTPS| TrueNAS[TrueNAS SCALE/CORE]
    Pulse -->|HTTPS| vSphere[VMware vCenter]
    DockerAgent[Docker Agent] -->|HTTPS POST| API
    HostAgent[Host Agent] -->|HTTPS POST| API
    K8sAgent[Kubernetes Agent] -->|HTTPS POST| API

    Monitor --> UR
    UR --> WS
    UR --> API
```

## 🔌 Backend Architecture (Go)

All backend code lives under `cmd/`, `internal/`, and `pkg/`. The binary is assembled from `cmd/pulse/main.go` which delegates to `pkg/server.Run()`.

### Core Components

1. **Entry Point (`cmd/pulse/main.go` → `pkg/server/server.go`)**
   - Loads unified configuration via `internal/config.Load()`.
   - Initialises the RBAC manager (`pkg/auth`), audit logger (`pkg/audit`), and crypto layer (`internal/crypto`).
   - Creates a `ReloadableMonitor` that manages the lifecycle of all monitoring goroutines.
   - Starts the HTTP/HTTPS server, WebSocket hub, AI services (Patrol + Chat), and the Relay client.
   - Supports graceful hot-reload via `SIGHUP` and `.env` file watching.

2. **Unified Resource Registry (`internal/unifiedresources`)**
   - Central data model that normalises resources from Proxmox, PBS, PMG,
     Docker, machine agents, Kubernetes, TrueNAS, and VMware providers into a
     shared `Resource` contract.
   - **Canonical v6 resource types**: `agent`, `vm`, `system-container`, `app-container`, `docker-host`, `k8s-cluster`, `k8s-node`, `pod`, `k8s-deployment`, `storage`, `pbs`, `pmg`, `ceph`, `physical_disk`.
   - Identity-matching engine: merges resources across sources using machine IDs, DMI UUIDs, hostnames, IPs, and MAC addresses.
   - Provides typed **views** (`NodeView`, `K8sClusterView`, etc.) for consumer-specific queries.
   - Canonical API endpoint: `GET /api/resources`.

3. **Monitoring Engine (`internal/monitoring`)**
   - **Polymorphic monitors**: Each Proxmox VE/PBS/PMG node runs in its own goroutine, polling via the platform REST API.
   - **Agent receivers** (`internal/api`): Docker, Host, and Kubernetes agents push metrics via HTTP POST to `/api/agents/{type}/report`.
   - **TrueNAS provider** (`internal/truenas`): Uses the versioned JSON-RPC 2.0
     WebSocket API on supported SCALE releases for system info, pools,
     datasets, disks, alerts, ZFS snapshots, and replication tasks. A
     version-gated REST path remains only for legacy SCALE and CORE systems.
   - Enterprise/internal multi-org aware: when `PULSE_MULTI_TENANT_ENABLED=true`, each organisation gets a separate shared-process monitor namespace with its own configuration.

4. **WebSocket Hub (`internal/websocket`)**
   - Manages active browser connections with per-message compression (deflate).
   - Broadcasts state diffs to all subscribed clients; supports per-organization broadcasts for internal multi-org setups.
   - Enforces origin validation, organization-level authorization, and Enterprise/internal multi-org license gating.

5. **Decomposed API Router (`internal/api`)**
   - The router is split into focused registration files for maintainability:
     - `router_routes_auth_security.go` — Auth, OIDC, SAML, SSO, security tokens, recovery, agent install scripts.
     - `router_routes_monitoring.go` — Unified resources, metrics history, charts, recovery points, alerts, notifications, discovery.
     - `router_routes_ai_relay.go` — AI settings, Patrol, Intelligence, Chat sessions, Relay config, and the approval workflow.
     - `router_routes_org_license.go` — Organisations, RBAC, audit logs, license/entitlements, billing.
     - `router_routes_registration.go` — Config CRUD, TrueNAS connections, update management, agent profiles, system settings.
     - `router_routes_hosted.go` — Hosted/SaaS-specific signup and org admin routes.
   - Every endpoint enforces **scoped access control** (e.g., `monitoring:read`, `settings:write`, `ai:execute`, `ai:chat`).

6. **Relay Client (`internal/relay`)**
   - Maintains a persistent WebSocket tunnel to a managed relay server for **mobile remote access**.
   - Uses an ECDH key-exchange for per-channel **end-to-end encryption**.
   - Multiplexes multiple mobile sessions over a single connection with back-pressure (data limiter) and per-channel authentication.
   - Supports push notifications through the relay.
   - Gated by the `relay` license feature.

7. **AI Subsystem (`internal/ai`)**
   - **Pulse Assistant (Chat)**: Interactive LLM-powered chat with infrastructure context. Supports bring-your-own-key (BYOK) providers (OpenAI, Anthropic, Ollama, etc.) plus Claude OAuth.
   - **Pulse Patrol**: Scheduled background analysis that produces findings, predictions, and remediation plans. Autonomy levels: `monitor`, `approval`, `assisted`, `full`.
   - **Intelligence Services**: Patterns, correlations, anomalies, baselines, forecasts, and incident recording. All surfaced via `/api/ai/intelligence/*`.
   - **Safety gates**: Command execution disabled by default (`--enable-commands` opt-in); circuit breakers and scoped permissions at every layer.

8. **Entitlements & Licensing (`pkg/licensing`)**
   - Capability-key based gating: `ai_autofix`, `rbac`, `multi_tenant`, `relay`, `agent_profiles`, `kubernetes_ai`, `ai_alerts`, etc.
   - Core tiers include **Community** (free), **Relay**, **Pro**, a reserved
     hosted **Cloud** capability tier, request-assisted **MSP**, and
     Enterprise/custom entitlements. The Cloud service is not generally
     available.
   - Activation, grant refresh, renewal, expiry, and legacy-license migration.
     Active state is exposed through `/api/license/*`.

9. **Provider-hosted MSP control plane (`internal/cloudcp`)**
   - A Stripe-free provider control plane can run one isolated Pulse runtime/container per client workspace.
   - A signed MSP license sets the provider plan and client workspace cap.
   - Pulse Account handles the provider workspace roster, sign-in-once handoff, client-bound agent install paths, and provider access surfaces.
   - Each client runtime remains a normal Pulse instance for monitoring data, alerts, webhooks, reports, users, and audit history. Shared-process organizations are for Enterprise/internal multi-org, not the default MSP boundary.

10. **Recovery Engine (`internal/recovery`)**
   - Aggregates backup data (PBS snapshots, ZFS snapshots, replication tasks) into a unified recovery point timeline.
   - Provides faceted queries: filter by type, source, status, and source health.
   - API: `/api/recovery/points`, `/api/recovery/series`, `/api/recovery/facets`, `/api/recovery/rollups`.

11. **Audit Logging (`pkg/audit`)**
    - Defence-in-depth: every mutation is logged to SQLite, optionally signed with per-tenant encryption keys.
    - Async logging mode (`PULSE_AUDIT_ASYNC`) for high-throughput environments.
    - Tenant-aware logger manager for isolated per-org audit trails.

### Data Flow

1. **Collection**:
   - **Proxmox VE / PBS / PMG**: Monitoring engine polls platform REST APIs (configurable interval, default 2 s for PVE).
   - **Docker / Host / Kubernetes**: Lightweight agents push metrics via HTTP POST on their configured interval.
   - **TrueNAS**: Provider polls the versioned JSON-RPC WebSocket API on
     supported SCALE releases, with REST compatibility limited to recognized
     legacy SCALE and CORE systems.
2. **Normalisation**: Platform-specific responses are mapped into `unifiedresources.Resource` structs by adapters in `internal/unifiedresources/adapters.go`.
3. **Registration**: Resources are inserted into the in-memory registry, which handles deduplication, identity matching, and status computation.
4. **Broadcast**: The latest state snapshot is serialised to JSON and pushed to all connected WebSocket clients by the Hub.
5. **Persistence**: Metrics history is stored in a SQLite-backed metrics store (`pkg/metrics`) with configurable retention.

## 🎨 Frontend Architecture (SolidJS)

The frontend is a modern SPA in `frontend-modern/`, built with **SolidJS** and **TypeScript**. It uses fine-grained reactivity (no Virtual DOM) for maximum performance.

### Key Technologies
- **SolidJS**: Reactive UI framework.
- **TailwindCSS** with a **semantic design token layer**: All structural colours use CSS custom-property tokens (`bg-base`, `bg-surface`, `text-base-content`, `border-border`, etc.) defined in `index.css`— components never hardcode hex values. Light/dark mode switches automatically via class-based theme resolution.
- **Vite**: Build tooling (dev server + production bundling).
- **Lucide Icons**: Icon library (imported as solid components).

### Routing & Navigation

Navigation is organised around platform-shaped pages with cross-platform
operational surfaces:

| Route | Page | Purpose |
|---|---|---|
| `/` and `/infrastructure` | Runtime home | Monitor-first authenticated entry point |
| `/proxmox/*` | Proxmox | PVE, PBS, PMG, guests, storage, recovery, and Ceph |
| `/docker/*` | Docker | Hosts, containers, Compose projects, Swarm, images, and storage |
| `/kubernetes/*` | Kubernetes | Clusters, workloads, networking, storage, and events |
| `/truenas/*` | TrueNAS | Systems, pools, datasets, disks, apps, VMs, and recovery |
| `/vmware/*` | vSphere | Early-access vCenter, host, cluster, VM, datastore, and network views |
| `/standalone/*` | Machines | Agent-backed machines and availability checks |
| `/alerts/*` | Alerts | Alert rules, active alerts, history |
| `/actions/*` | Actions | Governed action proposals, approvals, delivery, and audit state |
| `/patrol/*` | Patrol | Attention queue, findings, investigations, and run history |
| `/settings/*` | Settings | Infrastructure, security, notifications, plans, and Intelligence |

The retired aggregate `/workloads`, `/storage`, and `/recovery` top-level
routes are not canonical navigation. Shared resource, storage, and recovery
contracts remain backend building blocks consumed inside platform pages.

### State Management
- **WebSocket store** (`stores/websocket.ts`): Manages the live connection, reactive `State` object, reconnection logic, and per-org switching.
- **Metrics collector** (`stores/metricsCollector.ts`): Buffers time-series data for sparklines and historical charts.
- **AI stores** (`stores/aiChat.ts`, `stores/aiIntelligence.ts`): Manage chat sessions and patrol findings.
- **License store** (`stores/license.ts`): Tracks plan tier, feature flags, and multi-tenant state.
- **System settings store** (`stores/systemSettings.ts`): Caches server-side preferences (for example theme and feature toggles).

### Component Design
- **Shared primitives** in `components/shared/`: `Card`, `Button`, `Toggle`, `FilterButtonGroup`, `Table` — all mapped to the semantic design tokens.
- **Lazy-loaded pages**: All top-level pages are loaded via `lazy()` with optional preloading after initial render.
- **Virtual table windowing**: Large resource lists use virtualised rendering for smooth scrolling at scale.
- **Command Palette** (`Cmd/Ctrl+K`): Quick-access command launcher.
- **Keyboard shortcuts**: `g p` → Proxmox, `g d` → Docker, `g k` →
  Kubernetes, `g n` → TrueNAS, `g v` → vSphere, `g s` → Machines, `g a` →
  Alerts, `g r` → Patrol, `g t` → Settings, `/` → Search.

### Mobile Experience
- **MobileNavBar** component: Bottom tab bar for touch navigation.
- **Relay integration**: The mobile app connects through the relay protocol for encrypted remote access.

## 🔒 Security

- **Encryption at Rest**: Sensitive config (passwords, API keys) encrypted with AES-GCM via `internal/crypto`. Each tenant can have its own encryption key.
- **Scoped API Tokens**: Tokens carry explicit scopes (`monitoring:read`, `settings:write`, `ai:chat`, `ai:execute`, `docker:report`, etc.). Endpoints enforce scope checks before processing.
- **RBAC**: Role-based access control via `pkg/auth` with file-backed persistence. Resources: `users`, `settings`, `monitoring`.
- **SSO**: OIDC and SAML providers with per-provider configuration and multi-provider support.
- **Agent Commands**: Disabled by default for security. Operators must opt in with `--enable-commands`.
- **Audit Trail**: Every mutation logged to per-tenant SQLite databases with optional cryptographic signatures.
- **Rate Limiting**: Per-endpoint and per-tenant rate limiting with configurable thresholds.
- **CSRF Protection**: Token-based CSRF prevention on mutation endpoints.
- **Recovery Mode**: Localhost-only endpoint for emergency auth recovery with time-limited tokens.

## 🚀 Deployment

Pulse is distributed as:

1. **Docker Container**: Multi-stage build producing a minimal image with the Go binary + embedded frontend.
2. **Single Binary**: The frontend is compiled into the Go binary using `go:embed` (`internal/api/frontend_embed.go`), enabling single-file deployment.
3. **Kubernetes Helm Chart**: For cluster deployments with configurable replicas and persistence.
4. **Systemd Service**: For bare-metal or VM installations with `.env`-based configuration and `SIGHUP` reload support.
5. **Proxmox LXC Helper Script**: One-line install inside a Proxmox helper-scripts container.
