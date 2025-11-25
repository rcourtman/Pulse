# 🚀 Adaptive Polling Rollout

Safely enable dynamic scheduling (v4.24.0+).

## 📋 Pre-Flight
1.  **Snapshot Health**:
    ```bash
    curl -s http://localhost:7655/api/monitoring/scheduler/health | jq .
    ```
2.  **Check Metrics**: Ensure `pulse_monitor_poll_queue_depth` is stable.

## 🟢 Enable
Choose one method:
*   **UI**: Settings → System → Monitoring → Adaptive Polling.
*   **CLI**: `jq '.AdaptivePollingEnabled=true' /var/lib/pulse/system.json > tmp && mv tmp system.json`
*   **Env**: `ADAPTIVE_POLLING_ENABLED=true` (Docker/K8s).

## 🔍 Monitor (First 15m)
Watch for stability:
```bash
watch -n 5 'curl -s http://localhost:9091/metrics | grep pulse_monitor_poll_queue_depth'
```
*   **Success**: Queue depth < 50, no permanent errors.
*   **Failure**: High queue depth, open breakers.

## ↩️ Rollback
If instability occurs > 10m:
1.  **Disable**: Toggle off via UI or Env.
2.  **Restart**: Required if using Env/CLI overrides.
3.  **Verify**: Confirm queue drains.
