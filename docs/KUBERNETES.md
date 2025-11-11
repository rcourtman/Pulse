# Kubernetes Deployment (Helm)

Deploy Pulse to Kubernetes with the bundled Helm chart under `deploy/helm/pulse`. The chart provisions the Pulse hub (web UI + API) and can optionally run the Docker monitoring agent alongside it. Stable builds are published automatically to the GitHub Container Registry (GHCR) whenever a Pulse release goes out.

> The Helm chart ships with the release archives and pairs with the upgraded monitoring engine (staleness tracking, circuit breakers, detailed poll metrics) so Kubernetes clusters benefit from the same adaptive scheduling improvements as bare-metal installs.

## Prerequisites

- Kubernetes 1.24 or newer with access to a default `StorageClass`
- Helm 3.9+
- An ingress controller (only if you plan to expose Pulse through an Ingress)
- (Optional) A Docker-compatible runtime on the nodes where you expect to run the Docker agent; the agent talks to `/var/run/docker.sock`

## Installing from Helm Repository (recommended)

1. Add the Pulse Helm repository:

   ```bash
   helm repo add pulse https://rcourtman.github.io/Pulse/
   helm repo update pulse
   ```

2. Install the chart:

   ```bash
   # Latest version
   helm install pulse pulse/pulse \
     --namespace pulse \
     --create-namespace

   # Or pin to specific version
   helm install pulse pulse/pulse \
     --version 4.28.0 \
     --namespace pulse \
     --create-namespace
   ```

   Check available versions: `helm search repo pulse/pulse --versions`

3. Port-forward the service to finish the first-time security setup:

   ```bash
   kubectl -n pulse port-forward svc/pulse 7655:7655
   ```

4. Browse to `http://localhost:7655`, complete the security bootstrap (admin user + MFA + TLS preferences), and create an API token under **Settings → Security** for any automation or agents you plan to run.

The chart mounts a PersistentVolumeClaim at `/data` so database files, credentials, and configuration survive pod restarts. By default it requests 8 GiB with `ReadWriteOnce` access—adjust via `persistence.*` in `values.yaml`.

## Working From Source (local packaging)

Need to test local modifications or work offline? Install directly from the checked-out repository:

1. Clone the repository and switch into it.

   ```bash
   git clone https://github.com/rcourtman/Pulse.git
   cd Pulse
   ```

2. Render or install the chart from `deploy/helm/pulse`:

   ```bash
   helm upgrade --install pulse ./deploy/helm/pulse \
     --namespace pulse \
     --create-namespace
   ```

3. Continue with the port-forward and initial setup steps described above.

## Common Configuration

Most day-to-day overrides are done in a custom values file:

```yaml
# file: helm-values.yaml
service:
  type: ClusterIP

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: pulse.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - hosts: [pulse.example.com]
      secretName: pulse-tls

server:
  env:
    - name: TZ
      value: Europe/Berlin
  secretEnv:
    create: true
    data:
      API_TOKENS: docker-agent-token
```

### Runtime Logging Configuration (v4.25.0+)

Configure logging behavior via environment variables:

```yaml
server:
  env:
    - name: LOG_LEVEL
      value: info              # debug, info, warn, error
    - name: LOG_FORMAT
      value: json              # json, text, auto
    - name: LOG_FILE
      value: /data/logs/pulse.log  # Optional: mirror logs to file
    - name: LOG_MAX_SIZE
      value: "100"             # MB per log file
    - name: LOG_MAX_BACKUPS
      value: "10"              # Number of rotated logs to keep
    - name: LOG_MAX_AGE
      value: "30"              # Days to retain logs
```

**Note:** Logging changes via environment variables require pod restart. Use **Settings → System → Logging** in the UI for runtime changes without restart.

### Adaptive Polling Configuration (v4.25.0+)

Adaptive polling is **enabled by default** in v4.25.0. Configure via environment variables:

```yaml
server:
  env:
    - name: ADAPTIVE_POLLING_ENABLED
      value: "true"            # Enable/disable adaptive scheduler
    - name: ADAPTIVE_POLLING_BASE_INTERVAL
      value: "10s"             # Target cadence (default: 10s)
    - name: ADAPTIVE_POLLING_MIN_INTERVAL
      value: "5s"              # Fastest cadence (default: 5s)
    - name: ADAPTIVE_POLLING_MAX_INTERVAL
      value: "5m"              # Slowest cadence (default: 5m)
```

**Note:** These settings can also be toggled via **Settings → System → Monitoring** in the UI without pod restart.

Install or upgrade with the overrides:

```bash
helm upgrade --install pulse ./deploy/helm/pulse \
  --namespace pulse \
  --create-namespace \
  -f helm-values.yaml
```

The `server.secretEnv` block above pre-seeds one or more API tokens so the UI is immediately accessible and automation can authenticate. If you prefer to manage credentials separately, set `server.secretEnv.name` to reference an existing secret instead of letting the chart create one.

### Accessing Pulse

- **Port forward:** `kubectl -n pulse port-forward svc/pulse 7655:7655`
- **Ingress:** Enable via the snippet above, or supply your own annotations for external DNS, TLS, etc.
- **LoadBalancer:** Set `service.type: LoadBalancer` and, optionally, `service.loadBalancerIP`.

### Persistence Options

- `persistence.enabled`: Disable to use an ephemeral `emptyDir`
- `persistence.existingClaim`: Bind to a pre-provisioned PVC
- `persistence.storageClass`: Pin to a specific storage class
- `persistence.size`: Resize the default PVC request

## Enabling the Docker Agent

The optional agent reports Docker host metrics back to the Pulse hub. Enable it once you have a valid API token:

```yaml
# agent-values.yaml
agent:
  enabled: true
  env:
    - name: PULSE_URL
      value: https://pulse.example.com
  secretEnv:
    create: true
    data:
      PULSE_TOKEN: docker-agent-token
  dockerSocket:
    enabled: true
    path: /var/run/docker.sock
    hostPathType: Socket
```

Apply with (choose one):

- **Published chart:**

  ```bash
  helm upgrade pulse oci://ghcr.io/rcourtman/pulse-chart \
    --install \
    --version <pulse-version> \
    --namespace pulse \
    -f agent-values.yaml
  ```

- **Local checkout:**

  ```bash
  helm upgrade --install pulse ./deploy/helm/pulse \
    --namespace pulse \
    -f agent-values.yaml
  ```

Notes:

- The agent expects a Docker-compatible runtime and access to the Docker socket. Set `agent.dockerSocket.enabled: false` if you run the agent elsewhere or publish the Docker API securely.
- Use separate API tokens per host; list multiple tokens with `;` or `,` separators in `PULSE_TARGETS` if needed.
- Run the agent as a `DaemonSet` (default) to cover every node, or switch to `agent.kind: Deployment` for a single pod.

## Upgrades and Removal

### Upgrading Pulse

1. **Update the Helm repository:**

   ```bash
   helm repo update pulse
   ```

2. **Check available versions:**

   ```bash
   helm search repo pulse/pulse --versions
   ```

3. **Upgrade to latest:**

   ```bash
   helm upgrade pulse pulse/pulse -n pulse
   ```

4. **Or upgrade to specific version:**

   ```bash
   helm upgrade pulse pulse/pulse --version 4.28.0 -n pulse
   ```

5. **With custom values file:**

   ```bash
   helm upgrade pulse pulse/pulse -n pulse -f custom-values.yaml
   ```

### Rollback

If an upgrade causes issues:

```bash
# List revisions
helm history pulse -n pulse

# Rollback to previous revision
helm rollback pulse -n pulse

# Or rollback to specific revision
helm rollback pulse 3 -n pulse
```

### Uninstall

```bash
# Remove Pulse but keep PVCs
helm uninstall pulse -n pulse

# Remove everything including PVCs
helm uninstall pulse -n pulse
kubectl delete pvc -n pulse -l app.kubernetes.io/name=pulse
```

### Post-Upgrade Verification (v4.25.0+)

After upgrading to v4.25.0 or newer, verify the deployment:

1. **Check update history**
   ```bash
   # Via UI
   # Navigate to Settings → System → Updates

   # Via API
   kubectl -n pulse exec deploy/pulse -- curl -s http://localhost:7655/api/updates/history | jq '.entries[0]'
   ```

2. **Verify scheduler health** (adaptive polling)
   ```bash
   kubectl -n pulse exec deploy/pulse -- curl -s http://localhost:7655/api/monitoring/scheduler/health | jq
   ```

   **Expected response:**
   - `"enabled": true`
   - `queue.depth` reasonable (< instances × 1.5)
   - `deadLetter.count` = 0 or only known issues
   - `instances[]` populated with your nodes

3. **Check pod logs**
   ```bash
   kubectl -n pulse logs deploy/pulse --tail=50
   ```

4. **Verify rollback capability**
   - Pulse v4.24.0+ logs update history
   - Rollback available via **Settings → System → Updates → Restore previous version**
   - Or via API: `POST /api/updates/rollback` (if supported in Kubernetes deployments)

## Service Naming

**Important:** The Helm chart creates a service named `svc/pulse` on port `7655` by default, matching standard Kubernetes naming conventions.

**Service name variations:**
- **Kubernetes/Helm:** `svc/pulse` (Deployment: `pulse`, Service: `pulse`)
- **Systemd installations:** `pulse.service` or `pulse-backend.service` (legacy)
- **Hot-dev scripts:** `pulse-hot-dev` (development only, not used in production clusters)

**To check the active service:**
```bash
# Kubernetes
kubectl -n pulse get svc pulse

# Systemd
systemctl status pulse
```

## Troubleshooting

### Verify Scheduler Health After Rollout

**v4.24.0+** includes adaptive polling. After any Helm upgrade or rollback, verify:

```bash
kubectl -n pulse exec deploy/pulse -- curl -s http://localhost:7655/api/monitoring/scheduler/health | jq
```

**Look for:**
- `enabled: true`
- Queue depth stable
- No stuck circuit breakers
- Empty or stable dead-letter queue

### Port Configuration Changes

Port changes via `service.port` or environment variable `FRONTEND_PORT` take effect immediately but should be documented in change logs. v4.24.0 records restarts and configuration changes in update history.

**To verify port configuration:**
```bash
# Check service
kubectl -n pulse get svc pulse -o jsonpath='{.spec.ports[0].port}'

# Check pod environment
kubectl -n pulse exec deploy/pulse -- env | grep PORT
```

## Reference

- Review every available option in `deploy/helm/pulse/values.yaml`
- Inspect published charts without installing: `helm show values oci://ghcr.io/rcourtman/pulse-chart --version <version>`
- Helm template rendering preview: `helm template pulse ./deploy/helm/pulse -f <values>`
- `NOTES.txt` emitted by Helm summarizes the service endpoint and agent prerequisites after each install or upgrade

## Related Documentation

- [Scheduler Health API](api/SCHEDULER_HEALTH.md) - Monitor adaptive polling status
- [Configuration Guide](CONFIGURATION.md) - System settings and environment variables
- [Adaptive Polling Operations](operations/ADAPTIVE_POLLING_ROLLOUT.md) - Operational procedures
- [Reverse Proxy Setup](REVERSE_PROXY.md) - Configure ingress with rate limit headers
