# Relay / Pulse Mobile

> **Pulse Mobile is being retired on 31 March 2027.** Paired phones keep working until then. Relay is no longer sold as a plan; existing Relay subscribers have Pro features at their current price. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination under Alerts, and reach the Pulse web UI away from home through your own VPN or tunnel.

Pulse Relay is the end-to-end encrypted connection between a Pulse instance and paired Pulse Mobile devices. It lets the app reach your server without exposing it to the public internet. It does not provide remote access to the Pulse web UI.

> Supported Pulse Mobile clients pair from **Settings → Pulse Mobile** using a QR code or deep link and connect through Pulse Relay over an end-to-end encrypted channel.

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

1. Go to **Settings → Pulse Mobile**.
2. Toggle relay **On**.
3. Use the **QR Code** or **Deep Link** to pair a supported Pulse Mobile client.
4. Your paired mobile client connects through relay.

## Requirements

- **An existing licence with the `relay` entitlement** — Relay is no longer sold.
  Existing Relay subscribers receive Pro features at their current price for as
  long as their subscription continues, including after the app retires.
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
| **License-gated** | Requires an active licence with the `relay` entitlement |
| **Configurable** | Can be enabled/disabled at any time via Settings |
| **Audit** | Pulse Mobile relay access token creation is recorded in the audit trail. Relay connection events are written to the Pulse server log. |

## Configuration

### UI

**Settings → Pulse Mobile** — toggle on/off, view QR code, and manage relay pairing sessions.

### Environment Variables

For headless / container deployments, two env vars override the persisted
`relay.enc` values at load time. Unset leaves the file value untouched.

| Variable | Description | Default |
|---|---|---|
| `PULSE_RELAY_ENABLED` | Enable/disable relay (`true`/`false`/`yes`/`no`/`1`/`0`). Unrecognized values are ignored. | *(unset)* |
| `PULSE_RELAY_SERVER` | Override relay server URL. Must be `ws://` or `wss://`. Invalid values are logged and ignored. | `wss://relay.pulserelay.pro/ws/instance` |

Env vars take precedence over the file at load. Saving from the UI after an
env override is active persists the env-effective state to disk, so clearing
the env var alone will not revert the change — disable in the UI too.

### Storage

Relay configuration is stored encrypted in `relay.enc` in the Pulse data directory.

## API Reference

| Method | Endpoint | Scope | Description |
|---|---|---|---|
| `GET` | `/api/settings/relay` | `settings:read` | Get relay status and config |
| `PUT` | `/api/settings/relay` | `settings:write` | Update relay settings |
| `GET` | `/api/onboarding/qr` | `settings:read` | Generate mobile onboarding QR code |
| `GET` | `/api/onboarding/deep-link` | `settings:read` | Generate mobile deep link |

## Pulse Mobile Pairing

### iOS / Android

1. Pulse Mobile is in early access. Relay and Pro customers get install links from the authenticated [download page](https://pulserelay.pro/download.html).
2. Open Pulse Mobile and tap **Connect to Server**.
3. Scan the QR code from **Settings → Pulse Mobile** in your Pulse web UI.
4. The app connects via the relay for push notifications and secure Open Pulse handoff.

### Multiple Servers

Pulse Mobile can pair with multiple Pulse instances. Each pairing has its own encrypted channel.

## Troubleshooting

### Relay showing "Disconnected"

For an existing paired phone, inspect **Settings → Pulse Mobile** without
changing it: distinguish whether relay is enabled, whether the server's relay
connection is active, and whether the affected phone connects. An active server
connection does not prove phone delivery. Relay is no longer sold; do not buy
or switch plans as a troubleshooting step. If the panel shows a licence error,
keep its exact redacted wording rather than changing the subscription.

Use the original time, Pulse server and app versions, and any existing error
or screenshot. Unknown details are valid. If logs are needed, follow the
[bounded Pulse log readers](TROUBLESHOOTING.md#inspect-notification-logs) on the
Pulse server, then review only relevant lines locally. Do not post full logs,
QR codes, pairing deep links, cookies, tokens or private server addresses.

Keep the existing pairing and configuration. Do not disable/re-enable relay,
delete a pairing, rescan a QR code, restart services or change firewall rules
merely to collect evidence. A public relay health response would not establish
that this server's WebSocket or the phone's encrypted session recovered.

### Pulse Mobile can't connect

Compare the affected phone's existing connection error with the server's
[Relay status](#relay-showing-disconnected). If another already-paired phone
works, retain that distinction; do not pair another device as a test. Preserve
the current pairing and use the existing error or redacted screenshot in the
[same support thread](TROUBLESHOOTING.md#-getting-help).

### Open Pulse handoff not loading

Keep app connectivity separate from access to the web UI: Relay connects the
app, **not the web UI**. Open the Pulse web address in the phone's browser using
your own VPN or tunnel away from home. If the browser works but the app's
handoff does not, report that distinction and the existing app error without
restarting or re-pairing.

For alerts after **31 March 2027**, add [ntfy, Gotify or Pushover](WEBHOOKS.md#-quick-setup)
under **Alerts → Notifications**. These destinations do not require Relay or
a Pulse Mobile pairing. A successful Test is not proof of ordinary alert delivery;
see [notification delivery checks](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing).

## See Also

- [Configuration Guide](CONFIGURATION.md#relay) — environment variables
- [Security](../SECURITY.md#existing-mobile-pairings-retirement) — security for existing paired phones
- [Plans & Entitlements](PULSE_PRO.md) — feature availability by plan
