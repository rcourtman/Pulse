# Pulse Intelligence RG-01 through RG-12 external E2E record

Date: 2026-07-12

## Verdict

The closed RG-01 through RG-12 matrix reached independent GO for bounded limited autonomy at these exact implementation revisions:

- Pulse core: `a63b3eae2b5a62ee6803bfb4eee8fadbbba8e449`
- Pulse Mobile: `a1ddb451618664025d22a986fbd3fd837e4ffe97`

The proof set covers exact-revision unit and integration checks, Docker and Debian/Ubuntu Colima journeys, a clean Product Trust browser exercise, and physical-iPad behavior against the live Relay service. The physical exercise covered fresh pairing, notification registration, encrypted reconnect, canonical inert action detail, approval and denial, exact-token revocation, fail-closed reconnect behavior, and full temporary-state cleanup.

## Sealed evidence

- Physical iPad/live Relay artifact: `/Volumes/Development/.pulse-proof-artifacts/rg12-a63b3eae-a1ddb451-final-20260712T234230Z`
- Artifact `SHA256SUMS` SHA-256: `58778244cb370be4bf7bd74a8316a5bf4e8ec4dca23f08e45c5bc8cfec152397`
- Aggregate evidence: `/Volumes/Development/.pulse-proof-artifacts/pulse-intelligence-evidence-a63b3eae2b5a62ee6803bfb4eee8fadbbba8e449-final.json`
- Aggregate evidence SHA-256: `9992148c61420e527dab47fc33a49aecb4d757fe2030a40bff6656328566fa37`

The sealed artifact manifest verifies every included file. The aggregate evaluator binds the records to the audited core revision, requires the physical artifact descriptor, and validates its manifest, implementation revisions, inert action semantics, revocation barrier, cleanup state, physical test summaries, screenshots, and underlying result-bundle digests.

## Trust boundary

The approved and denied actions used canonical `target_type=test` and `executor=none`; neither action executed. The final action state had zero pending actions, dispatch attempts, outbox entries, and receipts. After the exact Relay token was revoked, protected requests returned HTTP 401 and the negative barrier remained canonical triple-zero: zero persistence writes, zero dispatch attempts, and zero external mutations.

Temporary Relay instance, device-token, connection-log, local action, audit, attempt, outbox, receipt, and token state were removed. The original Relay instance remained present. The iPad finished unpaired at onboarding, the environment was restored, and Colima was stopped.

Agent-executed RG06 and RG09 product outcomes remain `fix_verification_unknown`; independent Colima-control observations do not upgrade product verification. This GO certifies only the proved bounded capability set. It does not certify arbitrary infrastructure mutation, general MSP-scale autonomy, agent-native discovery, or operable onboarding and import planning.
