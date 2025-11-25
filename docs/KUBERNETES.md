# ☸️ Kubernetes (Helm)

Deploy Pulse to Kubernetes using the official Helm chart.

## 🚀 Installation

1. **Add Repo**
   ```bash
   helm repo add pulse https://rcourtman.github.io/Pulse/
   helm repo update
   ```

2. **Install**
   ```bash
   helm install pulse pulse/pulse \
     --namespace pulse \
     --create-namespace
   ```

3. **Access**
   ```bash
   kubectl -n pulse port-forward svc/pulse 7655:7655
   ```
   Open `http://localhost:7655` to complete setup.

---

## ⚙️ Configuration

Configure via `values.yaml` or `--set` flags.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type (ClusterIP/LoadBalancer) | `ClusterIP` |
| `ingress.enabled` | Enable Ingress | `false` |
| `persistence.enabled` | Enable PVC for /data | `true` |
| `persistence.size` | PVC Size | `8Gi` |
| `agent.enabled` | Enable Docker Agent sidecar | `false` |

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
  enabled: true
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
helm repo update
helm upgrade pulse pulse/pulse -n pulse
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
