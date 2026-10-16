# Pro business implementation plan

Status: guided onboarding delivered, later phases proposed. Owner:
project-owner. Recorded: 2026-10-16.

The product direction is to sell shared operational control, accountability
and support to businesses while preserving a useful Community edition.
The first authorised implementation is Pro team onboarding. Later phases are
ordered proposals, not commitments or changes to the current plan matrix.

## Commercial boundary

Keep current prices, feature gates, retention limits, named-seat allowances,
grandfathered and lifetime grants, and support commitments unchanged. No
existing customer loses access. This plan authorises no refunds, migration,
production deployment or customer announcements. Moving history, AI features
or other paid capabilities into Community needs a separate product decision
supported by usage evidence. Longer-history telemetry measures usage, not
willingness to pay.

## Priorities

| Order | Customer job | Existing foundation | Work and completion evidence |
| --- | --- | --- | --- |
| 1 | Add a colleague and control their access | Community SSO and built-in group mappings, Pro custom roles and manual assignments, stable SSO principals, session-revoking deprovisioning | Guided sign-in setup, shareable instance login link, first-login discovery, refresh and role review, removal guidance. Qualify configured, missing, loading, failure, clipboard and narrow-screen states. This is the first implementation slice. |
| 2 | Know who has access and how seats work | Separate Enterprise organization invitations and membership, existing licence limits | Align default-instance users, membership and seat accounting before introducing invitations for ordinary Pro. Define what consumes a seat, admission versus existing access, stable identity binding and grandfathered unlimited grants. Qualify a second real user through sign-in, role changes, revocation and restart, including current seat limits. Do not enable multi-tenancy as an onboarding shortcut. |
| 3 | Send a useful operational report | Paid historical reporting, PDF/CSV, period comparison, backup/alert evidence and scheduled delivery | Complete the schedule-to-delivery journey, expose delivery failures and retry, make coverage and missing data explicit. Test a scheduled report arriving with representative infrastructure. Branding remains at its existing entitlement boundary unless separately approved. |
| 4 | Roll out agent configuration safely | Agent profiles, desired/applied fingerprints, history and rollback | Add staged deployment and clear per-node progress/failure handling. Qualify a small canary group, partial failure, retry and rollback against real disposable agents. Preserve action-runner authority and existing profile gates. |
| 5 | Explain changes and obtain dependable help | Audit search/export and current support channels | Connect changes to actor, affected system and outcome. Make diagnostics/support handoff practical without exporting secrets. Define sustainable response commitments before selling stronger guarantees. MSP customer isolation and delegation remain a distinct product track. |

Priorities 2 through 5 are delivered as individually claimed slices after
reviewing the preceding customer journey. Avoid building a new reporting,
identity, audit or agent-control engine alongside an existing owner.

## First slice: guided team sign-in

The user's job is: "Give my colleague their own access to this Pulse instance,
then check what they can do."

`Settings → Security → Users → Add teammate` uses existing security-status
discovery and the normal `/login` entry. It guides an administrator to configure
OIDC/SAML or trusted proxy authentication, confirm provider admission, share
the instance link, refresh after first successful sign-in and review roles.
It does not send email, create local accounts, bind authority to a contact
email, or grant access merely because a link was copied. Group mappings remain
authoritative at sign-in. Removal uses the existing deprovision endpoint and
explains that upstream provider admission must also be removed.

The guide is on the existing `rbac` administration surface. Community SSO and
built-in group mapping remain available. Enterprise organization invitations
stay separate. A configured provider is a prerequisite, not proof that an
individual can authenticate. The shareable URL uses the current browser
origin and excludes its query, fragment and administrator route.

The remaining membership/seat alignment is recorded as a governed coverage
gap. Guided sign-in alone does not close that gap or qualify a new seat model.

## Verification matrix

- Existing entitlement gate: no onboarding control or new reads without RBAC.
- Start at User Access, open/close the guide, Escape, outside click, keyboard
  focus and focus return. Repeat with scrolling at 390px and a desktop width,
  plus an intermediate width for the toolbar.
- Pending status read, failed read, retry, no SSO, disabled SSO, configured
  OIDC/SAML and trusted proxy. Sharing stays unavailable until discovery succeeds.
- Copy success and rejected/unavailable clipboard with a selectable fallback.
  Links must exclude credentials, private route/query/fragment and unsafe schemes.
- Refresh clears a stale search, discovers a newly signed-in identity and leads
  to the existing role editor. Exercise role update, removal confirmation,
  cancellation and reloaded state against the current build.
- Synchronised RBAC documentation, targeted UI and presentation tests,
  frontend type check/build, subsystem registry and release-control audits.

Browser fixtures establish UI behaviour only where backend responses are
controlled. They must not be described as real paid entitlement, IdP admission
or end-to-end seat qualification. Preserve that distinction in the evidence.

## Decision evidence

Use completed business workflows and feedback from actual team administrators
to choose the next slice. Track onboarding failures, whether a second person
gets useful access, scheduled report delivery, rollout failures and support
workload without collecting infrastructure content. A payment decision needs
customer evidence. Feature existence and prior model comparisons do not
establish willingness to pay.

## First-slice verification

The guided sign-in slice passed 76 targeted frontend tests, type checking,
the embedded frontend build, scoped lint and the status, registry and contract
audits. Browser proof covered `/settings/security-users` at 1440×1000,
820×844 and 390×844, including setup navigation, clipboard recovery, discovery
loading/error/retry, dismissal, keyboard focus, scrolling and footer access
after transient success feedback expired. Screenshots were inspected.
The [current guide screenshot](pro-team-onboarding-2026-10-16.png) uses
synthetic sign-in data and the disposable loopback instance address.

A separate browser identity completed real OIDC sign-in against a disposable
IdP. Its stable identity appeared after refresh. A Viewer assignment survived
page reload, permitted infrastructure-state reads and denied user
administration. Confirmed removal deleted the record and revoked its session.
Proxy discovery was a controlled UI fixture, not real proxy-enrolment proof.

This used the real private runtime with an ephemeral, locally signed Pro grant,
normal signature validation and development/mock modes disabled. It proves
the named workflow, not production licensing, named-seat enforcement or release
qualification. No customer system or production licence service was used.
The browser receipt retains hashes of the four changed UI/runtime source files
under `pulse-dev:/opt/pulse-release-worker/task-proofs/pro-team-onboarding-20261016/browser-evidence/receipt.json`.
Its public base was `39dcdc4ba5b2b17d27d0f121049808e5f4d05578` and its private
runtime source was `a76b4576898d168361127ef02648ebe1a6afdffb`.
