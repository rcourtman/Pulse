# Pulse v6.2.2-rc.2 Release Notes

`v6.2.2-rc.2` is a release candidate for the next Pulse v6 patch. It follows
stable `v6.2.1` and supersedes `v6.2.2-rc.1`. This candidate retains the
security, monitoring-scale, operator-control, agent, and update reliability
work from the first candidate while fixing private-registry update checks and
SMART collection with smartmontools 7.5.

## Highlights

- Private-registry update checks now use the Docker or Podman login already on
  the agent host.
- smartmontools 7.5 power-mode objects no longer discard SMART data from
  guarded rotational-disk probes.
- All security, monitoring, Proxmox scale, Relay, alerting, and qualification
  improvements from the first candidate remain included.

## Added

- Host-local Docker and Podman registry credential discovery for update
  checks, including `auths`, credential helpers, global credential stores, and
  Podman `auth.json` sources.
- An explicit `PULSE_DISABLE_REGISTRY_CREDENTIALS=true` environment variable
  and `--disable-registry-credentials` agent flag for anonymous-only registry
  checks.

## Improved

- Registry authentication supports Basic challenges, authenticated Bearer
  token negotiation, and refresh-token grants used by identity-token logins.
- Credential lookups are cached in memory for five minutes; stale credentials
  fall back to the existing anonymous path without breaking public-registry
  checks.
- SMART standby detection recognizes the additional EPC mode names emitted by
  current smartmontools releases.

## Fixed

- Private-registry containers no longer remain stuck on an "authentication
  required" update badge when the agent host already has a working Docker or
  Podman login (#1706).
- smartmontools 7.5 `power_mode` objects no longer invalidate the complete JSON
  document for guarded rotational-disk probes, which previously caused usable
  SMART data to disappear on affected hosts (#1690).

## Security

- Registry credentials remain on the monitored host and are presented only to
  the registry or its advertised token endpoint. Credential-helper output is
  excluded from reported errors, and helper names are validated before
  execution.
- All security hardening shipped in `v6.2.2-rc.1` remains part of this
  candidate, including fail-closed configuration transfer, proxy-role
  authorization, and framed audit signatures.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.2-rc.2` only when you are
comfortable testing a release candidate. The rollback target is `v6.2.1`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.2.1
```

Private-registry update checks use credentials available to the account that
runs Pulse Agent. The supported systemd installer runs the agent as root, so
use `sudo docker login <registry>` on that host. Set
`PULSE_DISABLE_REGISTRY_CREDENTIALS=true` when anonymous-only checking is
required.

The changes since `v6.2.2-rc.1` do not alter mobile, Relay, onboarding, or
mobile-facing API contracts. No companion mobile build upload or public
mobile-store rollout is part of this candidate.

Windows Unified Agent binaries in this prerelease retain exact-SHA, checksum,
and detached-signature verification but are not Authenticode-signed, so Windows
may display an Unknown Publisher warning. Stable `v6.2.2` still requires the
normal SignPath Authenticode lane unless a separate version-bound owner decision
is recorded.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
