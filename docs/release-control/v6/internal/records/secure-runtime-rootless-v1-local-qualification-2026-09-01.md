# Secure rootless runtime v1 local qualification (2026-09-01)

## Classification

The standalone `secure-runtime-rootless-v1` packet passed from exact source
commit `60041ad9e60c282c892f944e04f777b874991a5d`. The independent validator
classified it as
`local-opt-in-rootless-runtime-artifact-bound-self-attestation` and verified
all 418 governed source hashes plus the exact collector, helper, installer, and
qualification-test artifact bindings.

The secret-free receipt has SHA-256
`7a116d63ab0cd1560482165055f0a8c9158ce0a8333a27bd9e8256582a52dfb5`.
The separate attestation has SHA-256
`566279ecd7d7dfa89c92f24243e9fcd5ae3e295d4284bd559d4be42f1e83b3b5`.
Its source manifest has SHA-256
`cae1c8c53ff7019ad0fa82c39e90635ed0290a97a2055c3978005cbda1bca497`.
The full receipt and attestation remain retained local evidence rather than
published release assets; this record intentionally stores only their
secret-free identifiers and qualification scope.

## Exercised runtime matrix

Two distinct disposable Ubuntu 24.04/systemd 255 arm64 hosts completed the
canonical eleven scenarios for each runtime, for twenty-two passing scenario
records in total:

1. `fresh_install`
2. `legacy_migration`
3. `collector_restart`
4. `daemon_restart`
5. `socket_loss_helper_fallback`
6. `direct_recovery`
7. `dual_socket_ambiguity_refusal`
8. `exact_pin_recovery`
9. `telemetry_parity`
10. `authority_isolation`
11. `cleanup`

The Docker host reported machine identity
`9050518de741748dc8bf3a1d04861e58`, Docker 29.7.2, rootless daemon identity
`1fb49913-d54b-41bc-864f-42e1e33f88ab`, and collector-owned socket
`/run/user/996/docker.sock` with mode `0660`. The Podman host reported machine
identity `c063c32387eafbfd6abec6e939256b59`, Podman 4.9.3, rootless daemon
identity `ecec4fdfedced3b59b47048cb1be54c14901d3f41b0aee6e28f6bc84446c6631`,
and collector-owned socket `/run/user/996/podman/podman.sock` with mode `0600`.
The distinct machine and daemon identities prevent one shared fixture from
standing in for both runtime families.

The packet proved fresh safe-profile installation, legacy authority-reducing
migration, collector and daemon restart continuity, same-family rootful typed-
helper summary fallback during rootless socket loss, direct recovery without
collector restart, fail-closed dual-socket ambiguity handling, root-owned exact
pin recovery, and direct telemetry parity against a root client of the same
rootless daemon. Direct telemetry included complete inventory, stats, full
fields, and secondary inventory. Authority isolation proved that the non-root
collector had no command session or command transport, no rootful socket
access, and no container action or update authority. Strict teardown removed
the fixture containers, stopped both runtimes, removed their sockets and
delegated state, and left no labeled qualification containers behind.

## Boundaries and remaining work

This packet closes the local live-proof residual for the exact exercised
rootless Docker and Podman monitoring, migration, fallback, recovery,
ambiguity, parity, authority, and cleanup paths. It does not extend or
reinterpret the schema-v7 systemd contract.

The attestation records four explicit limitations:

- `not-published-release-provenance`
- `not-default-profile-authorization`
- `not-independent-security-review`
- `production-exact-scope-proof-is-external-prior`

It does not qualify rootless container image-update checks or typed container
actions, which remain disabled in the safe collector. It does not qualify the
complete standalone rootful Docker/Podman lifecycle matrix, Proxmox or SMART
provider parity, appliance profiles, or an exact published release candidate.
The safe profile therefore remains explicit opt-in, and this record does not
complete or accept the proposed secure-agent-runtime-separation lane.
