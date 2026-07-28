# Pulse v6.2.0-rc.3 Release Notes

`v6.2.0-rc.3` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.2`. This focused support
candidate corrects the remaining multi-organization Proxmox connection
collision reported against the first two candidates.

## Highlights

- Proxmox clusters in different organizations stay separate even when they
  reuse the same internal Proxmox cluster name, node names, and private IP
  addresses.
- Captured TLS certificate fingerprints now govern the add-connection path,
  using the same identity decision as configuration consolidation.

## Fixed

- The Proxmox add-connection handler no longer merges a newly discovered
  cluster into an existing connection solely because their internal cluster
  names match.
- A contradictory TLS fingerprint vetoes the merge before either connection
  can be removed from saved configuration.
- Legitimate duplicate connections to the same physical cluster still merge
  when their cluster name, endpoint evidence, and TLS identity agree.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.3` only when you are
comfortable testing an RC. The rollback target is `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

After updating, a cluster that returned to Network Discovery can be connected
again. Existing healthy connections do not need to be removed first.

This server candidate has no mobile compatibility change and does not require a
companion build upload. No public mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
