# Next Pulse Release — Disclosure Draft

This draft records customer-visible disclosures that must be carried into the
next Pulse release packet. The release version is assigned only when that
packet is cut.

## Outbound usage telemetry schema v2

This release updates Pulse's legacy unversioned outbound usage telemetry
payload to schema v2. Telemetry remains enabled by default unless an operator
has disabled it; an existing enabled or disabled choice is preserved on
upgrade. The purpose remains aggregate product and release understanding, and
the payload remains pseudonymous rather than tied to a Pulse account or
person.

Schema v2 adds these deliberately coarse signal categories:

- closed deployment-method, known-install-age, activation-stage,
  time-to-first-monitored-resource, and estate-size buckets;
- authentication-configured and monitoring-active booleans, plus an aggregate
  configured-connection count;
- aggregate alert fired, acknowledged, and resolved counts from the existing
  30-day local window;
- aggregate notification attempt, delivery, and failure counts from the
  existing seven-day local window; and
- a boolean indicating whether an operational outcome was observed in the
  existing 30-day local window.

The payload does not include names, email addresses, account IDs, hostnames,
credentials, infrastructure or resource identifiers, IP addresses, URLs,
paths, locale, recipients, notification endpoints, alert or notification
content, prompts, chat messages, command text, action output, token values,
browser events, or an event-level journey or clickstream. The rotating
pseudonymous installation ID continues to rotate every 30 days. Telemetry rows
are retained server-side for up to 90 days; request IP addresses are used only
transiently for rate limiting and are not stored in telemetry rows.

Existing installations receive a one-time, non-blocking
**Telemetry payload updated** notice after upgrade. It links directly to the
exact payload preview, the disable action, and the full privacy disclosure.
Fresh installations do not receive a duplicate banner because the current
payload and controls are already disclosed during setup.

## Terminology correction

Earlier public website copy used an anonymity label that was too strong for a
payload containing a rotating installation identifier. The accurate term is
**pseudonymous**. The website wording was corrected in July 2026, before
schema v2 reached a public Pulse release; the shipped privacy documentation
and in-product control identify the rotating pseudonymous ID and the concrete
data categories excluded from the payload.
