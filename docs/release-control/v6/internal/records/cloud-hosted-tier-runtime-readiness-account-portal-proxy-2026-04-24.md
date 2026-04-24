# Cloud Hosted Tier Runtime Readiness - Account Portal Proxy

Date: 2026-04-24
Evidence tier: real-external-e2e
Owner: cloud-hosted-tier-runtime-readiness

## Scope

The public Pulse Account handoff at `https://cloud.pulserelay.pro/portal` was
failing from the workstation and Playwright audit with `ERR_CONNECTION_RESET`.
The reset happened before the application received the request.

## Finding

Host-local checks on `pulse-cloud` proved Traefik and the control-plane were
healthy:

- `curl --resolve cloud.pulserelay.pro:443:127.0.0.1 https://cloud.pulserelay.pro/healthz`
  returned `200`.
- `docker exec pulse-cloud-traefik-1 wget -S -O- http://control-plane:8443/healthz`
  returned `200`.
- `tcpdump` on `pulse-cloud` showed the external workstation path receiving
  SYNs and sending SYN-ACKs, but the app path never received the completed
  TLS handshake.

This put the defect below the Pulse application and below Traefik routing.

## Change

`cloud.pulserelay.pro` was switched from DNS-only to Cloudflare-proxied in the
`pulserelay.pro` zone. `*.cloud.pulserelay.pro` remains DNS-only so hosted
tenant subdomains keep the existing direct origin TLS behavior.

The DNS posture is recorded in `repos/pulse-pro/OPERATIONS.md` and in the
local capability registry. The wildcard tenant record must not be proxied
without first proving certificate coverage for tenant hosts.

## Proof

After local DNS cache settled:

- `curl -4fsS https://cloud.pulserelay.pro/healthz` returned `ok`.
- `curl -4fsS 'https://cloud.pulserelay.pro/portal?service=manage&email=buyer%40example.com'`
  returned the Pulse Account portal HTML.
- `PULSE_PUBLIC_RELEASE_TRACK=v5 ./scripts/audit_public_release.sh` in
  `repos/pulse-pro` passed without
  `PULSE_PUBLIC_AUDIT_SKIP_PULSE_ACCOUNT_FETCH`.

The public landing audit still reported `track: v5` and `approval: 0`; no v6
release flip, release tag, or GitHub Release was created.
