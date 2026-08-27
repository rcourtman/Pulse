# Pulse vX.Y.Z Release Notes

One short paragraph explaining the customer outcome of the release. Lead with
what feels better or works now, not how it was implemented.

## What's improved

- **Short outcome** — Explain where users notice it and why it matters.
- **Short outcome** — Keep each item concrete and independently useful.
- **Short outcome** — Include fixes here as outcomes rather than repeating
  them in a second section.

Use up to six meaningful improvements for a normal RC or minor release. A
narrow patch may use fewer rather than padding the notes with internal work.
Each user-visible change belongs in this list exactly once. Prefer observable
behavior over component names, group related implementation work into one
user-recognizable theme, and use plain language.

For an RC, cover only changes since the immediately preceding RC (or the
previous stable release for RC1). Do not repeat improvements already announced
in an earlier RC. For a stable GA release, cover the complete release train
since the previous stable release and boil the full commit range down to a few
themes that explain what users will experience differently. Do not concatenate
the RC notes or attempt to list every commit.

Do not add a separate `Fixes` section. That shape encourages the same change to
be described twice as both an improvement and a fix.

## Before you upgrade

Include only compatibility, migration, signing, companion-app, known-risk, or
required operator information. Omit this section when users do not need to do
or understand anything before upgrading.

The release pipeline appends the `Install` and `Roll back` sections from
governed version metadata. Do not add release qualification, validation,
workflow, gate, governance, or promotion-metadata sections to the customer
notes.
