# Pulse v6 RC-to-GA Rehearsal Record

Use this template to capture the human-side evidence for
`rc-to-ga-promotion-readiness` after running the `Release Dry Run` workflow.

The matching GitHub Actions artifact should be the machine-generated
`rc-to-ga-rehearsal-summary`.

## Required Fields

1. Rehearsal date:
2. GitHub Actions run URL:
3. Version under rehearsal:
4. Candidate stable tag:
5. Promoted RC tag:
6. Current rollback target:
7. Exact rollback or reinstall command:
8. RC soak hours at rehearsal time:
9. v5 end-of-support date to publish with GA:
10. Result:

## Minimum Human Notes

1. Confirm the rehearsed branch matched the intended release line.
2. Confirm the release path was exercised end to end up to, but not including,
   publication.
3. Confirm no manual input was surprising or ambiguous during the run.
4. Record any follow-up issue that must be fixed before the real promotion.
5. Link this record from the release ticket before clearing the gate.
