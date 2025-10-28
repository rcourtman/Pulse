# Docker Monitoring Agent

Pulse is focused on Proxmox VE and PBS, but many homelabs also run application stacks in Docker. The optional Pulse Docker agent turns container health and resource usage into first-class metrics that show up alongside your hypervisor data.

## What the agent reports

Every check interval (30s by default) the agent collects:

- Host metadata (hostname, Docker version, CPU count, total memory, uptime)
- Container status (`running`, `exited`, `paused`) and health probe state
- Restart counters and exit codes
- CPU usage, memory consumption and limits
- Images, port mappings, network addresses, and start times
- Health-check failures, restart-loop windows, and recent exit codes (displayed in the UI under each container drawer)

Data is pushed to Pulse over HTTPS using your existing API token – no inbound firewall rules required.

## Prerequisites

- Pulse v4.22.0 or newer with an API token enabled (`Settings → Security`)
- API token with the `docker:report` scope (add `docker:manage` if you use remote lifecycle commands)
- Docker 20.10+ on Linux (the agent uses the Docker Engine API via the local socket)
- Access to the Docker socket (`/var/run/docker.sock`) or a configured `DOCKER_HOST`
- Go 1.24+ if you plan to build the binary from source

## Installation

Grab the `pulse-docker-agent` binary from the release assets (or build it yourself):

```bash
# Build from source
cd /opt/pulse
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pulse-docker-agent ./cmd/pulse-docker-agent
```

Copy the binary to your Docker host (e.g. `/usr/local/bin/pulse-docker-agent`) and make it executable.

> **Why `CGO_ENABLED=0`?** Building a fully static binary ensures the agent runs on hosts still using older glibc releases (for example Debian 11 with glibc 2.31).

### Quick install from your Pulse server

Use the bundled installation script (ships with Pulse v4.22.0+) to deploy and manage the agent. Replace the token placeholder with an API token generated in **Settings → Security**. Create a dedicated token for each Docker host so you can revoke individual credentials without touching others—sharing one token across many hosts makes incident response much harder. Tokens used here should include the `docker:report` scope so the agent can submit telemetry (add `docker:manage` only if you plan to issue lifecycle commands remotely).

```bash
curl -fsSL http://pulse.example.com/install-docker-agent.sh \
  | sudo bash -s -- --url http://pulse.example.com --token <api-token>
```

> **Why sudo?** The installer needs to drop binaries under `/usr/local/bin`, create a systemd service, and start it—actions that require root privileges. Piping to `sudo bash …` saves you from retrying if you run the command as an unprivileged user.

Running the one-liner again from another Pulse server (with its own URL/token) will merge that server into the same agent automatically—no extra flags required.

To report to more than one Pulse instance from the same Docker host, repeat the `--target` flag (format: `https://pulse.example.com|<api-token>`) or export `PULSE_TARGETS` before running the script:

```bash
curl -fsSL http://pulse.example.com/install-docker-agent.sh \
  | sudo bash -s -- \
    --target https://pulse.example.com|<primary-token> \
    --target https://pulse-dr.example.com|<dr-token>
```

## Running the agent

The agent needs to know where Pulse lives and which API token to use.

**Single instance:**

```bash
export PULSE_URL="http://pulse.lan:7655"
export PULSE_TOKEN="<your-api-token>"

sudo /usr/local/bin/pulse-docker-agent --interval 30s
```

**Multiple instances (one agent fan-out):**

```bash
export PULSE_TARGETS="https://pulse-primary.lan:7655|<token-primary>;https://pulse-dr.lan:7655|<token-dr>"

sudo /usr/local/bin/pulse-docker-agent --interval 30s
```

You can also repeat `--target https://pulse.example.com|<token>` on the command line instead of using `PULSE_TARGETS`; the agent will broadcast each heartbeat to every configured URL.

The binary reads standard Docker environment variables. If you already use TLS-secured remote sockets set `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, etc. as normal. To skip TLS verification for Pulse (not recommended) add `--insecure` or `PULSE_INSECURE_SKIP_VERIFY=true`.

### Filtering container states

High churn environments can flood Pulse with noise from short-lived tasks. Restrict the agent to the container states you care about by repeating `--container-state` (for example, `--container-state running --container-state paused`) or by exporting `PULSE_CONTAINER_STATES=running,paused`. Allowed values match Docker’s status filter: `created`, `running`, `restarting`, `removing`, `paused`, `exited`, and `dead`. If no values are provided the agent reports every container, mirroring the previous behaviour.

### Swarm-aware reporting

The agent now recognises Docker Swarm roles. Managers query the Swarm control plane for service and task metadata, while workers fall back to the labels present on local containers. The **Settings → Docker Agents** view surfaces role, scope, service counts, and updates per host so you can spot noisy stacks or unhealthy rollouts at a glance.

Use the new flags to tune the payload:

- `--swarm-scope` / `PULSE_SWARM_SCOPE` chooses between node-only and cluster-wide aggregation (`auto` switches based on the node’s role).
- `--swarm-services` and `--swarm-tasks` disable service or task blocks if you only need a subset of data.
- `--include-containers` removes per-container metrics when service-level reporting is sufficient (note that workers need container data to derive task info).

If a manager cannot reach the Swarm API the agent automatically falls back to node scope so updates keep flowing.

Adjust warning and critical replica gaps (or disable service alerts entirely) under **Alerts → Thresholds → Docker** in the Pulse UI.

### Multiple Pulse instances

A single `pulse-docker-agent` process can now serve any number of Pulse backends. Each target entry keeps its own API token and TLS preference, and Pulse de-duplicates reports using the shared agent ID / machine ID. This avoids running duplicate agents on busy Docker hosts.

### Systemd unit example

```ini
[Unit]
Description=Pulse Docker Agent
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
Environment=PULSE_URL=https://pulse.example.com
Environment=PULSE_TOKEN=replace-me
Environment=PULSE_TARGETS=https://pulse.example.com|replace-me;https://pulse-dr.example.com|replace-me-dr
ExecStart=/usr/local/bin/pulse-docker-agent --interval 30s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Containerised agent (advanced)

If you prefer to run the agent inside a container, mount the Docker socket and supply the same environment variables:

```bash
docker run -d \
  --name pulse-docker-agent \
  -e PULSE_URL="https://pulse.example.com" \
  -e PULSE_TOKEN="<token>" \
  -e PULSE_TARGETS="https://pulse.example.com|<token>;https://pulse-dr.example.com|<token-dr>" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --restart unless-stopped \
  ghcr.io/rcourtman/pulse-docker-agent:latest
```

> **Note**: Official images for `linux/amd64` and `linux/arm64` are published to `ghcr.io/rcourtman/pulse-docker-agent`. To test local changes, run `docker build --target agent_runtime -t pulse-docker-agent:test .` from the repository root.

## Configuration reference

| Flag / Env var          | Description                                               | Default         |
| ----------------------- | --------------------------------------------------------- | --------------- |
| `--url`, `PULSE_URL`    | Pulse base URL (http/https).                              | `http://localhost:7655` |
| `--token`, `PULSE_TOKEN`| Pulse API token with `docker:report` scope (required).    | —               |
| `--target`, `PULSE_TARGETS` | One or more `url|token[|insecure]` entries to fan-out reports to multiple Pulse servers. Separate entries with `;` or repeat the flag. | — |
| `--interval`, `PULSE_INTERVAL` | Reporting cadence (supports `30s`, `1m`, etc.).     | `30s`           |
| `--container-state`, `PULSE_CONTAINER_STATES` | Limit reports to specific Docker statuses (`created`, `running`, `restarting`, `removing`, `paused`, `exited`, `dead`). Separate multiple values with commas/semicolons or repeat the flag. | — |
| `--swarm-scope`, `PULSE_SWARM_SCOPE` | Swarm data scope: `node`, `cluster`, or `auto` (auto picks cluster on managers, node on workers). | `node` |
| `--swarm-services`, `PULSE_SWARM_SERVICES` | Include Swarm service summaries in reports. | `true` |
| `--swarm-tasks`, `PULSE_SWARM_TASKS` | Include individual Swarm tasks in reports. | `true` |
| `--include-containers`, `PULSE_INCLUDE_CONTAINERS` | Include per-container metrics (disable when only Swarm data is needed). | `true` |
| `--hostname`, `PULSE_HOSTNAME` | Override host name reported to Pulse.              | Docker info / OS hostname |
| `--agent-id`, `PULSE_AGENT_ID` | Stable ID for the agent (useful for clustering).   | Docker engine ID / machine-id |
| `--insecure`, `PULSE_INSECURE_SKIP_VERIFY` | Skip TLS cert validation (unsafe).     | `false`         |

The agent automatically discovers the Docker socket via the usual environment variables. To use SSH tunnels or TCP sockets, export `DOCKER_HOST` as you would for the Docker CLI.

### Suppressing ephemeral containers

CI runners and short-lived build containers can generate noisy state alerts when they exit on schedule. In Pulse v4.24.0 and later you can provide a list of prefixes to ignore under **Alerts → Thresholds → Docker → Ignored container prefixes**. Any container whose name *or* ID begins with a configured prefix is skipped for state, health, metric, restart-loop, and OOM alerts. Matching is case-insensitive and the list is saved as `dockerIgnoredContainerPrefixes` inside `alerts.json`. Use one entry per family of ephemeral containers (for example, `runner-` or `gitlab-job-`).

Need the alerts but at a different tone? The same Docker tab exposes global controls for the container state detector. Flip **Disable container state alerts** (`stateDisableConnectivity`) to mute powered-off/offline warnings across the fleet, or change **Default severity** (`statePoweredOffSeverity`) to `critical` so unexpected exits page immediately. Individual host/container overrides still win when you need exceptions.

## Testing and troubleshooting

- Run with `--interval 15s --insecure` in a terminal to see log output while testing.
- Ensure the Pulse API token has not expired or been regenerated.
- If `pulse-docker-agent` reports `Cannot connect to the Docker daemon`, verify the socket path and permissions.
- Check Pulse (`/docker` tab) for the latest heartbeat time. Hosts are marked offline if they stop reporting for >4× the configured interval.
- Use the search box above the host grid to filter by host name, stack label, or container name. Restart loops surface in the “Issues” column and display the last five exit codes.

## Removing the agent

Stop the systemd service or container and remove the binary. Pulse retains the last reported state until it ages out after a few minutes of inactivity.
