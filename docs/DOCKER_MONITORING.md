# Docker Monitoring Agent

Pulse is focused on Proxmox VE and PBS, but many homelabs also run application stacks in Docker. The optional Pulse Docker agent turns container health and resource usage into first-class metrics that show up alongside your hypervisor data.

## What the agent reports

Every check interval (30s by default) the agent collects:

- Host metadata (hostname, Docker version, CPU count, total memory, uptime)
- Container status (`running`, `exited`, `paused`) and health probe state
- Restart counters and exit codes
- CPU usage, memory consumption and limits
- Images, port mappings, network addresses, and start times

Data is pushed to Pulse over HTTPS using your existing API token – no inbound firewall rules required.

## Prerequisites

- Pulse vX.Y.Z or newer with an API token enabled (`Settings → Security`)
- Docker 20.10+ on Linux (the agent uses the Docker Engine API via the local socket)
- Access to the Docker socket (`/var/run/docker.sock`) or a configured `DOCKER_HOST`
- Go 1.24+ if you plan to build the binary from source

## Installation

Grab the `pulse-docker-agent` binary from the release assets (or build it yourself):

```bash
# Build from source
cd /opt/pulse
GOOS=linux GOARCH=amd64 go build -o pulse-docker-agent ./cmd/pulse-docker-agent
```

Copy the binary to your Docker host (e.g. `/usr/local/bin/pulse-docker-agent`) and make it executable.

## Running the agent

The agent needs to know where Pulse lives and which API token to use. You can supply these via flags or environment variables:

```bash
export PULSE_URL="http://pulse.lan:7655"
export PULSE_TOKEN="<your-api-token>"

sudo /usr/local/bin/pulse-docker-agent --interval 30s
```

The binary reads standard Docker environment variables. If you already use TLS-secured remote sockets set `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, etc. as normal. To skip TLS verification for Pulse (not recommended) add `--insecure` or `PULSE_INSECURE_SKIP_VERIFY=true`.

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
  -v /var/run/docker.sock:/var/run/docker.sock \
  --restart unless-stopped \
  ghcr.io/rcourtman/pulse-docker-agent:latest
```

> **Note**: The container image will be published starting with the first release that ships this agent. Build locally if you are testing main.

## Configuration reference

| Flag / Env var          | Description                                               | Default         |
| ----------------------- | --------------------------------------------------------- | --------------- |
| `--url`, `PULSE_URL`    | Pulse base URL (http/https).                              | `http://localhost:7655` |
| `--token`, `PULSE_TOKEN`| Pulse API token (required).                               | —               |
| `--interval`, `PULSE_INTERVAL` | Reporting cadence (supports `30s`, `1m`, etc.).     | `30s`           |
| `--hostname`, `PULSE_HOSTNAME` | Override host name reported to Pulse.              | Docker info / OS hostname |
| `--agent-id`, `PULSE_AGENT_ID` | Stable ID for the agent (useful for clustering).   | Docker engine ID / machine-id |
| `--insecure`, `PULSE_INSECURE_SKIP_VERIFY` | Skip TLS cert validation (unsafe).     | `false`         |

The agent automatically discovers the Docker socket via the usual environment variables. To use SSH tunnels or TCP sockets, export `DOCKER_HOST` as you would for the Docker CLI.

## Testing and troubleshooting

- Run with `--interval 15s --insecure` in a terminal to see log output while testing.
- Ensure the Pulse API token has not expired or been regenerated.
- If `pulse-docker-agent` reports `Cannot connect to the Docker daemon`, verify the socket path and permissions.
- Check Pulse (`/docker` tab) for the latest heartbeat time. Hosts are marked offline if they stop reporting for >4× the configured interval.

## Removing the agent

Stop the systemd service or container and remove the binary. Pulse retains the last reported state until it ages out after a few minutes of inactivity.
