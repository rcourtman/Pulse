# Pulse Helm Chart

This chart deploys the Pulse hub (web UI + API) and, optionally, the Docker monitoring agent.

## Installing from GHCR

```bash
helm registry login ghcr.io
helm install pulse oci://ghcr.io/rcourtman/pulse-chart \
  --version $(curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/VERSION) \
  --namespace pulse \
  --create-namespace
```

Replace the inline `curl` command if you need to pin to a specific release. The chart version tracks the Pulse release version (e.g., Pulse `v4.24.0` → chart `4.24.0`).

## Common Values

| Key | Description | Default |
| --- | --- | --- |
| `persistence.enabled` | Persist /data using a PVC. Disable for ephemeral testing. | `true` |
| `ingress.enabled` | Create an Ingress resource. Configure hosts/TLS via `ingress.hosts` and `ingress.tls`. | `false` |
| `server.secretEnv` | Manage sensitive env vars (API tokens, auth secrets). Enable `create` or point at an existing secret. | `{}` |
| `agent.enabled` | Deploy the Docker agent as a DaemonSet or Deployment. Requires access to the Docker socket by default. | `false` |

See `values.yaml` for the full list of overridable settings.

## Local Development

Install from the working copy when testing chart changes:

```bash
helm upgrade --install pulse ./deploy/helm/pulse \
  --namespace pulse \
  --create-namespace \
  --set persistence.enabled=false \
  --set server.secretEnv.create=true \
  --set server.secretEnv.data.API_TOKENS=dummy-token
```

For more deployment details (ingress, agent, persistence options), refer to `docs/KUBERNETES.md`.
