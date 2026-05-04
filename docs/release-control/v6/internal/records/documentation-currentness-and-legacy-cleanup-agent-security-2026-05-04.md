# Documentation Currentness Agent Security Follow-Up

- Date: `2026-05-04`
- Lane: `L9`
- Related discussion: `#1453`

## Context

Discussion `#1453` relayed a Proxmox community question about whether Pulse
agents should run as `root`, whether API-only monitoring is enough for
read-only Proxmox use cases, and what supply-chain risk exists when running an
installer as `root`. The existing agent security guide already documented the
Linux/systemd root default and checksum/signature verification, but it did not
clearly separate Proxmox API-only monitoring from host/guest agent installs.

## Outcome

The active public docs now make the Proxmox deployment choice explicit:

- start with Proxmox API-only monitoring when inventory, node status,
  VM/container status, storage usage, and normal API metrics are enough;
- install agents only where Pulse needs data that Proxmox cannot provide
  through the API, such as inside-guest Docker/Podman visibility, host SMART
  and temperature data, local ZFS/Ceph/mdadm detail, arbitrary mount reads, or
  Kubernetes node/pod reporting;
- treat custom non-root systemd service users as local hardening profiles, not
  supported full-telemetry mode;
- describe the agent-local health/metrics listener and the `--health-addr`
  options for loopback binding or disabling;
- keep release-pinned, signature-verified server installer guidance visible
  alongside agent checksum/signature update verification.

This keeps the project response in durable official guidance rather than a
social-thread reply.
