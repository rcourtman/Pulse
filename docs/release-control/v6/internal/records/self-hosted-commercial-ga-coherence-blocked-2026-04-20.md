# Self-Hosted Commercial GA Coherence Blocked Record

- Date: `2026-04-20`
- Gate: `self-hosted-commercial-ga-coherence`
- Result: `blocked`

## Blocking Facts

1. The active control-plane target is still `v6-rc-stabilization`, and the live
   release profile still describes the current posture as an RC floor rather
   than a GA-ready self-hosted commercial package.
2. `repos/pulse-pro/landing-page/README.md` still states that the live public
   site intentionally mirrors the current v5 marketing posture and that the
   public v6 landing package must be reintroduced explicitly later.
3. `repos/pulse-pro/V6_LAUNCH_CHECKLIST.md` still records the dedicated public
   v6 landing surfaces, self-hosted v6 public checkout rehearsal, and broader
   GA-facing cutover work as incomplete.
4. `repos/pulse-pro/landing-page/index.html` still carries both `v5` and `v6`
   pricing models, with the current live posture intentionally remaining on the
   `v5` track while the `v6` model is retained as preview data.
5. The governed commercial lane (`L2`) had previously been treated as complete
   at the RC floor even though GA still lacks a named self-hosted commercial
   package pass covering the public site, explicit in-app commercial handoff,
   checkout/account/license-management flow, and GA-facing guidance together.

## Why The Gate Cannot Be Cleared Yet

The current bridge posture is acceptable for prerelease stabilization, but it
is not acceptable for GA. GA would otherwise expose a self-hosted commercial
path that still reads as transitional: live v5-style public framing, preview
v6 packaging, and incomplete GA-facing cutover guidance. That is exactly the
kind of "broken setup" that creates customer confusion even when the runtime
itself is technically working.

## Required Unblock Steps

1. Re-author the self-hosted GA commercial package so the public landing story,
   pricing presentation, and product promise all describe the actual v6
   Community / Relay / Pro offer instead of a v5 live bridge plus v6 preview.
2. Exercise the GA candidate through the real externally reachable
   self-hosted commercial path:
   - public landing
   - in-app upgrade or trial CTA
   - checkout/account/license-management flow
   - GA-facing guidance or release communication surface
3. Record a dated real-browser rehearsal showing that those surfaces stay
   coherent end-to-end and do not fall back to legacy v5 framing or ambiguous
   plan definitions.
4. Change the gate from `blocked` only after that rehearsal exists and the
   resulting package still matches the governed self-hosted pricing and product
   model.
