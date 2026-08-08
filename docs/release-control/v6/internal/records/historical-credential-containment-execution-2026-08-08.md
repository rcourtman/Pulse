# Historical Credential Containment — Execution Evidence

Recorded: 2026-08-08.

Evidence tier: `production-observed`.

This record contains redacted role labels and sanitized operational outcomes
only. It contains no credential values, fragments, prefixes, fingerprints,
provider object identifiers, account identifiers, request material, or
credential values returned by providers. Restricted provider receipts,
sanitized probe details, and one-time secret handling records remain outside
the repository.

## Closed provider and replacement rows

### `PXM-01` and `PXM-02`

The two legacy, node-owned Pulse token identities were removed through the
authoritative Proxmox VE administration plane. A fresh cluster token inventory
showed both identities absent while the separately named current production and
development identities remained. Production Pulse stayed healthy and both live
Proxmox connections remained present after removal.

Disposition: provider `revoked`; replacement `validated`.

### `SLACK-01`

The historical incoming-webhook URL was recovered without printing or
persisting it and was sent an intentionally empty JSON payload. An active Slack
incoming webhook rejects that payload as missing message text; this request
instead returned HTTP 404, Slack's documented invalid-or-disabled webhook
condition. Signed-in workspace searches for both likely owner accounts returned
no workspace, and the current repository contains no project Slack credential
consumer. The generic user-configured webhook feature remains available but
does not require a project-owned replacement.

Disposition: provider `decommissioned`; replacement `not-applicable-retired`.

### `TELEGRAM-01`

The historical identity was confirmed to be a provider-shaped bot token in the
removed integration setup material. A read-only Telegram Bot API identity query
returned HTTP 401, proving that token is no longer accepted. The current
repository contains no project Telegram bot credential consumer. Telegram
remains supported through generic user-supplied webhook configuration and does
not require a project-owned replacement bot.

Disposition: provider `decommissioned`; replacement `not-applicable-retired`.

### `PBS-01` and `PBS-02`

The supported Proxmox Backup Server token-deletion command removed the legacy
Pulse read-only identity. The post-removal inventory showed that identity
absent while the current Pulse monitoring identity and the independent backup
identity remained. The two historical source values map to this retired legacy
role and close independently against the same authoritative removal event.

Disposition: provider `revoked`; replacement `validated`.

### `PULSE-01` through `PULSE-04`

The production Pulse security-token inventory contained exactly two current
records, both created in July 2026, and no historical or pre-2026 token record.
The current agent credential had production activity on 2026-08-08 and the
production health endpoint remained healthy.

Disposition: provider `inventory-absent`; replacement `validated`.

### `PRO-DIGITALOCEAN-01`

The signed-in provider token inventory contained exactly one current personal
access token. The historical role was absent. The surviving administration
credential remained valid through a read-only account query after inventory.

Disposition: provider `inventory-absent`; replacement `validated`.

### `PRO-LICENSE-ADMIN-01`

A new high-entropy credential was installed atomically in the production
license service, canonical local store, and private repository secret. The
service restarted with one configured admin credential. The replacement passed
the canonical admin-posture verifier, public health, the backup-export read
path, and a non-mutating refund workflow dry run. The time-limited recovery
copy was destroyed after validation. Replacing the service's single configured
credential and restarting it invalidated the historical value.

Disposition: provider `revoked`; replacement `validated`.

### `PRO-RESEND-01`

A sending-only replacement key was installed in the production license
service, canonical local store, and private repository secret. The restarted
service sent an internal subscription-management verification message, and a
dedicated private repository workflow sent a separate internal operational
verification message. Provider metadata showed replacement-key activity. The
matched historical provider key was then deleted and subsequent authentication
with it was rejected by the provider. Production health remained green.

Disposition: provider `revoked`; replacement `validated`.

### `PRO-STRIPE-WEBHOOK-01`

The production license webhook and disabled cloud webhook were replaced as
separate provider endpoint objects with the same event contracts. The new
production signing credential was installed atomically in the production
service and canonical local store; the disabled cloud replacement credential
was installed in its protected recovery source. The service restarted, the new
production endpoint was enabled, and the new cloud endpoint remained disabled.
Stripe resent an already-processed live-safe subscription event to the new
production endpoint; the service recorded one accepted webhook and no signature
rejection. Both old endpoint objects were deleted, making their signing
credentials unusable, and the recovery copy was destroyed.

Disposition: provider `revoked`; replacement `validated`.

## Scoped replacement closure

### `PRO-CLOUDFLARE-01`

The historical role was identified as the legacy Global API Key, not a scoped
API token. Provider audit history showed an earlier key deactivation and
replacement, but that event predated the public exposure. A new post-exposure
Global API Key change was therefore executed on 2026-08-08, immediately
invalidating the prior global key. The active landing deployment remains on a
separate scoped API token. A current-tree inventory found no consumer of the
legacy global-key variable, while the landing deployment contract explicitly
uses the scoped token. The scoped token passed the provider's active-token
verification and successfully read the current Pages project inventory. The
invalidated global key was removed from the protected local credential source,
which now retains only the scoped-token path and non-secret account metadata.
No replacement global key was retrieved or persisted.

A later local diagnostic rendered the scoped token outside its protected
source. That token was capability-matched to its provider record, revoked in
the provider dashboard, and then rejected by the provider verification API
with HTTP 401. Its exact dashboard row is absent. A fresh single-account token
grants only `Cloudflare Pages: Edit`; it was installed atomically in the
protected local credential source with mode `0600`, and verified active. The
replacement also successfully read the Pages project inventory and located the
current landing project. Neither repository has a GitHub Actions secret or
current-tree credential consumer for this role; the only active consumer is
the protected local landing deployment path.

Disposition: provider `revoked`; replacement `validated` after scoped-token
re-rotation.

No history rewrite was performed or is required for provider containment.
