# ☸️ Kubernetes (Helm)

Deploy Pulse to Kubernetes using the official Helm chart.

## 🚀 Installation

1. **Install (OCI chart, recommended)**
   ```bash
   helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart \
     --namespace pulse \
     --create-namespace
   ```

2. **Access**
   ```bash
   kubectl -n pulse port-forward svc/pulse 7655:7655
   ```
   Open `http://localhost:7655` to complete setup.

> If you installed using a Helm repository URL previously, you can keep using it. OCI is the preferred distribution format going forward.

---

## ⚙️ Configuration

Configure via `values.yaml` or `--set` flags.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type (ClusterIP/LoadBalancer) | `ClusterIP` |
| `ingress.enabled` | Enable Ingress | `false` |
| `persistence.enabled` | Enable PVC for /data | `true` |
| `persistence.size` | PVC Size | `8Gi` |
| `agent.enabled` | Enable legacy docker agent workload | `false` |

> Note: the `agent.*` block is legacy and currently references `pulse-docker-agent`. For new deployments, prefer the unified agent (`pulse-agent`) where possible.

### Example `values.yaml`

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: pulse.example.com
      paths:
        - path: /
          pathType: Prefix

server:
  env:
    - name: TZ
      value: Europe/London
  secretEnv:
    create: true
    data:
      API_TOKENS: "my-token"

agent:
  enabled: false
  secretEnv:
    create: true
    data:
      PULSE_TOKEN: "my-token"
```

Apply with:
```bash
helm upgrade --install pulse pulse/pulse -n pulse -f values.yaml
```

---

## 🔄 Upgrades

```bash
helm upgrade pulse oci://ghcr.io/rcourtman/pulse-chart -n pulse
```

**Rollback**:
```bash
helm rollback pulse -n pulse
```

---

## ⚠️ Troubleshooting

- **Check Pods**: `kubectl -n pulse get pods`
- **Check Logs**: `kubectl -n pulse logs deploy/pulse`
- **Scheduler Health**:
  ```bash
  kubectl -n pulse exec deploy/pulse -- curl -s http://localhost:7655/api/monitoring/scheduler/health
  ```
