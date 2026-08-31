# Unified Agent Privilege-Boundary Plan

Last updated: 2026-08-30
Status: PROPOSED
Governance surfaces:
- `status.json.coverage_gaps.agent-privilege-boundary-separation`
- `status.json.candidate_lanes.secure-agent-runtime-separation`

## Intent

Pulse should keep broad host visibility without asking operators to trust one
network-connected, root-running process with monitoring, update, and arbitrary
command authority.

The target is a split runtime:

1. API-only monitoring remains the recommended path where a platform API can
   provide the required inventory and health signals.
2. The Unified Agent collector runs as an unprivileged service account by
   default on supported general-purpose Linux hosts.
3. Narrow privileged reads and update activation cross a local typed helper
   boundary instead of widening the collector process.
4. Remediation is a separately installed, separately credentialed runtime with
   explicit action policy. It is never implied by installing monitoring.

This is a trust-boundary change, not an installer-default cleanup. It crosses
agent lifecycle, token authority, installer ownership, platform visibility,
action governance, updates, UI guidance, and migration compatibility. It
therefore needs its own governed lane rather than an untracked expansion of
the complete agent-lifecycle lane.

## Product Sentence

Pulse monitors with the least authority the selected data source requires,
isolates exceptional host privileges behind typed local operations, and grants
remediation authority only through a separate operator-enabled runtime and
credential.

## Current Baseline

The current implementation has useful hardening, but it does not yet establish
that product sentence:

- `scripts/install.sh` defaults the Unified Agent service to `root`; the
  optional `--least-privilege` profile is not the generated setup default.
- A Proxmox systemd unit can receive ambient `CAP_SETUID` and `CAP_SETGID` even
  when commands are not enabled, because later remote command enablement is
  anticipated by the unit.
- The least-privilege installer adds `pulse-agent` to the rootful Docker group
  when present. Access to a rootful Docker socket is root-equivalent authority,
  even if Pulse currently intends only read operations.
- The least-privilege service account owns the installed agent binary so the
  in-process updater can replace it. A compromised collector can therefore
  persist by replacing its own executable.
- SMART and `pct` access use sudo wrappers that forward caller-controlled
  arguments to privileged binaries. The wrapper and sudoers entry do not form
  a typed operation boundary.
- `internal/api/agenttokens/install.go` gives Proxmox install credentials
  `agent:exec` unconditionally, while host install credentials grant it only
  when commands are enabled.
- `internal/hostagent/commands.go` carries typed operations and arbitrary shell
  execution in the same command channel; `read_file` also reaches a command
  execution path, and trusted internal requests can bypass human approval.
- Setup copy correctly recommends API inventory where possible, but the
  agent-mode path describes a supported root service rather than offering a
  safe default privilege profile.

Existing hardening such as a dedicated listener, token binding, command policy,
systemd sandboxing, scoped API tokens, and the optional least-privilege profile
is retained as useful input. None of it substitutes for separating monitoring
from root-equivalent and remediation authority.

## Current Implementation Checkpoint (2026-08-30)

The optional Linux systemd path now establishes more of the intended boundary,
without changing the product default:

- a configured helper must answer the exact versioned health protocol before
  collector readiness, and failed helper SMART/Proxmox reads do not fall back
  into local privileged collection;
- helper-backed collector activation is pending under a bounded deadline until
  the replacement is locally ready and a newly collected authoritative report
  is accepted; explicit commit, restart/power-loss recovery, watchdog rollback,
  and fixed staging/quarantine cleanup preserve the last-known-good binary;
  the helper independently executes `--version` from root-owned staging,
  rejects a signed artifact that is not the requested advancing collector
  version, and binds commit to the same process after it execs the active
  digest;
- runner enrollment persists the canonical hostname, durable credential
  rotation invalidates only the superseded live session after storage commits,
  and uninstall performs an exact bearer self-revoke without exposing the
  secret in argv;
- safe migration rejects effective unit overrides, requires a fresh post-start
  registration timestamp and live helper protocol response, irreversibly
  removes `agent:exec` and cross-host `agent:manage` from the host-bound
  collector credential before local
  privilege changes, restores only installer-owned state-root metadata and
  Proxmox markers on rollback, and disables rootful Docker unless a usable
  collector-owned rootless socket exists. Explicit or automatic rollback may
  restore the old root service identity, but it removes `--enable-commands`
  and cannot resurrect the reduced server-side authority;
- the safe-profile declaration is bound to effective systemd state, not unit
  file contents or socket health alone. The collector, helper service, helper
  socket, and installed action runner must have the installer-owned
  `FragmentPath`, no `DropInPaths`, and the expected effective executable,
  identity, environment-file, address-family, network, and filesystem
  hardening properties. The typed helper is additionally bounded to 64 tasks,
  256 file descriptors, and 256 MiB of memory, and those effective limits are
  override-tested. Existing overrides fail closed;
- the root action runner is networked and host-mutating by design. Its unit
  retains kernel/home/control-plane hardening but does not set the host
  filesystem read-only, because apt and Proxmox operations require their
  documented host writes.

This is a qualification foundation, not the Slice E ratchet. The generated
setup default remains the legacy/root profile. The earlier clean disposable
arm64 Ubuntu systemd exercise remains recorded in
`internal/records/secure-agent-runtime-systemd-receipt-2026-08-30.json` and its
committed-main attestation, but it predates the hardening above: its action
scenario proved a pre-mutation refusal and receipt replay, not a successful
host mutation; its rollback claim predates irreversible credential reduction;
and its secret-free receipt is hash-bound but not independently authenticated.
It is historical evidence, not qualification of the current implementation.

The v3 guarded lab requires a real verified apt-cache mutation, a separate
stale-fingerprint refusal, durable receipt replay, an observed collector
authority-reduction request (with persistence covered by API regressions),
the then-required fixed boundary-source hashes, and artifact Go build stamps for the exact
clean commit. A fresh Ubuntu 24.04.4/systemd 255 run at committed main
`b2543c5c6e03bc8f38502098d3a356983d0d41b2` passed all twelve scenarios; its
separate receipt and attestation are recorded as
`internal/records/secure-agent-runtime-systemd-receipt-v3-2026-08-30.json` and
`internal/records/secure-agent-runtime-committed-main-attestation-v3-2026-08-30.json`.
That schema-v3 receipt is now historical input only. The post-audit attester
requires schema v4 and will not reinterpret v3 as proof of the current tree.
V4 expands a committed source manifest across the production collector,
helper, runner, host-agent provider, API admission, update, configuration, and
receipt roots; requires the receipt hashes to match that exact set; validates
ordered timestamps and scenario-specific causal claims; binds every scenario
to a retained secret-free JSONL transcript event; validates typed receipt kind
and report chronology; and requires the intended repository record path to be
inside the hashed receipt.

A fresh Ubuntu 24.04.4/systemd 255 arm64 run at committed main
`defc24af837b91428fbee939d09cd31e9559fb4f` passed all twelve schema-v4
scenarios in 107 seconds. The 345-source manifest matched the commit, all four
artifacts carried its clean Go VCS identity, and the receipt bound 81 retained
transcript events, including 69 raw command-output events. The receipt,
transcript, and attestation are recorded as
`internal/records/secure-agent-runtime-systemd-receipt-v4-2026-08-30.json`,
`internal/records/secure-agent-runtime-systemd-transcript-v4-2026-08-30.jsonl`,
and
`internal/records/secure-agent-runtime-committed-main-attestation-v4-2026-08-30.json`.
This remains artifact-bound, secret-free, self-attested systemd fixture
evidence rather than an independently authenticated assessment or proof of the
production Router/TLS/durable-store credential lifecycle. The focused
`TestActionRunnerRotationProductionRouterTLSPersistenceRestart` regression now
exercises the real Router over HTTPS and WSS with encrypted token persistence:
failed rotation persistence leaves the active secret and socket intact;
replacement admission closes the exact predecessor socket; successful
activation durably replaces its secret; restart rejects the predecessor; and
self-revoke survives a second restart. That code-level transport proof is not
part of the systemd receipt or an exact release-candidate exercise. The
committed schema-v5 contract added the three helper-update recovery scenarios
and remains immutable at its original ordered fifteen-scenario definition.
Schema v6 is a separate twenty-scenario contract. It adds effective helper
service, helper resource-limit, helper socket, and runner override rejection
plus the host-canary helper-network-namespace exercise, and binds those claims to
`secure_runtime_source_manifest_v6.json`. It does not reinterpret schema-v4 or
schema-v5 evidence.

Schema v7 is a separate twenty-three-scenario contract and leaves every prior
receipt schema immutable. It adds live rootful Docker summary-inventory
migration, helper restart continuity, and helper-loss/recovery scenarios. The
release workflow creates its daemon inside the disposable systemd host, seeds
an offline source-bound fixture image, exposes only the root-owned Unix socket,
and never mounts the hosted runner's Docker socket. V7 narrows only the
rootful-Docker summary-inventory residual; Podman, rootless-runtime parity,
container metrics, image-update checks, typed container actions, appliance
coverage, and external review remain open. No schema-v7 receipt exists until
an exact immutable prerelease packet successfully completes that workflow, so
this contract does not change the product default or upgrade the current v6
evidence classification.

A fresh Ubuntu 24.04.4/systemd 255 arm64 run at committed main
`22fd662fb794f63efb9d3ca2158de73c4e07e1b8` passed all twenty schema-v6
scenarios in 248.13 seconds. The 437-source manifest matched the qualified
commit, all six artifacts carried its clean Go VCS identity, and the receipt
bound 117 retained transcript events, including 97 command-output events. It
exercised the helper update authoritative-report commit, watchdog rollback,
helper-restart recovery, effective collector/helper/socket/runner override
rejection, bounded helper resources, host-interface network isolation, typed
apt-cache mutation and replay, exact runner rotation, and self-revocation. The
receipt, transcript, and attestation are recorded as
`internal/records/secure-agent-runtime-systemd-receipt-v6-2026-08-31.json`,
`internal/records/secure-agent-runtime-systemd-transcript-v6-2026-08-31.jsonl`,
and
`internal/records/secure-agent-runtime-committed-main-attestation-v6-2026-08-31.json`.
Their SHA-256 digests are
`498f14580b0c63a1d9e24ddd44dd32dfad96024a187330e9ede4c777cd5ab123`,
`29b51dbe7d393523e3b9f69284ec8b07b1ca6930e94822ce220fbd59fbcaa1e2`, and
`bbeb11b72f983953392ddea282c0ede336f4739156d35b72188de3f07b12cae2`.
The committed-main attester bound the canonical Pulse origin, a fresh remote
`origin/main` value, exact ancestry, the artifact bytes, source hashes, receipt,
and transcript. It remains self-attested fixture evidence rather than trusted
RC compilation provenance or an external review, and it makes no default
change.

Schema-v6 and schema-v7 release-candidate classification does not trust a local ref or Go
VCS stamp. Pulse release tags are workflow-created annotated tags rather than
signed tags, so tag presence alone is not authority. RC classification
requires the exact canonical remote tag object and peeled commit, the immutable
GitHub Release and release attestation, release-attested `checksums.txt`, the
portable hosted-assembly provenance from `build-release-candidate.yml`, a
separate provenance bundle that binds every qualification binary to
`compile-release-payload.yml` with self-hosted runners denied, and a signed
secure-runtime build contract. That contract must bind every qualification
artifact to the checksums and record both workflow identities, a
GitHub-hosted-only compiler policy, the source commit, Go toolchain, package,
target, `CGO_ENABLED`, `-trimpath`, `-buildvcs=false`, exact ldflags, version,
and production update-key fingerprint. Hosted assembly of a payload emitted by
a self-hosted compiler is not trusted compilation provenance. The canonical
compiler workflow now has a separate GitHub-hosted qualification job that
builds and attests the six Linux amd64 subjects without signing secrets. Hosted
candidate assembly requires its current collector, helper, and runner to match
the ordinary release payload byte for byte, then publishes the three predecessor
collectors, compiler provenance, and build contract through the normal signed
release packet. A published prerelease automatically runs the twenty-three-scenario schema-v7
lab on a disposable systemd host. Before any candidate byte can reach that
privileged container, the workflow copies all six binaries, four collector
signatures, checksum and provenance sidecars, and the build contract into a
private snapshot and verifies the immutable release, canonical tag and source,
hosted compiler chain, exact digests, and production update key against those
copies. Only the verified snapshot becomes executable and is mounted read-only.
failed or mutated packets never reach Docker. The post-run attester consumes
the same snapshot paths and all four release signatures. No qualifying RC
containing this wiring has been published yet, so current evidence remains
committed-main, artifact-bound, and self-attested; a local tag still cannot
upgrade it to RC status.

Schema-v6 committed-main classification is likewise not caller-relative. The
attester accepts only `origin/main`, requires the canonical Pulse origin URL,
and compares the local remote-tracking commit with a fresh `refs/heads/main`
query before proving ancestry. `HEAD`, a local branch, or an arbitrary ref can
never receive the committed-main label.

The repository still needs a fresh exact committed release-candidate run,
representative Proxmox, SMART, Docker and rootless Podman telemetry/action
parity, appliance profiles, and the external security review. Until those
proofs are recorded, the safe profile remains opt-in and provider degradation
remains an explicit residual rather than evidence that the candidate lane is
complete.

## Target Architecture

### 1. API-Only Monitoring

API-backed platform integrations remain first choice when they meet the
support floor. Installing an agent must not be presented as a security upgrade
over an adequate API connection.

API credentials stay scoped to collection. They do not grant host command or
remediation authority.

### 2. Unprivileged Collector

The default supported Linux collector runs as `pulse-agent` and owns only its
mutable state, logs, and credential files. Its executable and unit definition
remain root-owned and not writable by the service account.

The collector may:

- gather unprivileged OS telemetry
- connect outbound to the Pulse agent listener
- receive its credential through a narrowly permissioned service mechanism
- request a versioned local privileged operation
- report capability loss explicitly when a privileged provider is unavailable

The collector may not:

- hold `agent:exec`
- join a root-equivalent daemon group
- hold ambient privilege-escalation capabilities
- invoke arbitrary sudo commands
- replace its installed executable directly
- accept arbitrary remote shell or file-read requests

### 3. Typed Privileged Helper

A small root-owned helper provides only the privileged operations that have a
documented monitoring or update need. It has no Pulse credential and runs in a
private network namespace restricted to `AF_UNIX`. The preferred transport is
a root-owned local Unix socket with peer-credential validation and a versioned
request/response schema. A qualification receipt may claim only the exercised
network boundary: a host-reachable non-loopback TCP canary was unreachable
from the live helper namespace. Broader platform-wide non-connectivity remains
unqualified until exercised on that platform.

Initial operation families are expected to be:

- SMART device discovery and bounded health reads
- Proxmox guest/container inventory needed for host-level visibility
- bounded Docker/Podman inventory through the daemon without granting the
  collector direct socket access
- activation and rollback of an already downloaded, signature-verified agent
  update

Each operation must define allowed fields, validation, time limit, output
limit, stable error classes, audit metadata, and platform support. The helper
must reject unknown operation versions and arbitrary command arguments. It
must not expose a generic exec, shell, file-read, daemon-proxy, or path-read
primitive.

The helper is not automatically installed merely because the collector is
installed. The setup flow selects required operation bundles from the
operator's enabled telemetry providers and shows the resulting authority.

### 4. Separate Action Runner

Remediation moves out of the collector process into a separate service and
install choice. The action runner has its own host-bound credential and only
that credential may carry `agent:exec`.

The runner receives typed, versioned actions after the server-side action
governance path has completed authorization, approval, lease, freshness, and
audit admission. It returns durable receipts using the existing idempotency
model.

Arbitrary shell and unrestricted file reads are not part of the safe target
architecture. If compatibility requires a transition period, they remain only
in an explicitly named full-trust legacy profile with separate disclosure,
credentialing, install action, and deprecation tracking. They must not be
described as least privilege.

### 5. Appliance Profiles

TrueNAS, Synology, QNAP, Unraid, FreeBSD, and other appliance or non-systemd
platforms do not silently inherit the Linux claim. Each platform keeps an
explicit capability profile until its own service-manager, filesystem, update,
and privileged-read boundaries are proven.

An unsupported safe profile fails closed with clear setup guidance. It does
not silently fall back to root.

## Security Invariants

These are completion conditions, not implementation preferences:

1. A monitoring-only install credential never includes `agent:exec`.
2. Remote configuration cannot promote a monitoring collector into a command
   runtime or cause new ambient capabilities to appear.
3. The collector service account cannot mutate its executable, unit, helper,
   or root-owned update staging metadata.
4. The collector has no direct rootful Docker socket access and is not a member
   of a root-equivalent runtime group.
5. The privileged helper has no network credential, outbound network path, or
   generic execution primitive.
6. Helper requests authenticate the local caller, validate a versioned typed
   operation, and enforce time and output bounds before returning data.
7. A compromised collector can request only the same bounded operations needed
   for configured telemetry; it cannot convert them into arbitrary host writes
   or command execution.
8. Update activation verifies signed artifact identity and target paths before
   an atomic root-owned swap, preserves a last-known-good binary, and produces
   a durable result.
9. Installing or enabling remediation is a separate operator action with a
   separate service, credential, status, and uninstall path.
10. A trusted server-side origin does not turn an untyped command string into
    an approved action.
11. Doctor and fleet status report effective privilege profile, helper
    operation availability, action-runner state, credential authority, and
    degraded telemetry separately.
12. Setup and migration never claim a safe profile when the running process or
    effective groups/capabilities contradict it.

## Execution Slices

The slices are ordered to reduce current authority before the full helper and
runner split lands. Each slice must remain releasable and must not claim the
final architecture early.

### Slice A: Immediate Authority Containment

Deliver:

- remove unconditional `agent:exec` from Proxmox monitoring install tokens
- make command authority explicit at every install-token call site
- stop provisioning ambient `CAP_SETUID`/`CAP_SETGID` for monitoring-only
  agents in anticipation of future remote configuration
- prevent remote settings from promoting a monitoring-only runtime into a
  command-capable runtime
- expose monitoring versus remediation authority in generated setup guidance
  and Doctor output
- label current root and optional least-privilege modes accurately; do not
  overstate either as the completed split architecture

Required proof:

- token tests show monitoring credentials reject `agent:exec`
- installer tests show command-disabled units receive no command-only ambient
  capabilities
- remote-config tests show authority cannot widen without reinstalling or
  explicitly enrolling the action runtime
- upgrade tests preserve existing installations without silently changing
  telemetry or command behavior

### Slice B: Safe Collector Profile and Migration

Deliver:

- make the unprivileged collector the default on supported Linux systemd hosts
- keep the binary, service unit, and helper root-owned
- keep mutable state and credentials narrowly accessible to `pulse-agent`
- remove automatic Docker-group membership
- generate install commands with a typed privilege profile and required
  telemetry bundles
- provide an inspect-only migration command that reports expected telemetry
  changes before applying them
- provide explicit apply and rollback operations; never silently convert an
  existing root install during ordinary update

Required proof:

- fresh-install tests verify user, groups, ownership, modes, capabilities, and
  unit sandboxing
- upgrade tests verify existing root installs remain stable until an operator
  selects migration
- migration rehearsal verifies preflight, apply, rollback, credential
  preservation, and honest degraded capability reporting
- setup UI and generated commands describe unsupported appliance profiles
  without a root fallback masquerading as success

### Slice C: Typed Privileged Helper

Deliver:

- add the local protocol, root-owned socket/unit, peer authentication, request
  validation, audit fields, timeouts, and output bounds
- implement SMART and Proxmox operation bundles
- implement bounded container-runtime inventory without exposing the daemon
  socket to the collector
- move signed update activation and rollback behind a root-owned operation
- remove sudo argument-forwarding wrappers and their grants after parity is
  proven

Required proof:

- protocol conformance tests cover every accepted and rejected operation
  version and field
- adversarial tests reject path traversal, argument injection, unknown ops,
  oversized output, timeout abuse, symlink swaps, and unauthorized peers
- process/network proof first reaches a non-loopback host-interface TCP canary
  from the host namespace, then enters the live helper process's network
  namespace and proves that the same connection is denied. This is evidence
  for the exercised helper namespace and endpoint, not a universal claim about
  every Linux distribution, firewall, address family, or future systemd build
- telemetry parity tests compare the safe profile with the current root
  baseline and classify every intentional difference
- update tests prove signature identity, atomic activation, restart, health
  verification, and rollback while the collector cannot write its binary

### Slice D: Separate Action Runner

Deliver:

- create a separate action-runner install, service, credential, registration,
  health, and uninstall lifecycle
- route typed host, Proxmox, and container remediation through that runtime
- bind action-runner sessions to organization, canonical host identity, token,
  and action capability
- preserve server-side approval, policy lease, audit, cancellation, and durable
  terminal receipts
- remove arbitrary shell and unrestricted file read from the normal collector
  protocol
- bound any necessary legacy full-trust compatibility with explicit status and
  removal criteria

Required proof:

- monitoring-only hosts cannot open action sessions or receive actions
- collector credentials are rejected by the action listener and action
  credentials are rejected for collector report/config paths unless explicitly
  granted
- typed action schema, policy admission, target binding, replay protection,
  cancellation, and receipts pass adversarial tests
- disabling or uninstalling the runner leaves monitoring intact
- no trusted-origin flag bypasses typed action validation or live policy
  admission

### Slice E: Qualification and Default Ratchet

Deliver:

- publish and keep current the per-platform privilege and telemetry matrix in
  `docs/AGENT_SECURITY.md`; every unqualified row names its owner, visible
  fail-closed behavior, and removal condition
- run migration rehearsals on representative Proxmox, Linux, Docker/Podman,
  SMART, and update environments
- complete an external security review focused on the local helper protocol,
  update boundary, action credential separation, and migration
- make the setup UI default to the safe supported profile only after proof
  meets the matrix
- retain a clearly named legacy/full-trust path only for platforms or features
  with an owned residual and a removal condition

Required proof:

- fresh installs and migrations produce the declared profile on live hosts
- the capability matrix matches observed telemetry and action behavior
- failures degrade visibly and never fall back to broader privilege
- support and troubleshooting material can distinguish collector, helper, and
  action-runner failures without asking operators to run the whole agent as
  root

## File-Level Change Map

Expected primary surfaces:

- `scripts/install.sh`: profile selection, service ownership, helper/runner
  units, migration, rollback, and uninstall
- `scripts/installtests/`: install, upgrade, migration, ownership, capability,
  and appliance-profile proof
- `internal/api/agenttokens/install.go`: monitoring and action credential scope
  separation
- `internal/api/configapi/install_command.go`: typed privilege and action
  profile generation
- `internal/hostagent/`: collector-only transport, capability reporting, and
  removal of command authority
- a new narrowly owned helper package/command: typed local privileged protocol
  and implementations
- a new action-runner package/command: remediation transport and typed action
  execution
- `internal/agentexec/` and action APIs: runner-specific admission, target
  binding, health, and receipts
- Infrastructure and Agent Doctor frontend surfaces: effective authority,
  capability differences, migration preflight, and separate remediation
  enrollment
- `docs/AGENT_SECURITY.md`, `docs/PRODUCTION_SECURITY.md`, install docs, and
  troubleshooting docs: supported privilege claims and migration guidance

Before implementation, every path must be resolved through
`subsystem_lookup.py`; the owning subsystem contracts and registry entries must
be updated in the same slice when the canonical ownership boundary moves.

## Migration and Compatibility

Existing installations are not silently switched from root to the safe
profile during an update. The migration flow is:

1. inspect the running unit, user, groups, capabilities, binary ownership,
   enabled providers, command state, and platform support
2. calculate the target collector/helper/runner profile and list telemetry or
   action differences
3. require an explicit operator apply action
4. stage root-owned units and binaries, preserve credentials and identity, and
   perform a health check before declaring success
5. roll back service definitions and binaries if the new profile cannot reach
   its declared health floor
6. retain the previous profile label and reason in Doctor/fleet history

Current command-enabled installations map to two choices: monitoring-only safe
migration with remediation disabled, or safe collector plus explicit action
runner enrollment. They never receive action authority merely because the old
combined process had it.

## Rollback Boundaries

- Collector rollback restores the prior signed binary and unit while retaining
  agent identity and report credential, but it removes the legacy command flag.
  The pre-migration credential reduction is deliberately irreversible, so
  rollback cannot restore `agent:exec` or remote command authority.
- Helper rollback disables its socket and restores the previous declared
  telemetry profile; it does not add the collector to privileged groups.
- Action-runner rollback disables and revokes its credential without disabling
  monitoring.
- A platform that cannot meet the safe profile remains on an explicitly named
  legacy/full-trust profile until a platform-specific plan lands. Rollback
  never silently broadens privilege.

## Explicit Exclusions

This plan does not:

- promise a single non-root implementation for every appliance platform
- make API-only integrations depend on an agent
- treat rootful Docker-group membership as least privilege
- build a generic privileged daemon proxy
- preserve arbitrary remote shell as part of the safe architecture
- redesign the full server-side action-approval model except where runtime and
  credential separation require contract changes
- change the current release gate merely because the proposal is recorded

## Candidate-Lane Completion Definition

The proposed secure-agent-runtime-separation lane can close only when:

1. supported Linux monitoring installs default to the unprivileged collector
2. monitoring credentials and processes have no remediation authority
3. privileged reads and update activation use the typed local helper with the
   security invariants and adversarial proof above
4. root-equivalent Docker access, ambient command capabilities, writable agent
   binaries, and sudo argument-forwarding wrappers are absent from the safe
   profile
5. remediation runs only through the separately installed and credentialed
   action runner
6. migration, rollback, Doctor, fleet, setup UI, and documentation expose the
   effective profile accurately
7. platform exceptions are explicit residuals with owners and removal criteria
8. subsystem contracts, registry ownership, tests, and live proof agree with
   the shipped architecture

Until those conditions are met, Pulse may describe the current optional
least-privilege profile and hardening controls factually, but must not claim
that monitoring, host privilege, and remediation are fully separated.
