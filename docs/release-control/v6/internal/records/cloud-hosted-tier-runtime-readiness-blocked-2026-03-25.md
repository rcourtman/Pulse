# Cloud Hosted Tier Runtime Readiness Blocked Record

- Date: `2026-03-25`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Result: `blocked`

## Blocking Facts

1. The live hosted Pulse Cloud control plane is still provisioning tenant
   runtimes without a machine-owned relay registration path.
2. Live hosted tenant containers are healthy on `pulse-cloud`, but sampled
   tenant runtimes still lack persisted relay state:
   - `/etc/pulse/billing.json` present
   - `/etc/pulse/relay.enc` absent
   - `/etc/pulse/activation.enc` absent
3. The live relay service is currently reporting zero connected Pulse instances:
   - `pulse_relay_instance_connections 0`
   - `pulse_relay_app_sessions 0`
4. The live relay host currently confirms only `PULSE_RELAY_PUBLIC_KEY` in the
   running environment for the checked relay validation surface; the hosted
   entitlement key path required for hosted lease validation is not yet wired on
   the live relay service.
5. The current hosted runtime code line only auto-starts relay when persisted
   relay config already exists and only hands the relay client a classic
   activated-license token. Hosted entitlement leases do not currently cross
   that boundary in production.
6. The canonical fix now exists in repo but is not yet the live hosted/runtime
   behavior:
   - `pulse`: hosted entitlement-backed relay bootstrap and hosted relay
     registration-token fallback
   - `pulse-pro/relay-server`: hosted entitlement lease validation and hosted
     instance-id derivation from `instance_host`
7. The current owned relay journey proof only exercises manual relay enablement
   and does not prove that a fresh hosted tenant automatically becomes
   mobile-visible through relay after provisioning.

## Why The Gate Cannot Be Cleared Yet

The currently recorded hosted-runtime production evidence still proves hosted
auth handoff, runtime entry, and hosted billing/admin surfaces, but it does not
prove the current expected v6 hosted-mobile path. In the live system today, a
fresh hosted tenant can still come up with valid hosted billing entitlement and
no canonical relay registration path. That means hosted runtime is not yet
coherent enough to treat the hosted tier as fully RC-ready for the cloud/mobile
surface.

## Required Unblock Steps

1. Deploy the hosted runtime fix that auto-bootstraps relay from hosted
   entitlement state and uses the hosted entitlement lease as the relay
   registration credential when no activated license exists.
2. Deploy the relay-server fix that validates hosted entitlement leases and
   derives stable hosted instance ids from `instance_host`.
3. Wire the hosted entitlement public key into the live relay environment.
4. Re-run the real hosted proof from a fresh production canary:
   - provision hosted tenant
   - confirm relay connects without manual `/api/settings/relay` mutation
   - confirm onboarding QR/deep-link exposes a real relay `instance_id`
   - confirm Pulse Mobile can pair against that hosted tenant
5. Replace this blocked record with fresh `real-external-e2e` evidence only
   after that end-to-end hosted-mobile path is exercised successfully.
