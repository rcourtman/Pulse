# Pulse v6 RC Demo Site Copy Deck

This document locks the public-facing copy for exposing Pulse v6 as an
optional preview on the existing public demo while Pulse v5 remains the stable
default.

Use this copy when the demo website or demo runtime adds a v5 versus v6 switch.
Do not improvise alternative wording on the public site unless this document is
updated as the canonical source.

## Positioning Rules

- Keep Pulse v5 as the default public demo while v5 remains the current stable
  release.
- Expose Pulse v6 only as an explicit `Preview` or `RC` path.
- Do not use apologetic language such as "probably broken" or "expect lots of
  bugs".
- Do not use a blocking modal just to explain the preview.
- Public demo copy must explain that the v6 demo runs on mock data.
- Public demo copy must not expose billing, license, or customer-specific
  commercial state just to explain the preview.

## Demo Switch

Use a two-option segmented switch or equivalent top-level choice:

- `Pulse v5 Stable`
- `Pulse v6 Preview`

Helper text directly below the switch:

`Pulse v5 remains the stable demo. Pulse v6 Preview shows the upcoming v6 experience on mock data before general availability.`

If the switch needs shorter mobile copy:

`v5 is stable. v6 Preview shows the upcoming experience on mock data.`

## Preview Entry CTA

If the website needs a dedicated CTA into the v6 demo path, use:

- Primary label: `Try Pulse v6 Preview`
- Secondary label: `Open v6 Preview`

Supporting copy:

`Explore the upcoming Pulse v6 interface on mock data while Pulse v5 remains the current stable demo.`

## In-App Demo Banner

When a user is inside the public v6 demo runtime, show one low-key non-modal
banner near the top of the shell.

Title:

`You’re viewing Pulse v6 RC on the public demo.`

Body:

`This preview runs on mock data and is intended to show the new v6 experience before general availability. Pulse v5 remains the current stable release.`

Optional shorter body for tighter layouts:

`This v6 preview runs on mock data. Pulse v5 remains the current stable release.`

Primary actions:

- `View release notes`
- `Send feedback`

Optional third action if the product surface supports returning to v5 from the
same entry point:

- `Back to Pulse v5`

## Preview Badge

If a compact badge is needed in navigation or cards, use:

- badge text: `Preview`
- optional long form: `v6 Preview`

Avoid:

- `Beta`
- `Experimental`
- `Unstable`
- `Try at your own risk`

## Demo Landing Card

If the demo website includes a short explanatory card before entering the v6
preview, use:

Title:

`Try the new Pulse v6 experience`

Body:

`Preview the upcoming v6 interface on mock data, compare it with the stable Pulse v5 demo, and send feedback before general availability.`

Secondary note:

`The public preview is for evaluation of the v6 experience. It is not a customer-specific environment and does not represent live billing or license state.`

## Supportive FAQ Snippet

If the public demo page includes a short FAQ or disclosure block, use:

Question:

`Is the v6 demo the current stable Pulse release?`

Answer:

`No. Pulse v5 remains the current stable release. The v6 demo is an opt-in public preview of the first Pulse v6 release candidate running on mock data.`

Question:

`Does the public v6 demo reflect live account or billing state?`

Answer:

`No. The preview is intended to show the v6 experience on mock data. Customer-specific billing, licensing, and account state are not the purpose of the public demo.`

## Copy To Avoid

Do not ship public demo copy like:

- `This probably has lots of issues`
- `Use at your own risk`
- `This is just an alpha`
- `Billing may be wrong`
- `License state is fake`

Those statements undermine confidence and turn a deliberate RC preview into an
uncontrolled warning banner.

## Canonical Links

- Release notes: `docs/releases/RELEASE_NOTES_v6.md`
- Support pack: `docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md`
