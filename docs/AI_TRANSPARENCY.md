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

I set the product direction, control releases, and remain responsible for
everything that ships. I do not claim to have personally written every line.
Automated changes must pass the applicable project tests and audit gates, and
released builds still go through Pulse's release qualification process.

## How it is used

- **Code and tests.** Coding agents implement and test changes from product,
  architectural, issue, and operational requirements.
- **Maintenance.** Automation monitors project signals, investigates defects,
  prepares fixes, and performs bounded routine repository work continuously.
- **Documentation and releases.** Automation may draft documentation,
  changelogs, and release material, which must stay consistent with the code
  and the evidence used to qualify a release.
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

## Authority and responsibility

The automation can operate continuously, but it is not the project owner and
does not have unrestricted authority. Product direction, architecture,
acceptable risk, capability boundaries, and release decisions remain mine.
Release publication and other high-impact actions require explicit approval.

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
