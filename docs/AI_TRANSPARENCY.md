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
The automated maintainer owns day-to-day maintenance and public issue replies;
releases follow the fixed train described below. I do not claim to have
personally written every line.
Automated changes must pass the applicable project tests and audit gates, and
released builds still go through Pulse's release qualification process.

## How it is used

- **Code and tests.** Coding agents implement and test changes from product,
  architectural, issue, and operational requirements.
- **Maintenance.** Automation monitors project signals, investigates defects,
  prepares fixes, and performs bounded routine repository work continuously.
- **Documentation and releases.** The maintainer prepares documentation and
  changelogs. The release train builds, checks and publishes releases. Release
  claims must stay consistent with the actual published source and checks.
- **Issue triage.** Automated issue and discussion replies post
  under the dedicated `pulse-triage` bot identity and link back to this page.
  Automated issue state changes use that identity as well. Mixed reports follow
  the [topic-integrity triage contract](ISSUE_TRIAGE.md): automation can surface
  declared secondary topics, but a maintainer or triage agent must give every
  actionable topic a linked disposition. Private support email is handled by
  Richard, not the automated maintainer: it does not read a support mailbox or
  send email. Keep credentials and private diagnostics out of public threads.
- **Change provenance.** Commits made by the continuously running maintainer
  carry a dedicated bot author and committer identity. Issue-driven changes
  link back to the originating report where applicable.

The link on an automated reply is intentionally understated. It makes the
process discoverable without turning every technical exchange into a banner
about how the work was produced. Individual commits, fixes, and release notes
are not given tool-specific labels.

## How changes land and ship

Every change reaches `main` through a pull request, whoever or whatever wrote
it. Maintainer changes receive independent review and must pass the
repository's required checks before merging.
The `main` branch ruleset requires that for every writer, with no bypass, so
a red check blocks the maintainer and me alike. The maintainer's pull
requests are opened by `pulse-triage[bot]` and state what changed, why it
was needed, which reports or demand-ledger entries it answers, and what
validation was used, so the record on GitHub is the record of the decision.

Every 14 days after the last stable release, the train cuts the next minor
release candidate from the head of `main`. It carries everything on `main` at
that cut, not a selected set of fixes. Repairs go on `main`, not onto an older
release line.

The candidate soaks for 24 hours on the opt-in preview channel, then is promoted
unchanged to stable unless an open issue labelled `release-blocker` stops
promotion. A candidate regression from the previous stable release or a release
failure is repaired on `main`; once the blockers are closed, the train cuts a
fresh candidate that carries the repair. Product changes are not inserted into the frozen candidate.

A fix merged to `main` is not yet a published fix. A published beta or release
candidate is available to preview users, not proof that stable users have it.
Issue replies name the containing version only after checking its actual
released source, and ask for a retest when appropriate. See the
[published releases](https://github.com/rcourtman/Pulse/releases) for available
versions; a source commit or a passing test alone does not establish availability.

## Authority and responsibility

The automated maintainer owns ordinary maintenance and public Pulse issue
replies under standing authority. These actions do not require my approval for
each change or comment. The release train publishes prereleases and stable
releases without per-release approval; it follows the cycle above rather than
having a model choose release scope, maturity or timing.

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
