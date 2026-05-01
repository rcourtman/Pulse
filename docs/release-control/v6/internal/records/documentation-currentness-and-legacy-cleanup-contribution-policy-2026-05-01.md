# Documentation Currentness And Legacy Cleanup Contribution Policy Record

- Date: `2026-05-01`
- Gate: `documentation-currentness-and-legacy-cleanup`
- Scope:
  - `pulse`
  - lane `L9`

## Context

The `release/5.1` maintenance branch added an issue-first contribution policy
to reduce unreviewed external PR churn while recent RC feedback is active.
Pulse v6 already had the dedicated v6 pre-release feedback template and hub,
but the public contribution docs still presented a generic PR-first flow.

## Review Surface

1. `README.md`
2. `CONTRIBUTING.md`
3. `.github/PULL_REQUEST_TEMPLATE.md`
4. `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`
5. `.github/v6_rc_feedback_hub.md`

## Outcome

Active v6-facing contribution guidance now matches the maintained policy:

- issues and discussions are the normal intake path;
- unsolicited external pull requests are not part of the normal contribution
  flow;
- maintainer-requested PRs remain possible when tied to a tracked issue;
- v6 pre-release feedback still has a dedicated template and feedback hub.

This keeps the RC3 feedback path issue-first without removing the local
development notes needed for reproducible reports and maintainer-directed
patches.
