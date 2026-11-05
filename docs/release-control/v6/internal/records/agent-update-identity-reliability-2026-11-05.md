# Agent update identity and ARMv6 reliability

Recorded 5 November 2026. These are reproduced, unresolved update-contract
gaps. No agent-update implementation is qualified by this record, and no
release or installed-fleet recovery is claimed.

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
The reproduction and design therefore remain evidence for a future owned
implementation, not delivered runtime behavior.
