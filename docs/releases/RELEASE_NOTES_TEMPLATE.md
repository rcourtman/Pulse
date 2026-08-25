# Pulse vX.Y.Z Release Notes

One short paragraph explaining the customer outcome of the release. Lead with
what feels better or works now, not how it was implemented.

## What's improved

- **Short outcome** — Explain where users notice it and why it matters.
- **Short outcome** — Keep each item concrete and independently useful.
- **Short outcome** — Prefer observable behavior over component names.
- **Short outcome** — Use plain language and avoid implementation detail.

Use four to six meaningful improvements for a normal RC or minor release. A
narrow patch may use fewer rather than padding the notes with internal work.

## Fixes

- State a visible problem that no longer happens.
- Name the affected page, workflow, integration, or platform when useful.

Omit this section only when there are genuinely no user-facing fixes.

## Before you upgrade

Include only compatibility, migration, signing, companion-app, known-risk, or
required operator information. Omit this section when users do not need to do
or understand anything before upgrading.

The release pipeline appends the `Install` and `Roll back` sections from
governed version metadata. Do not add release qualification, validation,
workflow, gate, governance, or promotion-metadata sections to the customer
notes.
