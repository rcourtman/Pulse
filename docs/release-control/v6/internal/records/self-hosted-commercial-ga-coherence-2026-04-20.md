# Self-Hosted Commercial GA Coherence Record

- Date: `2026-04-20`
- Gate: `self-hosted-commercial-ga-coherence`
- Assertion: `RA8`
- Result: `pass`

## Automated Baseline

- `python3 scripts/release_control/documentation_currentness_test.py`
- `python3 scripts/release_control/release_promotion_policy_test.py`
- `cd /Volumes/Development/pulse/repos/pulse/frontend-modern && npm test -- src/components/Settings/__tests__/ProLicensePanel.test.tsx`
- `cd /Volumes/Development/pulse/repos/pulse-pro/license-server && go test ./...`
- Result: pass

## Public v6 Preview Rehearsal

- Environment:
  - externally reachable public v6 preview:
    `https://v6-commercial-preview.pulserelay-landing.pages.dev`
  - live license server:
    `https://license.pulserelay.pro`
- Command:
  - `PULSE_PUBLIC_SITE_URL=https://v6-commercial-preview.pulserelay-landing.pages.dev PULSE_PUBLIC_RELEASE_TRACK=v6 PULSE_V6_RELEASE_APPROVED=1 PULSE_PUBLIC_AUDIT_SKIP_PULSE_ACCOUNT_FETCH=1 bash /Volumes/Development/pulse/repos/pulse-pro/scripts/audit_public_release.sh`
- Result: pass
- Verified:
  1. The public preview renders the governed v6 Community / Relay / Pro ladder.
  2. The preview checkout buttons resolve against the live license pricing contract.
  3. The utility pages keep the canonical Pulse Account handoff targets.
  4. The browser audit stayed on v6 pricing and did not fall back to legacy v5 public framing.

## In-App Self-Hosted Upgrade Rehearsal

- Environment:
  - managed enterprise local backend started by
    `tests/integration/scripts/managed-local-backend.mjs`
  - exercised base URL:
    `http://127.0.0.1:62336`
  - runtime state file:
    `tmp/ga-commercial-preview-runtime-state.json`
- Browser observations:
  1. `/settings/system/billing/plan` renders the default self-hosted billing posture with
     `Compare self-hosted plans`.
  2. The default CTA resolves to
     `/auth/license-purchase-start?feature=self_hosted_plan`.
  3. Rehearsed navigation from that CTA produced the real handoff chain:
     - `GET http://127.0.0.1:62336/auth/license-purchase-start?feature=self_hosted_plan`
     - `303` redirect to
       `https://cloud.pulserelay.pro/portal?portal_handoff_id=cph_851a9101e81f70d224e5e966fff86234`
  4. The app-side route no longer falls back to the local `Pulse Account unavailable` dead end.

## Pulse Account Reachability Check

- Environment:
  - verified from `pulse-cloud` because this workstation still shows the known intermittent TLS reset when direct local Playwright or curl follows `cloud.pulserelay.pro`
- Commands:
  - `ssh root@pulse-cloud "curl -sS -D - -o /tmp/pulse-portal.out 'https://cloud.pulserelay.pro/portal?portal_handoff_id=cph_851a9101e81f70d224e5e966fff86234'"`
  - `ssh root@pulse-cloud "python3 - <<'PY' ... urllib.request.urlopen('https://cloud.pulserelay.pro/portal?portal_handoff_id=cph_851a9101e81f70d224e5e966fff86234') ... PY"`
- Result: pass
- Verified:
  1. The exact handoff URL returned `200`.
  2. The portal response rendered the live `Pulse Account` HTML surface rather than a legacy Pulse Pro page or error fallback.

## Caveat

- This workstation still shows the previously recorded intermittent TLS reset when local Playwright or curl follows `https://cloud.pulserelay.pro` directly.
- That quirk does not change the product verdict for this gate because:
  1. the local self-hosted app route was exercised in-browser and produced the correct live `303` handoff target
  2. the exact live Pulse Account URL was then verified from `pulse-cloud` and returned `200`

## Outcome

- `self-hosted-commercial-ga-coherence` is exercised and passed.
- The self-hosted commercial package is now coherent across the public v6 preview, the in-app `Plans & Billing` entry, the live license-server handoff, and the live Pulse Account portal.
- `rc-to-ga-promotion-readiness` remains blocked separately; this record clears only the self-hosted commercial coherence gate, not GA promotion itself.
