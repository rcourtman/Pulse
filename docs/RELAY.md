# Relay / Pulse Mobile Handoff (Relay and Above)

Pulse Relay provides **end-to-end encrypted remote access** foundations for Pulse instances. It allows secure remote connectivity without exposing your Pulse server to the public internet.

> Supported Pulse Mobile clients pair from **Settings → Relay** using a QR code or deep link and connect through Pulse Relay over end-to-end encrypted remote access.

## How It Works

```text
┌──────────┐         ┌──────────────┐         ┌──────────┐
│  Pulse    │◄──E2E──►│  Relay       │◄──WSS──►│  Pulse   │
│  Mobile   │  ECDH   │  Server      │         │  Server  │
└──────────┘         └──────────────┘         └──────────┘
```

1. Your Pulse server maintains a persistent WebSocket connection to the relay server.
2. A mobile client connects to the relay server and authenticates.
3. An ECDH key exchange creates a per-channel encryption key.
4. Tunneled remote-access traffic is encrypted end-to-end — the relay server **never sees plaintext data**.

## Quick Start

1. Go to **Settings → Relay**.
2. Toggle relay **On**.
3. Use the **QR Code** or **Deep Link** to pair a supported Pulse Mobile client.
4. Your paired mobile client connects through relay.

## Requirements

- **Relay, Pro, legacy Pro+, or Cloud license** — relay is gated by the `relay` feature key.
- **Outbound WebSocket** — Pulse must be able to reach `relay.pulserelay.pro` (port 443).
- **No inbound ports** — you do not need to open any ports on your firewall.

## Security

Relay was designed with a zero-trust model:

| Property | Detail |
|---|---|
| **Encryption** | End-to-end ECDH key exchange per channel |
| **Plaintext** | Relay server never sees your monitoring data |
| **Authentication** | Per-session mobile authentication |
| **Back-pressure** | Data limiters prevent channel flooding |
| **License-gated** | Requires an active Relay-or-higher license |
| **Configurable** | Can be enabled/disabled at any time via Settings |
| **Audit** | Relay connection events are logged to the audit trail |

## Configuration

### UI

**Settings → Relay** — toggle on/off, view QR code, and manage relay pairing sessions.

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `PULSE_RELAY_ENABLED` | Enable/disable relay | `false` |
| `PULSE_RELAY_SERVER` | Override relay server URL | `relay.pulserelay.pro` |

### Storage

Relay configuration is stored encrypted in `relay.enc` in the Pulse data directory.

## API Reference

| Method | Endpoint | Scope | Description |
|---|---|---|---|
| `GET` | `/api/settings/relay` | `settings:read` | Get relay status and config |
| `PUT` | `/api/settings/relay` | `settings:write` | Update relay settings |
| `POST` | `/api/onboarding/qr` | `settings:read` | Generate mobile onboarding QR code |
| `POST` | `/api/onboarding/deep-link` | `settings:read` | Generate mobile deep link |

## Pulse Mobile Pairing

### iOS / Android

1. Join mobile early access when available.
2. Open Pulse Mobile and tap **Connect to Server**.
3. Scan the QR code from **Settings → Relay** in your Pulse web UI.
4. The app connects via the relay for push notifications and secure Open Pulse handoff.

### Multiple Servers

Pulse Mobile can pair with multiple Pulse instances. Each pairing has its own encrypted channel.

## Troubleshooting

### Relay showing "Disconnected"

1. Confirm your Relay, Pro, grandfathered Pro+, or Cloud license is active (**Settings → Plans**).
2. Verify the Pulse server can reach the relay server:
   ```bash
   curl -s https://relay.pulserelay.pro/healthz
   ```
3. Check Pulse logs for relay errors:
   ```bash
   journalctl -u pulse | grep -i relay
   # or
   docker logs pulse | grep -i relay
   ```

### Pulse Mobile can't connect

1. Verify relay is enabled in **Settings → Relay**.
2. Confirm your mobile account has beta access.
3. Re-scan the QR code — sessions can expire.
4. Ensure your mobile device has internet access.

### Open Pulse handoff not loading

1. Check the relay connection status in **Settings → Relay**.
2. Look for WebSocket reconnection messages in Pulse logs.
3. Restart Pulse Mobile.

## See Also

- [Configuration Guide](CONFIGURATION.md#relay) — environment variables
- [Security](../SECURITY.md#relay-security-pro) — relay security details
- [Plans & Entitlements](PULSE_PRO.md) — feature availability by plan
