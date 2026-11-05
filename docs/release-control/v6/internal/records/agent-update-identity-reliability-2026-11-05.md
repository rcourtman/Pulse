# Agent update identity and ARMv6 reliability

Recorded 5 November 2026. The reproduction below records the original update-contract gaps. The source repair and execution evidence are recorded separately below. No release or installed-fleet recovery is claimed.

## Reproduction evidence

The isolated audit used source
`56004ca079014e1d846ad22f39d0f227e727a61a`. Real stripped release-style
`pulse-agent` binaries were passed through the production server validator
and update download handler.

| Advertised version | Actual binary version | Observed server result |
| --- | --- | --- |
| `v6.2.0` | `v6.2.0-rc.8` | Accepted |
| `v6.2.1` | `v6.2.10` | Accepted |
| `v6.4.2` | `v6.2.0` | Accepted through a dependency's version literal |
| Exact matching version | Exact matching version | Accepted control |

`scanUnifiedAgentBinaryContract` searches for a version substring anywhere in
the executable. A dependency's build-info literal can satisfy it. A byte
boundary rule is insufficient: correct version strings in the Go linker's
packed constants can touch unrelated non-boundary bytes. The normal updater
does not verify the downloaded executable's reported version against the
advertised one. A stale prerelease passed self-test, restarted and remained
stale across repeated update rounds.

The ARMv6 experiment cross-compiled the real updater with `GOARM=6` and ran it
under QEMU's `arm1176` and `cortex-a7` CPUs against a loopback server. The
updater requested `linux-armv7` in both cases. On `arm1176`, the downloaded
agent failed the existing self-test with an illegal instruction before
replacement. The existing binary remained, leaving a repeat-download failure
loop rather than a bricked installation. On `cortex-a7`, the ARMv6 build was
replaced by an ARMv7 binary. Emulation is not physical-device qualification.

## Canonical repair boundaries

A shared release-target authority should derive the executable's published
target from Go build metadata, including `GOARM`, and refuse missing or
unsupported combinations rather than guessing. Both the server and updater
need the exact executable package and target. The updater should check bounded
`--version` output against the advertised version after the existing download,
format, checksum and required-signature checks, before self-test or activation.

The server also needs a bounded, explicitly delimited release identity stamped
by the canonical agent build recipe. It must accept one distinct valid identity
value and refuse conflicting values. Repeated identical bytes can occur in
non-trimpath linker metadata. The Makefile and runtime-attestation builders
must use the same contract as published release builds. Real stripped ELF, PE
and Mach-O controls and rejection cases are required alongside unit tests.
Normal, Windows rename and privileged activation paths must all refuse a
mismatched executable before replacement, restart or saved-state mutation.

Already installed ARMv6 clients retain their old selection logic. Repair needs
an installer rerun using the existing saved-state update path and the correct
architecture. A shared token or matching hostname is insufficient evidence for
a server-side CPU bridge. Do not infer installed-client identity from either.

The Pro server and dormant Docker updater have related ARM mappings. They are
separate consumers requiring ownership and reachability review, not established
end-to-end failures in this experiment.

An isolated 26-file source candidate was reported at base
`bdfe94d70995af316de84b3c308f6acafea63d17`, with patch SHA-256
`efdab79f67bd0dd3cecde33c2d44415b3089e5fcc40c34902413e8fbcc46f7ac`.
Its complete source export was unavailable and a text-export request was
rejected by the service safeguard. No further export attempt was made. The
candidate was not independently reviewed, imported, committed or qualified.
That unavailable candidate remains unqualified historical evidence. The source repair below was independently implemented from the current canonical source.


## Source repair and execution evidence

`internal/agentidentity` now owns compiler-derived executable targets and a
bounded, delimited release stamp. Server admission requires the canonical main
package, a supported target and one distinct valid release identity, including
exact prerelease equality. Repeated identical stamps remain valid for ordinary
non-trimpath linker metadata. Dependency literals and version prefixes cannot
satisfy the release contract. Static package/stamp bytes describe the signed
artifact contract; they do not authenticate source code or symbol origin.

The updater checks format, checksum and required signatures before static
identity admission and separate bounded `--version` and `--release-identity`
probes. The latter reads the actual canonical stamp variable and refuses an
unstamped executable. These checks precede self-test, replacement, Windows
rename, privileged staging/activation, restart and continuity-state writes.
Privileged quarantine and cleanup remain allowed before admission. Unsupported
or missing compiler target metadata fails closed. The caller's admitted target is verified before typed-helper staging or activation.

Local server downloads validate, hash and serve one open descriptor. Cached
artifact facts include inode, size, modification/change time, expected version and
target. An atomic publication cannot substitute different pathname bytes into
an admitted response or reuse a previous inode's cache entry. Deterministic
proxied release-identity failures are logged and return HTTP 409.

The canonical release recipe stamps the collector, developer builds consume
the canonical published target matrix and runtime attestation checks the same
flags. The universal agent launcher now dispatches ARMv6 and 386 assets as well
as the existing Linux targets. Pro and the dormant Docker selector consume the
same compiler target authority. The published unified agent disables the old
Docker self-update path, so this does not establish an old Docker fleet journey.

Retained worker proofs cover real stripped ELF, PE and Mach-O fixtures; actual
canonical `cmd/pulse-agent` builds in those formats; dependency-literal, stale
prerelease, longer-version, wrong-target and conflicting-stamp refusals; bounded
runtime output; normal and simulated Windows refusal before mutation; and
privileged quarantine-only refusal. The canonical native executable prints the
actual release identity. Tests exercise descriptor/cached-inode replacement and
the adjacent executable continuity-file boundary.

The real GOARM=6 updater and predecessor/successor-version ARM agents built
from the repaired source run under QEMU
`arm1176` and `cortex-a7`. Both select `linux-armv6`, execute the admitted ARMv6
version/identity/self-test probes, activate it and leave a current successor
without another artifact download. A supplied ARMv7 candidate is rejected
without executing it, replacing the predecessor, restarting or creating
continuity state. This is emulation, not physical ARM qualification.

The complete installer ARMv6 saved-state preflight requests the ARMv6 artifact
and leaves saved credentials, identity and modes unchanged. A separate native
test executes the canonical download, signature verification, staged URL
admission, upgrade detection and replacement blocks in private directories
against a signed actual agent, preserving the saved token, identity and
connection file. Host service-stop effects are recorded locally rather than
applied to the worker. Existing state-directory lifecycle integration covers
subsequent runtime enrollment/report continuity. This composes evidence for
the documented local repair path; it does not claim an entire installed ARM
device or native Windows service upgrade.

The original broad serial API proof reached its 20-minute budget and is not a
pass. Focused identity/download tests and the isolated mock-mode test passed;
no test-stabilization change was included. Exact-commit source CI remains a
landing requirement. The release gate `unified-agent-v5-upgrade-continuity`
remains open for real release-asset and installed-client qualification. RA9
contains the reproducible source proof commands and evidence. No lane or
release gate is closed by this source repair.


The full external execution proof is reproducible with
`bash scripts/verify-agent-executable-identity.sh` on a Linux amd64 validation
worker with QEMU ARM available on PATH (or supplied using `--qemu-arm`). The
driver builds the staged Git tree into its own private source packet, prepares
only a test API embed fixture there, creates the canonical cross-format and ARM
artifacts, and requires actual execution of the opt-in tests. It does not use
release credentials, install a service or publish artifacts. Its raw output,
source tree identity and artifact hashes remain in the reported private output
directory. RA9 includes this full proof command alongside the ordinary source
tests. The source repair narrows the existing planning gap to installed ARMv6
fleet recovery and physical ARMv6/ARMv7 qualification. Protected-branch CI must
pass before this change reaches main. The installed-client release qualification
gate remains open.

Native CI watches the shared identity authority and build recipes. Development
deployment also uses the canonical target matrix and stamp rather than placing
unstamped binaries in served paths. Runtime mismatch controls pass static
admission before executing a different well-formed stamp; download diagnostics
retain both architecture-specific refusal and fallback failure.


The release assembler now verifies the final collector bytes after compiled
payload and platform-native replacements, using `cmd/verify-agent-release` and
the same shared identity authority for every published target. Windows signing
refuses a failed or empty metadata recipe before compilation. This closes an
artifact-production gap rather than relying on client refusal after publication.
The proof driver runs the verifier against actual canonical ELF, PE, Mach-O and
ARM artifacts and requires refusal of a copied ARMv7 artifact labelled ARMv6.

Recorded linker flags carrying a marker must identify the canonical stamp
symbol. An actual non-trimpath release build remains admitted; an actual wrong-
symbol control is refused statically and by the runtime identity probe. Toy
fixtures explicitly clear inherited Go flags and assert repeated metadata is
present. The updater validates advertised versions before artifact transfer or
helper quarantine, and the release proxy refuses an invalid expected version.
The focused RA9 command includes the bounded-copy, cache-ctime, diagnostic and
related Pro-target controls as well as the full external driver. Both universal
server and collector wrappers cover the published Linux matrix. Development
deployment diagnostics preserve failed-host results and continue to later hosts.

The Docker context includes the shared grammar. Production macOS and Windows
signing builders use the canonical recipe. Native installed-profile fixtures
derive versions from their executables; rootful qualification uses installer
updates. Existing systemd-lab examples also build collectors with the canonical
recipe. These source checks do not establish installed ARM fleet recovery,
physical ARM timing, native Windows replacement or broker delivery observation.
