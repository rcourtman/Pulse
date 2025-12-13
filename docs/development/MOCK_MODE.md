# 🧪 Mock Mode Development

Develop Pulse without real infrastructure using the mock data pipeline.

## 🚀 Quick Start

```bash
# Start dev stack
./scripts/hot-dev.sh

# Toggle mock mode
npm run mock:on     # Enable
npm run mock:off    # Disable
npm run mock:status # Check status
```

## ⚙️ Configuration
Edit `mock.env` (or `mock.env.local` for overrides):

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PULSE_MOCK_MODE` | `false` | Enable mock mode. |
| `PULSE_MOCK_NODES` | `7` | Number of synthetic nodes. |
| `PULSE_MOCK_VMS_PER_NODE` | `5` | VMs per node. |
| `PULSE_MOCK_LXCS_PER_NODE` | `8` | Containers per node. |
| `PULSE_MOCK_RANDOM_METRICS` | `true` | Jitter metrics. |
| `PULSE_MOCK_STOPPED_PERCENT` | `20` | % of offline guests. |
| `PULSE_MOCK_TRENDS_SEED_DURATION` | `1h` | Pre-seed backend chart history (improves demo “Trends” immediately). |
| `PULSE_MOCK_TRENDS_SAMPLE_INTERVAL` | `30s` | Backend chart sampling interval while in mock mode. |

## ℹ️ How it Works
*   **Data**: Swaps `PULSE_DATA_DIR` to `/opt/pulse/tmp/mock-data`.
*   **Restart**: Backend restarts automatically; Frontend hot-reloads.
*   **Reset**: To regenerate data, delete `/opt/pulse/tmp/mock-data` and toggle mock mode on.

## ⚠️ Limitations
*   **Happy Path**: Focuses on standard flows; use real infrastructure for complex edge cases.
*   **Webhooks**: Synthetic payloads only.
*   **Encryption**: Uses local crypto stack (not a sandbox for auth).
