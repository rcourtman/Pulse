# Localized Product Surfaces Lane - 2026-08-06

## Decision

Promote `localized-product-surfaces` from a planning-only candidate into L24,
`Localized product surfaces`. The existing implementation is broad enough to
form a durable governed product floor, while the remaining untranslated
journeys and locale-QA depth stay explicit through the
`localized-product-surfaces-expansion` lane follow-up.

This is a cross-cutting product lane rather than a replacement owner for the
`frontend-primitives` or `cloud-paid` subsystems. Those contracts continue to
own their runtime boundaries; L24 owns the coherence of customer-visible
locale behavior across repositories and delivery surfaces.

## Accepted Floor

- English is the source catalog and deterministic fallback language.
- German (`de`) and Spanish (`es`) are the first-wave supported locales.
- Desktop locale normalization, lazy catalogs, persisted selection, and
  fallback behavior are centralized under `frontend-modern/src/i18n`.
- Settings General, first-session setup and monitoring handoff, Alerts
  Overview triage, and the pricing handoff have explicit first-wave copy and
  guardrails.
- German and Spanish install/getting-started documentation is linked from the
  canonical documentation entry point.
- Pulse Mobile uses one root provider, typed catalogs, locale normalization,
  and explicit first-wave policy across pairing, approvals, alerts, findings,
  recovery, source connection, settings, and the Now experience.
- The public landing site publishes generated German and Spanish routes from
  explicit catalogs, with localized structured data and deployment drift
  checks.
- Checkout preserves the chosen first-wave locale and license delivery uses
  localized email catalogs.
- Machine-facing identifiers, commands, environment variables, configuration
  keys, API fields, payloads, logs, resource and user-entered names, product
  names, and integration names remain untranslated unless a contract says
  otherwise.

## Evidence

The canonical L24 evidence list in `status.json` links the desktop catalog and
journey records, localized documentation, Pulse Mobile localization contract
and proof, and Pulse Pro landing, checkout-locale, and license-email sources.
The historical slice records remain the dated implementation proof for the
first desktop journeys; later mobile and commercial evidence demonstrates that
the floor now spans all three active customer-facing repositories.

## Residual

L24 is a bounded residual, not a claim that every string is translated.
Remaining work includes untranslated desktop and mobile journeys, Pulse
Account and checkout-completion surfaces, wider install and troubleshooting
documentation, alert configuration/history and monitoring tables,
native-speaker review, pseudo-locale and extraction tooling, broader visual QA,
and evidence-based admission of later locale waves.

These residuals are owned by the
`localized-product-surfaces-expansion` lane follow-up. They do not reopen the
accepted first-wave architecture or allow ad hoc page-local translation paths.
