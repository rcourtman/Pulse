# Self-Hosted Commercial Transition Read-Only Reconciliation Record

- Date: `2026-07-21`
- Gate: `self-hosted-commercial-transition-coherence`
- Result: `local reconciliation hardened; external transition gate remains blocked`
- Evidence posture: `test-proof` plus `production-observed` read-only checks
- Reconciliation implementation commit: `pulse-pro:4fc4820`
- Current operator-packet commit: `pulse-pro:d18ebb3`

## Safety Boundary

This slice performed local tests, public HTTP GETs, pinned read-only SSH
inspection, and a Stripe GET-only catalog/portal audit. It did not change a
Stripe object, customer, subscription, invoice, payment method, webhook,
license record, deployed binary, service configuration, secret, release, or
customer system. No customer was contacted.

The local Stripe CLI has no current test-mode authority: its configured test
credential metadata expired on `2026-05-26`, and no `STRIPE_SECRET_KEY` was
present in the task environment. Consequently, no Stripe test-mode mutation
was attempted.

## Read-Only Observations

1. `https://license.pulserelay.pro/healthz` and
   `https://relay.pulserelay.pro/healthz` both returned `{"status":"ok"}`.
2. The public v6 pricing model passed
   `validate_public_pricing_model.py --track v6 --require-plan-keys`.
3. The production Stripe GET-only audit passed all 25 governed prices and the
   dedicated portal configuration `bpc_1TtNsxBrHBocJIGHTjRvG4Qf`. The four
   expected public Relay/Pro price IDs resolved. The only warnings were the two
   already-governed inactive v1 recurring prices.
4. Read-only service inspection found both `pulse-license` and `pulse-relay`
   active and enabled with healthy loopback endpoints. The Relay configuration
   contained both required license-server/feed settings. The deployed Relay
   binary had SHA-256
   `e00b1ba9a5a69a1958a4e8f6e9e85eefa285fe824422f19973401900397f1225`
   and reported VCS revision
   `be79bb1a074d296b8b227ed10bb2eebb745b1070`. The deployed license binary
   had SHA-256
   `5241742fb62d778fd5570fad99240b5f2af472f724b0b6943734d590e15d879b`.

One initial local audit invocation was discarded: sourcing the Bash SSH helper
from a different shell misresolved its pin path, and the helper's process-
substitution error did not propagate before `ssh` ran. The audit was repeated
through Bash with the repository pin enforced. Commit `pulse-pro:4fc4820`
changes both SSH and SCP helpers to obtain the pin arguments synchronously and
fail closed; a subprocess test proves that a missing pin cannot reach SSH.

## Reconciliation Defect And Fix

`invoice.payment_failed` previously projected `past_due` from the historical
invoice delivery and locally cached plan state. Invoice and subscription
events share the subscription ordering cursor, so an older failure is rejected
after an observed recovery. That cursor cannot protect against a recovery event
that never arrived, however: a delayed historical invoice failure newer than
the last observed event could regress already-recovered Stripe state into
grace.

Commit `pulse-pro:4fc4820` now retrieves the current Stripe subscription with a
GET, verifies subscription/customer ownership, resolves its one governed
price, and feeds the resulting authoritative snapshot through the same atomic
commercial projection used by transition application. Tests prove:

- a current `past_due` snapshot enters payment-failure grace and increments
  the version floor;
- a stale invoice failure cannot regress a currently active Stripe
  subscription even when its recovery event was missed locally;
- the durable inbox accepts a delayed failure after the last observed baseline,
  then converges to the authoritative active subscription; and
- replaying that same invoice event is idempotent and does not repeat the
  Stripe fetch or increment `license_version`.

The complete local license-server and Relay Go suites passed. The focused
Stripe catalog/remediation/runtime/price-creation/SSH Python suites passed 24
tests. `bash -n scripts/license_ssh.sh` and `git diff --check` also passed.

## Executable Operator Packet For The Remaining Gate

> **Superseded outline — do not execute the embedded commands below.** This
> record preserves the first operator outline for audit history, but it writes
> one-use Checkout URLs, quote tokens, and raw Stripe Event envelopes into the
> evidence directory and assumes a Checkout-created customer can use an
> existing Stripe test clock. The reviewed executable packet is
> `pulse-pro:docs/self-hosted-commercial-transition-gate-operator-packet.md` at
> commit `d18ebb3`; use only that version or a later reviewed replacement.

Run this only against a new isolated rehearsal license server, Relay server,
Pulse installation, and Stripe **test-mode** account. It intentionally refuses
the production license origin and live Stripe keys. The candidate must include
`pulse-pro:4fc4820`; do not substitute the currently deployed production
license binary because that commit has not been deployed.

### 1. Establish And Record The Isolated Authority

```bash
set -euo pipefail

: "${PULSE_REHEARSAL_LICENSE_ORIGIN:?set the isolated license-server origin}"
: "${PULSE_REHEARSAL_RELAY_ORIGIN:?set the isolated Relay origin}"
: "${PULSE_REHEARSAL_PULSE_ORIGIN:?set the isolated Pulse origin}"
: "${STRIPE_TEST_SECRET_KEY:?set a current Stripe test-mode secret key}"
: "${RELAY_MONTHLY_PRICE_ID:?set the isolated Relay monthly test price}"
: "${RELAY_ANNUAL_PRICE_ID:?set the isolated Relay annual test price}"
: "${PRO_MONTHLY_PRICE_ID:?set the isolated Pro monthly test price}"
: "${PRO_ANNUAL_PRICE_ID:?set the isolated Pro annual test price}"

case "$STRIPE_TEST_SECRET_KEY" in
  sk_test_*) ;;
  *) echo "refusing a non-test Stripe key" >&2; exit 1 ;;
esac
case "$PULSE_REHEARSAL_LICENSE_ORIGIN" in
  https://license.pulserelay.pro|https://license.pulserelay.pro/*)
    echo "refusing the production license origin" >&2; exit 1 ;;
esac

PULSE_REHEARSAL_EVIDENCE="$(mktemp -d)"
export PULSE_REHEARSAL_EVIDENCE
curl -fsS "$PULSE_REHEARSAL_LICENSE_ORIGIN/healthz" | tee "$PULSE_REHEARSAL_EVIDENCE/license-health.json"
curl -fsS "$PULSE_REHEARSAL_RELAY_ORIGIN/healthz" | tee "$PULSE_REHEARSAL_EVIDENCE/relay-health.json"
curl -fsS "$PULSE_REHEARSAL_LICENSE_ORIGIN/v1/public/pricing-model" \
  | tee "$PULSE_REHEARSAL_EVIDENCE/pricing-model.json" >/dev/null
for price_id in "$RELAY_MONTHLY_PRICE_ID" "$RELAY_ANNUAL_PRICE_ID" "$PRO_MONTHLY_PRICE_ID" "$PRO_ANNUAL_PRICE_ID"; do
  stripe prices retrieve "$price_id" --api-key "$STRIPE_TEST_SECRET_KEY" \
    >"$PULSE_REHEARSAL_EVIDENCE/price-$price_id.json"
done
```

The isolated license server must use a fresh database, the four test price IDs,
`PULSE_LICENSE_V6_ENABLED=true`, a new rehearsal-only
`PULSE_LICENSE_RELAY_FEED_TOKEN`, and a new rehearsal-only
`STRIPE_WEBHOOK_SECRET`. The isolated Relay must use the same feed authority
through `PULSE_RELAY_LICENSE_SERVER_URL` and
`PULSE_RELAY_REVOCATION_FEED_TOKEN`. Keep Stripe webhook IP enforcement off
only on this isolated service because the controlled replay below originates
from the operator host. Do not copy either feed token into Pulse.

### 2. Exercise Acquisition And Quoted Transitions

For each combination `relay/monthly`, `relay/annual`, `pro/monthly`, and
`pro/annual`, create a new test email subject and request checkout:

```bash
export PULSE_REHEARSAL_TIER=relay
export PULSE_REHEARSAL_CADENCE=monthly
curl -fsS -X POST "$PULSE_REHEARSAL_LICENSE_ORIGIN/v1/checkout/session" \
  -H 'Content-Type: application/json' \
  --data "{\"tier\":\"$PULSE_REHEARSAL_TIER\",\"billing_cycle\":\"$PULSE_REHEARSAL_CADENCE\"}" \
  | tee "$PULSE_REHEARSAL_EVIDENCE/checkout-$PULSE_REHEARSAL_TIER-$PULSE_REHEARSAL_CADENCE.json"
```

Open each returned test Checkout URL, pay with Stripe's governed test cards,
and retrieve the result through the returned session flow. Record the Stripe
customer/subscription IDs, Pulse license ID, price, tier, cadence,
`license_version`, and continuity epoch. Repeat with a declining test card for
the decline case.

For each owned test subject, request a fresh manage code from the rehearsal
mail sink, then create and apply a quote:

```bash
: "${PULSE_REHEARSAL_EMAIL:?set the owned rehearsal email}"
curl -fsS -X POST "$PULSE_REHEARSAL_LICENSE_ORIGIN/v1/manage/request" \
  -H 'Content-Type: application/json' \
  --data "{\"email\":\"$PULSE_REHEARSAL_EMAIL\"}"
read -r -p 'Rehearsal manage code: ' PULSE_REHEARSAL_CODE
export PULSE_REHEARSAL_TARGET_TIER=pro
export PULSE_REHEARSAL_TARGET_CADENCE=annual
curl -fsS -X POST "$PULSE_REHEARSAL_LICENSE_ORIGIN/v1/commercial/transition/quote" \
  -H 'Content-Type: application/json' \
  --data "{\"email\":\"$PULSE_REHEARSAL_EMAIL\",\"code\":\"$PULSE_REHEARSAL_CODE\",\"target_tier\":\"$PULSE_REHEARSAL_TARGET_TIER\",\"target_billing_cycle\":\"$PULSE_REHEARSAL_TARGET_CADENCE\"}" \
  | tee "$PULSE_REHEARSAL_EVIDENCE/transition-quote.json"
jq -er '.quote_token' "$PULSE_REHEARSAL_EVIDENCE/transition-quote.json" >"$PULSE_REHEARSAL_EVIDENCE/quote-token"
curl -fsS -X POST "$PULSE_REHEARSAL_LICENSE_ORIGIN/v1/commercial/transition/apply" \
  -H 'Content-Type: application/json' \
  --data "$(jq -n --rawfile token "$PULSE_REHEARSAL_EVIDENCE/quote-token" '{quote_token:($token|rtrimstr("\n"))}')" \
  | tee "$PULSE_REHEARSAL_EVIDENCE/transition-apply.json"
```

Repeat immediate Relay-to-Pro and monthly-to-annual transitions for success,
timeout-after-Stripe-success, duplicate apply, and replay. Repeat
renewal-bound Pro-to-Relay and annual-to-monthly transitions, including cancel
and resume before the effective timestamp. Use Stripe test clocks to advance
renewal, paid-through cancellation, payment-failure grace, grace expiry,
refund/dispute, recovery, and current-price re-entry without waiting on wall
clock time. After every step, record Stripe's current subscription and the
Pulse Account/runtime projection; require exactly one subscription item and
one current local subscription projection.

### 3. Deliver Real Stripe Events In Controlled Order

Do not register the isolated webhook endpoint with Stripe during this phase.
The test-mode mutations still create real Stripe Event objects, while delivery
remains under operator control. Retrieve each required event envelope:

```bash
: "${PULSE_REHEARSAL_EVENT_ID:?set a Stripe test-mode event ID}"
stripe events retrieve "$PULSE_REHEARSAL_EVENT_ID" --api-key "$STRIPE_TEST_SECRET_KEY" \
  >"$PULSE_REHEARSAL_EVIDENCE/$PULSE_REHEARSAL_EVENT_ID.json"
```

Deliver the saved envelopes in chronological order, reversed order, and with
duplicates. Set `PULSE_REHEARSAL_EVENT_FILES` to the exact desired sequence:

```bash
: "${STRIPE_REHEARSAL_WEBHOOK_SECRET:?set the isolated webhook signing secret}"
: "${PULSE_REHEARSAL_EVENT_FILES:?space-separated event JSON paths in delivery order}"
for event_file in $PULSE_REHEARSAL_EVENT_FILES; do
  event_ts="$(date +%s)"
  event_sig="$( (printf '%s.' "$event_ts"; cat "$event_file") \
    | openssl dgst -sha256 -hmac "$STRIPE_REHEARSAL_WEBHOOK_SECRET" -hex \
    | awk '{print $NF}')"
  curl -fsS -X POST "$PULSE_REHEARSAL_LICENSE_ORIGIN/stripe/webhook" \
    -H 'Content-Type: application/json' \
    -H "Stripe-Signature: t=$event_ts,v1=$event_sig" \
    --data-binary "@$event_file"
done
```

For the missing-event case, omit the event, leave the isolated service running
for one full reconciliation interval plus jitter, and require reconciliation
to fetch the event from Stripe. Then replay the omitted envelope and require no
state or version change. Capture the durable inbox statuses and final Stripe,
license, account, and runtime projections without recording buyer secrets.

### 4. Prove The Joined Relay And Pulse Version Floor

Activate the owned rehearsal license in the candidate Pulse installation and
establish a real Relay connection. Record the pre-change Pulse
`/api/license/status`, Relay session presence, grant `license_version`, and
reconnect token. While the session remains connected, apply a material
entitlement reduction from the matrix and deliver or reconcile its Stripe
event.

Require all of these postconditions:

1. the license transaction increments `license_version` and emits the matching
   `bump_license_version` feed event atomically;
2. the isolated Relay consumes that event and disconnects the old session by
   the next successful 30-second feed poll;
3. the old reconnect token is rejected;
4. Pulse observes the installation-scoped authoritative version, refreshes or
   clears the old grant, and never receives the global feed token; and
5. settings, report definitions, audit records, and the governed downgrade
   history remain present while paid work is denied.

Repeat for payment recovery and current-price re-entry. Capture timestamps so
the feed convergence bound is measurable.

### 5. Customer Journey And Verdict

Run the buyer/account journey at a desktop viewport and a phone viewport. At
each transition, capture the quote amount, recurring amount, effective date,
plan, cadence, cancellation/recovery copy, and the matching Stripe/runtime
state. Redact email, activation, grant, session, reconnect, feed, and webhook
secrets before moving evidence into the canonical repository.

The operator may mark the gate passed only when every matrix row converges to
one Stripe billing contract and one matching Pulse entitlement projection,
all restrictive changes advance the version floor and invalidate old Relay
and Pulse authority, replay/order/missing delivery produces the same result,
and protected customer data survives downgrade/cancellation. Otherwise record
the exact failed row and keep the gate blocked.

The follow-on executable packet at
`pulse-pro:docs/self-hosted-commercial-transition-gate-operator-packet.md`
binds every Stripe CLI request to an explicit `sk_test_` key. Its controlled
delivery phase deletes only the disposable test webhook endpoint before event
creation, retrieves real test-mode Stripe Event envelopes, and replays their
exact bytes through `pulse-pro:scripts/replay_stripe_test_events.py`. The helper
fails closed on live envelopes, the production license origin, redirects,
non-TLS remote targets, and missing isolated webhook authority. Its output
records event identity, order, payload digest, timestamp, and HTTP result
without recording the signing secret or response body.

The packet's Stripe command shapes are also exercised with the installed
Stripe CLI in `--dry-run` mode and a dummy test key, which performs no network
request. This caught and removed the obsolete `--format json` flag rejected by
Stripe CLI 1.42; the supported commands emit JSON without that flag. The test
now covers balance retrieval, webhook retrieval/deletion/listing, subscription
retrieval/update, invoice listing/payment, and event retrieval before an
operator receives real test authority.

Commit `pulse-pro:87cf91f` further makes each transition-matrix row use a fresh
subject in its exact starting state, supplies executable scheduled-cancel,
payment-failure/recovery, self-refund, and dispute paths, and replaces the
inapplicable test-clock step with an explicit test-mode billing-anchor reset.
It retains one-use Checkout and portal URLs and quote/activation secrets only
in shell memory, stores raw Event envelopes only in a private temporary
directory for exact-byte replay, and removes those temporary files after a
successful delivery. Tests parse every Bash block with `bash -n`, bind all 13
Stripe CLI invocations to the explicit test key, validate all supported CLI
shapes without network access, and execute all four evidence SELECTs against a
freshly initialized license-server schema. The full license-server suite and
all 386 script tests pass at that commit.

Commit `pulse-pro:bea4295` adds `set -euo pipefail` to the operator session and
an exit/signal cleanup trap for the private Event-envelope directory, so a
failed pipeline cannot be mistaken for passing evidence and an interrupted
replay does not strand the raw envelopes. The nine focused packet/replay tests
pass at that commit.

Commit `pulse-pro:d18ebb3` makes the separately approved production floor
phase operator-executable without broadening its authority. It requires the
approval-record digest, exact proof license/subscription/Relay instance and
target transition, and an `rk_live_` restricted read-only Stripe key; validates
all identifiers before interpolating remote read-only queries; retains the
manage code, quote token, email and any payment-pending secrets only in shell
memory; and records only filtered Stripe, license, feed-sequence, Relay
registry, online-state and journal evidence. The post-transition loop is
bounded to 75 seconds and passes only after the stale instance is offline and
its persisted reconnect credential is empty. Assertions require exactly one
license-version advance, Stripe/license price agreement, exactly one matching
`bump_license_version` event after the pre-proof feed sequence, the old Relay
grant version before and after disconnect, and the expected online-to-offline
change. All seven license-database evidence queries and the Relay registry
query execute against freshly initialized real schemas; both complete Go
suites, all 387 script tests, and the ten focused packet tests pass. No
production request or mutation was made while preparing this packet.

## Residual And Approval Boundary

The remaining proof requires fresh Stripe test-mode authority and controlled
test-mode mutations. The joined proof also requires deploying the candidate to
an isolated license/Relay environment and applying material entitlement
changes to an owned rehearsal subject. Using the live production services or a
real customer would mutate billing/license/runtime state and requires a
separate explicit approval. No such approval was present in this slice, so the
gate remains blocked.
