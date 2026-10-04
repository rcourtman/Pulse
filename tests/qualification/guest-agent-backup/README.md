# Guest-agent backup observation — #2439

This is a **measurement tool**, not a new release gate or a backup hook. It
does not start Pulse, a backup, PVE or QGA. It never sends a guest-agent command,
freezes/thaws a filesystem, changes ACLs, disables freezing or restarts/resets
a guest. Local tests and an OK backup task do not establish native acceptance.

The reported job is PVE **9.2.21**, **HAOS with QGA**, and Pulse **6.4.5** using
the same effective **Audit + FileRead** access: a backup task reported OK while
the guest remained frozen. Pulse's containing-source mitigation starts at
`3c57c2d20728f9e711ae2930e282b853eb218730` (with its ancestors). Source controls
cover admission, lock checks, in-flight payload withdrawal, no replay and cache
truth. They cannot atomically reserve the native PVE/QGA channel or prove thaw.

## Smallest useful native route

Delivery owns provisioning and native execution. Core supplies the independent
guest writer and record checks here. The existing private CI VM can build these
tools and judge returned records; guest sudo does not supply a PVE hypervisor,
HAOS, QGA's real channel or permission to reach an existing host. An ordinary
QGA subprocess, simulated PVE API or observation guest with PVE/QGA disabled
cannot meet this job. Do not build a replacement platform just to call it native.

The missing resource is **one owned disposable PVE target with a real HAOS/QGA
guest and backup storage**, isolated from production, plus verified official
image/package inputs and a supported way for its owner to observe it. Use an
already provisioned suitable target if available; otherwise Delivery returns
the exact provisioning/input gap through its normal reviewed infrastructure
route. This file is not permission to use a host, account or credential.

Before native execution the owner retains, privately:

- The exact inspected Pulse binary digest, source/tree, required guard ancestry,
  build/artifact provenance and process identity. Use a containing reviewed
  build, not a version string or an unverified rebuild. Preserve 6.4.5 as the
  reported source, not proof of the candidate. No new-source acceptance follows
  from old app/source receipts.
- Actual PVE manager/kernel/package identities (reconcile what the reported
  `9.2.21` identifies), HAOS image and QGA executable/version/configuration.
  Record filesystem-freeze enablement and every writable filesystem covered by
  the backup. Map opaque filesystem IDs to these mounts; exclude tmpfs and
  container overlays that are not actually frozen. Stage the writer through
  the owner's supported guest-console/image route **before** the backup.
- Effective privileges of the *same* Pulse token: Audit and FileRead on the
  target VM, QGA enabled, ordinary TLS verification, one Pulse process, no other
  manual QGA probes. A refusal stops the attempt; do not substitute root or
  broaden access. Never record the credential, full config or service environment.
- Preselected observation window (at most ten minutes), backup task identity,
  Pulse polling/cache intervals, native command-trace coverage and guest/host
  clock calibration with at most one second relative error. Hash/link original
  provenance, clock/mount crosswalk and raw task/command traces separately.

Run a normal freeze-enabled backup with Pulse stopped first as the fixture
control, observing the guest independently. Then, on a fresh equivalent
disposable fixture, observe normal containing-build polling across a backup.
For in-flight acceptance, require a **genuinely observed** Pulse read spanning
lock acquisition, not a sleep or a claimed overlap. Never force overlap on a
customer guest or queue manual QGA probes. If no overlap occurs within the
chosen window, retain that inconclusive attempt; it is not an acceptance pass.
No repeated run is prescribed merely to obtain a favourable result.

## Independent guest filesystem writer

Build with the declared Go compiler; stage the resulting Linux binary for the
actual guest architecture. Python is **not** required in HAOS:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/guest-writer \
  ./tests/qualification/guest-agent-backup/guest-writer
```

The native owner runs one writer on each mapped, freeze-covered filesystem:

```sh
guest-writer --directory /owner-provisioned/scratch-on-covered-mount \
  --run-id run-01 --filesystem-id data --duration-seconds 600 --interval-ms 1000
```

The directory must exist, be absolute and have no symlink components. Each run
creates a new private child; repeated IDs refuse reuse. Each write uses an
exclusive new 0600 file, writes 512 synthetic bytes, fsyncs both file and
directory, reads back and checks the bytes, then emits their SHA-256. At most
600 such files are left per filesystem; it deletes nothing. Only opaque IDs,
fixed outcomes and timestamps reach stdout. Collect stdout/exit through an
**independent console channel**, not QGA or the filesystem being frozen.

One write can be in flight. Heartbeats continue while it is blocked; no further
write is queued. Interruption, output failure, readback/write error or a write
pending at the deadline cannot pass. The program does not unblock a frozen
filesystem. Preserve failed evidence and leave disposal/recovery to the fixture
owner; do not turn the failure into a guest restart/reset workaround.

A successful write witnesses only that mapped filesystem at that instant, not
all filesystems, application integrity or a successful backup restore. Require
at least two completed baseline writes, liveness during the frozen interval and
two **newly started** fsync/readback writes after task completion on every mapped
filesystem. A pre-freeze write that finally returns is insufficient on its own.

## Bounded record checks

`check.py` reads a redacted JSON record and adjacent hash-bound writer JSONL
files. It has no network, subprocess or guest-control capability. The shape is
exercised by `record()` and `witness_events()` in `test_check.py`; those are
**synthetic checker fixtures, never native evidence**. Fields are exact and
extra fields (including private names/raw logs) are rejected.

The native owner normalises its actual observations into that shape, retaining
underlying receipts separately. `identity` binds Pulse/PVE/QGA/HAOS digests;
`preflight` and its mount-crosswalk hash cover all declared filesystem IDs.
`backup` contains native start, lock, freeze, thaw-*request*, unlock and completed
times. A thaw request is not a thaw result. `coverage` binds the complete
bounded native command trace with zero dropped events. `commands` are **actual
dispatched** reads, not planned or deferred calls; preserve adverse outcomes.
The checker requires real in-flight overlap, no concurrent/duplicate dispatch
and no dispatch inside the observed lock/clock-uncertainty interval.

`snapshots` are baseline, at least two locked polls and resumed readbacks of the
same VM. Retain metadata/memory/disk payload hashes, guest-agent state, memory
source, disk reason, latest memory/disk History times and CPU observation time.
Obtain them from the existing read-state, memory diagnostics and History APIs;
normalise only the target guest's fixed fields, without raw snapshots in public
evidence. Locked values/History cannot be renewed as fresh; CPU must progress.
Resumption needs available/agent memory, no disk deferral and fresh post-task
memory/disk History. The complete command trace must also contain successful
`get-fsinfo` and `file-read` reads newly started after task completion and its
clock-uncertainty interval, and completed by the resumed readback within the
recorded clock allowance. Fresh History timestamps, independent filesystem
writes, the pre-lock overlap or metadata-only reads cannot substitute for those
post-task guest reads. Later polling cannot support an earlier resumed snapshot.
This is a consistency check: independently reconcile the raw command trace and
returned payloads with the target guest and History observations before judging
resumption. Do not send extra guest probes to make a record pass; observe normal
polling and retain an incomplete window as inconclusive.
**Internal cache timestamp preservation is source-tested;
these public readbacks do not directly inspect those internal timestamps.**

```sh
python3 tests/qualification/guest-agent-backup/check.py \
  --record /private/new-native-record/record.json --expected-source EXACT_40_HEX_SHA
```

Files are bounded, hash-bound, single-name regular files; symlinks, traversal,
duplicate JSON keys, wrong identities, truncation, impossible order, replay,
incomplete coverage and adverse witness exits fail closed. Evidence-class and
provenance fields are owner observations, **not access grants or independent
artifact verification by the checker**. Exit 0 means the supplied record's
checks passed. Output deliberately retains `native_acceptance_complete: false`:
review original native provenance, mount completeness and command/task traces
before judging the job. It does not establish the reported command-ID cause,
fleet-wide safety, shipment, restore integrity or release qualification.

## Source checks and next result

```sh
python3 -m unittest discover -s tests/qualification/guest-agent-backup -v
go test -race ./tests/qualification/guest-agent-backup/guest-writer -count=1
```

Use the assigned source-proof VM for full source suites, builds and the Go
writer checks. The focused Python record/CLI controls use only synthetic local
files and can run locally without a native target or listener. They do not
validate the runtime lock guard or restore an interrupted executor attempt.
The real local writer smoke is ordinary synthetic fsync/readback, **not** a filesystem-freeze test.
Do not replay unchanged guard/app suites to disguise the missing native target.

Delivery's next material result is verified native inputs and a completed (or
adverse/inconclusive) bounded run: independent writes/liveness, actual command
overlap, truthful lock-time readings/History and normal cache resumption on the
exact containing build. Coordinator/reviewer/publisher own protected landing;
Delivery/steward own its containing all-main release. Community owns useful
reporter contact after material evidence. Until native acceptance, preserve
stop-Pulse-before-backup and restart-only-after-verified-thaw advice, explicitly
including the monitoring/alert outage. An OK task or cleared lock is not enough.
