# Development and Automation Transparency

Pulse is maintained by me and developed with extensive automation, including
coding agents. This page is the standing disclosure for that process, so the
same explanation does not need to be repeated across every commit, release,
or issue thread.

## The short version

Automation contributes to code, tests, documentation, release notes, issue
triage, and routine repository maintenance. Routine changes may be
investigated, implemented, tested, and merged to `main` without line-by-line
human review. The continuously running maintainer does this within boundaries
I define.

I set the product direction and remain responsible for everything that ships.
The automated maintainer owns day-to-day maintenance, public issue replies,
and qualified releases. I do not claim to have personally written every line.
Automated changes must pass the applicable project tests and audit gates, and
released builds still go through Pulse's release qualification process.

## How it is used

- **Code and tests.** Coding agents implement and test changes from product,
  architectural, issue, and operational requirements.
- **Maintenance.** Automation monitors project signals, investigates defects,
  prepares fixes, and performs bounded routine repository work continuously.
- **Documentation and releases.** The maintainer prepares documentation,
  changelogs, and release material, then qualifies and publishes prereleases
  and stable releases under standing authority. Release claims must stay
  consistent with the code and qualification evidence.
- **Issue triage and support.** Automated issue and discussion replies post
  under the dedicated `pulse-triage` bot identity and link back to this page.
  Automated issue state changes use that identity as well. Mixed reports follow
  the [topic-integrity triage contract](ISSUE_TRIAGE.md): automation can surface
  declared secondary topics, but a maintainer or triage agent must give every
  actionable topic a linked disposition. Automated support replies are sent as
  Pulse Triage and link here as well.
- **Change provenance.** Commits made by the continuously running maintainer
  carry a dedicated bot author and committer identity. Issue-driven changes
  link back to the originating report where applicable.

The link on an automated reply is intentionally understated. It makes the
process discoverable without turning every technical exchange into a banner
about how the work was produced. Individual commits, fixes, and release notes
are not given tool-specific labels.

## How changes land and ship

Every change reaches `main` the same way, whoever or whatever wrote it: a
pull request that auto-merges when the repository's required checks pass.
The `main` branch ruleset requires that for every writer, with no bypass, so
a red check blocks the maintainer and me alike. The maintainer's pull
requests are opened by `pulse-triage[bot]` and state what changed, why it
was needed, which reports or demand-ledger entries it answers, and what
validation was used, so the record on GitHub is the record of the decision.

Releases run on a train rather than on demand. A release candidate is cut
from `main` into a `release/vX.Y` branch on a fixed schedule, soaks on the
opt-in preview channel, takes only backports of regression and security
fixes while it soaks, and is promoted to stable as the exact candidate
content. The promotion resolver in the release pipeline enforces the soak
and the exact-content rule for every dispatcher. The full rules are in
[RELEASE_PROMOTION_POLICY.md](https://github.com/rcourtman/Pulse/blob/b64709e7b7ad174e9c94ad2a0d3d841678690935/docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md),
under "Release Train".

## Authority and responsibility

On 8 September 2026 I confirmed standing authority for the automated maintainer
to own public Pulse issue replies and release publication, including stable
releases. These actions do not require my approval for each comment or release.
The maintainer chooses release scope, maturity and timing from current evidence.

Standing authority does not waive independent review, release qualification,
exact-candidate promotion, required soak, or verification after publication.
Public replies must follow the conversation and verified implementation and
release evidence. The maintainer must avoid repetitive unsolicited follow-ups.
I retain project ownership, product direction and the ability to revoke authority.

My role is to direct the project, design and maintain those boundaries,
monitor outcomes, and answer for the result. If an automated change is wrong,
it is still my bug and my responsibility to correct it.

## How the work should be judged

The relevant standard is the resulting software: whether a change is
understandable, maintainable, secure, covered by appropriate tests, and borne
out by real behaviour. Pulse's issue history, source, test gates, release
process, and corrections remain visible so specific concerns can be evaluated
on their merits.

This document describes the current standing policy. Older repository and
issue activity may predate it and may not carry the same link or identity.

For what Pulse itself does with AI as a product, including Pulse Patrol,
Assistant, and MCP, see [AI.md](AI.md). Those features are separate from the
development process described here.
