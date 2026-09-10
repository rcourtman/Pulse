# Installed alert lifecycle probe

This is an external Python-standard-library driver, not a notifier mock. It uses
normal HTTP host ingest and notification APIs and receives real webhook requests.
It does **not** start an installed Pulse instance. Local harness tests do not prove
installed acceptance.

## Exact target and fixture contract

Target: public source `2dcf23b9d75742049f72f7eac21b0bf7266ebf6e`,
`rcourtman/pulse:6.4.4-beta.2`, published server manifest digest
`sha256:d7d24aec91da45b901e7b1c4094d508b5f6c708dca114c0f4dbbdfddd86a82b4`.
Never substitute current main or rebuild the binary to pass this probe.

The fixture operator must supply:
- A disposable, **empty** instance with no provider connections, agents, customer
  data, or real destinations, isolated from production. The driver changes alert
  settings and uses global terminal recovery actions; loopback alone does not
  establish a disposable instance.
- An independently inspected immutable artifact, fresh persistent volume, and
  an identity JSON with `source` and `image_digest` above. Retain the registry
  inspection, selected platform digest/container image ID, process identity,
  volume identity, and launch command alongside it. This file is an operator
  attestation, not independent artifact verification by the driver.
- The normal webhook allowlist containing only `127.0.0.1/32`. Use the supported
  settings, not a security bypass. Pulse and the driver must share the same
  isolated network namespace (for example, both inside a disposable VM with the
  server using host networking); the receiver binds only loopback.
- A disposable admin API token with agent:report, monitoring:read,
  monitoring:write, settings:read and settings:write scopes, injected as
  `PULSE_ACCEPTANCE_TOKEN` without writing it to evidence or command arguments.
- Alert configuration PUTs require `monitoring:write`; `settings:write` does
  not imply it. A 403 stops the attempt: preserve it, correct the fixture
  credential contract through review, and use a fresh fixture rather than
  switching authentication methods or weakening the server scope check.
- An executable restart hook taking no arguments. It must verify the server is
  still healthy, gracefully stop/start **only this server** with the same exact
  image and volume, and wait for authenticated HTTP readiness. It must fail on
  unexpected exit/crash rather than restart away the adverse result. It emits
  only JSON with `before_process`, `after_process`, `same_volume: true`, and
  `image_digest`. Process identity must include start time/container identity,
  not merely a reused PID. Retain the underlying inspection and restart logs.
  The hook timeout is 120s; the driver does not invoke a shell.

Do not point this at an existing installation, disable enforced URL checks, use
customer destinations, delete state, or inspect database internals. If the server
crashes, stop, retain logs and failed results, and do not attempt crash diagnosis
through this driver. No process restart is performed by the local harness tests.

## Run

From the repository root, after the fixture operator supplies the above:

```sh
pulse-heavy-run -- python3 tests/qualification/alert-lifecycle/acceptance.py \
  --base http://127.0.0.1:7655 --port 18765 \
  --identity /absolute/private/fixture-identity.json \
  --restart-hook /absolute/private/restart-fixture \
  --evidence /absolute/private/new-evidence-directory \
  --run-id unique01 --disposable-fixture
```

The evidence directory must not exist. It is private and append-only by numbered
files within the run. Keep the data volume after failure; do not rerun against
the now-populated fixture. A new attempt needs a separately recorded fresh
fixture. API credentials and response headers are not recorded. Proxies and
HTTP redirects are disabled for the authenticated client.

## Scenarios and bounds

1. CPU 95% through `POST /api/agents/agent/report`, threshold 80/60 and no delay,
   using synthetic unified-agent identities with commands disabled.
2. Controlled first HTTP503 with `Retry-After: 0`, then HTTP200; require both
   recipient observations, active CPU alert, and sent delivery audit. This is
   transport retry acceptance, not proof that a durable queue retry occurred.
3. Indefinite HTTP503 until a dead-letter audit appears (600s deadline); retain
   the exact notification IDs and occurrence-specific incident timeline.
4. Actual operator-owned restart; require retained incident ID and terminal
   audit, then low CPU to clear and high CPU for a distinct recurrence.
5. Global terminal Retry; require the **original terminal notification ID** to
   gain a sent audit and two distinct delivered occurrence start times. Retain
   old and current incident timelines separately.
6. Disable agent alerts; ingest a fresh high-CPU identity for 30s and require
   no CPU alert or successful alert delivery for that identity.
7. Re-enable agent alerts, exhaust that identity (another 600s bound), Dismiss
   terminal failures, require healthy queue status and retention of all prior
   delivery-audit identities within the 200-entry query window.

Other waits default to 90s, HTTP requests to 10s. Observation errors fail rather
than being ignored. No queue budgets, retry timings, production thresholds or
product source are changed. Only the disposable alert *configuration* is set for
the controlled CPU stimulus. The driver deliberately disables grouping,
escalation and resolved notifications to isolate these scenarios.

## Interpretation and remaining checks

A zero exit means only the named assertions passed. The result deliberately
keeps `installed_acceptance_complete: false`. Review all receipt/snapshot files,
including failures. Do not infer that the entire release is qualified.

Still separate: recurrence timeline side-effect correctness (historical replay
must not mark the new occurrence delivered), retirement of the visible system
notification-warning, resolved-message delivery, pending/sending restart windows,
acknowledgement/snooze/quiet-hours policies, SMTP/email-provider acceptance, UI
reload and reporter acceptance. The driver does not automate these claims.
An ordinary generic-webhook recipient is not a real email provider.

## Contract proof

```sh
python3 -m unittest discover -s tests/qualification/alert-lifecycle -v
go test ./tests/qualification/alert-lifecycle -run TestExternalDriverWireContract -count=1
```

Python tests exercise the real loopback recipient's failure/success responses,
persistent failure, resolved-event exclusion, evidence preservation, URL and
redirect guards, deadlines and observer errors. The Go test decodes the driver's
actual report against production host wire types and executes its template
against production webhook payload types. Neither test launches Pulse.

The target's route/field contracts were traced in:
`internal/api/agent_ingest.go`, `internal/api/alerting/{alerts,notifications,notification_queue}.go`,
`internal/api/router_routes_monitoring.go`, `pkg/agents/host/report.go`,
`internal/alerts/config/types.go`, and
`internal/notifications/{notifications,webhook_enhanced,delivery_log}.go`.
Use `git show 2dcf23b9:<path>` when reviewing the installed target; current main
has newer incident-read implementation and must not stand in for this artifact.
