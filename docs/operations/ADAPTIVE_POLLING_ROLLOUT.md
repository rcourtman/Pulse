# 🚀 Adaptive Polling Rollout

Safely enable dynamic scheduling.

## 📋 Pre-Flight
1.  **Snapshot Health**:
    ```bash
    curl -s -H "X-API-Token: $TOKEN" http://localhost:7655/api/monitoring/scheduler/health | jq .
    ```
2.  **Check Metrics**: Ensure `pulse_monitor_poll_queue_depth` is stable.

## 🟢 Enable
Choose one method:
- **UI**: Not currently exposed in the UI (use CLI or env vars).
- **CLI**:
  - systemd/LXC: `jq '.adaptivePollingEnabled=true' /etc/pulse/system.json > /tmp/system.json && sudo mv /tmp/system.json /etc/pulse/system.json`
  - Docker/Kubernetes: edit `/data/system.json` in the volume and restart the container/pod
- **Env**: `ADAPTIVE_POLLING_ENABLED=true` (Docker/K8s).

## 🔍 Monitor (First 15m)
Watch for stability:
```bash
watch -n 5 'curl -s http://localhost:9091/metrics | grep pulse_monitor_poll_queue_depth'
```
- **Success**: Queue depth < 50, no permanent errors.
- **Failure**: High queue depth, open breakers.

## ↩️ Rollback
If instability occurs > 10m:
1.  **Disable**: Remove the env var override or set `adaptivePollingEnabled=false` in `system.json`.
2.  **Restart**: Required if using Env/CLI overrides.
3.  **Verify**: Confirm queue drains.
