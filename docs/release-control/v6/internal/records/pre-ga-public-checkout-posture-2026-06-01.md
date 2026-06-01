# Pre-GA Public Checkout Posture Record

- Date: `2026-06-01`
- Lane: `L2`
- Surface: public landing checkout and license-server production handoff
- Result: production remains on v5 checkout

## Owner Direction

On 2026-06-01 the project owner clarified that Pulse v6 has not been released
as GA. The governed v6 pricing and checkout package may be exercised through
explicit preview and proof paths, but it is not the production public checkout
posture.

## Required Posture

1. Production public landing and checkout use `PULSE_PUBLIC_RELEASE_TRACK=v5`.
2. Production license runtime keeps `PULSE_V6_RELEASE_APPROVED=0`.
3. `PULSE_LICENSE_GRANDFATHERED_RECURRING_SNAPSHOT_AT` stays unset until the
   intentional GA/public pricing cutover.
4. Public v6 checkout runs only under preview/proof hosts or a deliberate GA
   cutover packet.
5. The production v6 flip requires explicit owner approval tied to the GA launch
   packet; readiness records, local history, preview audits, and internal target
   completion are not approval by themselves.
6. If a mistaken production v6 flip happens before GA, rollback is
   `PULSE_PUBLIC_RELEASE_TRACK=v5 PULSE_V6_RELEASE_APPROVED=0` followed by the
   public release deploy/audit.

## Verification On 2026-06-01

- `/etc/pulse-license/secrets.env` held `PULSE_PUBLIC_RELEASE_TRACK=v5`.
- `/etc/pulse-license/secrets.env` held `PULSE_V6_RELEASE_APPROVED=0`.
- No active `PULSE_LICENSE_GRANDFATHERED_RECURRING_SNAPSHOT_AT` value remained.
- Production `https://pulserelay.pro` rendered v5 public purchase copy and the
  v5 annual checkout price.
- `PULSE_PUBLIC_RELEASE_TRACK=v5 ./scripts/audit_public_release.sh` passed from
  `repos/pulse-pro`.
- `bash ./scripts/validate_license_runtime_config.sh` passed from
  `repos/pulse-pro`.
- Database rollback verification found no remaining v6-cutover flags on legacy
  or v6 rows.

## Outcome

- `self-hosted-commercial-ga-coherence` remains preview/GA-package evidence only.
- `commercial-ga-promotion-package` owns the future production public flip.
- Until that follow-up is actively executed with explicit owner approval,
  production checkout stays v5 and v6 GA public checkout is not live.
