# Pulse v6 RC-to-GA Rehearsal Record

Use this template to capture the human-side evidence for
`rc-to-ga-promotion-readiness` after running the `Release Dry Run` workflow.

Prefer generating the record with
`python3 scripts/release_control/record_rc_to_ga_rehearsal.py ...` rather than
hand-writing it.

The matching GitHub Actions artifact should be the machine-generated
`rc-to-ga-rehearsal-summary`.

Treat that artifact as the machine-owned source for the candidate stable tag,
promotion channel, promoted RC tag, rollback target, exact rollback command,
and planned GA/EOS dates. Only override those values in the human record if
the operator is correcting a verified artifact mismatch.

The generator should fail closed if the artifact omits any of that promotion
metadata. Do not repair a malformed artifact by hand-writing the missing
candidate tag, promotion channel, promoted RC tag, rollback target, rollback
command, or GA/EOS dates into the record.

## Required Fields

1. Rehearsal date:
2. GitHub Actions run URL:
3. Version under rehearsal:
4. Candidate stable tag:
5. Promotion channel:
6. Promoted RC tag:
7. Current rollback target:
8. Exact rollback or reinstall command:
9. RC soak hours at rehearsal time:
10. Exact GA date to publish with GA:
11. v5 end-of-support date to publish with GA:
12. Result:

## Minimum Human Notes

1. Confirm the rehearsed branch matched the governed release line from `control_plane.json` (currently `pulse/v6-release`).
2. Confirm the release path was exercised end to end up to, but not including,
   publication.
3. Confirm no manual input was surprising or ambiguous during the run.
4. Confirm the artifact-owned candidate stable tag, promotion channel,
   promoted RC tag, rollback target, exact rollback command, planned GA date,
   and planned v5 end-of-support date match the intended publish notice.
5. Record any follow-up issue that must be fixed before the real promotion.
6. Link this record and the matching artifact from the release ticket before
   clearing the gate.
