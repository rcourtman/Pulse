# Development and Automation Transparency

Pulse is maintained by me and built with heavy use of coding agents. This page
explains how, so it doesn't need repeating in every commit, release or issue.

## Who does what

- **Coding agents** write code, tests and documentation from requirements I set.
- **The automated maintainer** runs continuously. It investigates bugs, prepares
  fixes, triages issues and replies on GitHub as `pulse-triage[bot]`, and its
  commits carry that identity. Repository workflows appear as
  `github-actions[bot]`. Each problem in a mixed report gets its own answer
  under the [triage contract](ISSUE_TRIAGE.md).
- **I** set product direction, decide what the automation may do, and answer
  for everything that ships. I don't claim to have written every line.

I handle private support email myself. The automated maintainer does not read
a support mailbox or send email. Keep credentials and private diagnostics out
of public threads.

## How changes land and ship

Every change reaches `main` through a pull request. Maintainer changes receive
independent review, and every change must pass the repository's required checks
before merging. Nobody can bypass this, me included.

Releases follow a fixed weekly train: daily betas, then a release candidate that
soaks for 24 hours and is promoted unchanged to stable unless a `release-blocker`
issue stops it. Stable patches are not yet available, so fixes reach stable in
the next minor release; see [limited stable patches](RELEASE_PROCESS.md#limited-stable-patches).
Full rules: [Releases and update channels](RELEASE_PROCESS.md).

A fix merged to `main` is not yet a published fix. Replies name a version only
after checking its released source.

## Issue resolution and retesting

An issue is resolved when the reporter confirms the fix, or the maintainer
reproduces it with no reported symptom left unresolved. A merged fix or a
workaround alone is not enough.

Once the fix ships in a stable release, the maintainer names that version and
asks for a retest. After 14 days with no reply, and nobody else reporting the
same problem, the issue may be closed with the `closed-needs-retest-timeout`
label. That is **not confirmed resolution**, and it can be reopened. Feature
requests, `needs-human` issues and unexplained symptoms are never closed this way.

Still broken? Comment on the original issue rather than filing it again. Say
which symptom remains and which version you're on; add
[redacted diagnostics](TROUBLESHOOTING.md#collect-diagnostics-safely) if you
have them. Don't repeat an outage, update or backup just to retest.

## Authority and responsibility

The automated maintainer handles routine maintenance and issue replies without
asking me each time. The release train publishes on schedule without
per-release approval; no model chooses what goes in a release or when. None of
this skips review, soak or release checks. I can revoke that authority at any
time, and if an automated change is wrong, it's my bug to fix.

## How the work should be judged

By the software: is it correct, secure, tested and maintainable? The source,
issues, tests and releases are all public.

Older activity may predate this policy. For Pulse's own AI features, such as
Patrol and MCP, see [AI.md](AI.md).
