# Current Branch GA Release Policy

## Direction

The next public Pulse v6 release target is GA from the current
`pulse/v6-release` branch after accumulated post-RC7 fixes.

The published RC7 candidate must not be promoted unchanged. Another RC is not
planned by default; seven RC releases are enough unless the release owner
explicitly changes direction.

## Owner risk acceptance

On 2026-07-02, the release owner explicitly accepted the accumulated post-RC7
changes for the v6.0.0 GA release without requiring RC8, another soak, or
additional current-branch validation before GA.

This is a release-owner risk acceptance, not proof that the post-RC7 branch
state received RC validation. Future stable releases still inherit the normal
promotion policy unless a separate owner decision records a bounded exception.

## Release-control impact

The historical RC7 readiness evidence remains useful as prerelease lineage
evidence, but it no longer proves the final GA candidate by itself because the
current branch intentionally contains additional fixes and changes after RC7.

The GA promotion readiness gate may clear for v6.0.0 only with this owner risk
acceptance recorded alongside the prior release-pipeline rehearsal, rollback
target, v5 maintenance policy, and explicit publication approval.
