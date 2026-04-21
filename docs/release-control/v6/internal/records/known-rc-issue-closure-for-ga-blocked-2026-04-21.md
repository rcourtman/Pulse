# Known RC Issue Closure For GA Blocked Record

- Date: `2026-04-21`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `blocked`

## Blocking Facts

1. All issues explicitly labeled `affects-6.0.0-rc.1` are already closed, so
   the current blocker is not unlabeled RC1 drift.
2. The only issue explicitly labeled `affects-6.0.0-rc.2` remains open:
   `#1435` (`[Bug]: LXC command installing v6.0.0-rc.2`).
3. Additional open RC-soak issues remain active on the v6 line:
   - `#1409` (`No limit devices for self-hosted / homelab`) still contains a
     fresh `2026-04-21` screenshot showing stale rc.2 cap copy on a self-hosted
     install.
   - `#1429` (`missing docker info including updates`) remains open after RC2
     with unresolved user-visible trust concerns around discoverability, stale
     cap copy, compare-plans handoff, and trend wording.
   - `#1430` (`Width of the Name column`) remains open for Firefox table
     layout.
   - `#1432` (`Dashbord filter`) remains open as dashboard behavior expected
     before the feature-complete v6 GA line is called done.
   - `#1436` (`Better disk i/o reads for LXC containers`) remains open as an
     LXC workload visibility gap discovered during the RC soak.
4. Some local fixes already exist on `pulse/v6-release`, including
   `4711d1116` for `#1435` and `770cceae5` plus related cap-scrub commits for
   the stale self-hosted cap regression still surfacing in `#1409` and
   `#1429`, but local fixes alone are not sufficient evidence to clear the GA
   issue-closure rule.
5. The project owner has now locked a stricter product truth for v6 GA:
   Pulse v6 is intended to be feature-complete at GA, so known RC-era
   user-visible issues are not acceptable carryover.

## Why The Gate Cannot Be Cleared Yet

The RC program exists to surface the exact bugs, regressions, layout failures,
trust breaks, and coverage gaps that would otherwise reach stable users. Once
v6 GA is defined as feature-complete for the admitted v6 scope, those issues
cannot be normalized as "post-GA cleanup" just because the promotion packet is
already exercised. Shipping GA while the current RC issue set is still open
would knowingly make stable users inherit unresolved feedback from the very
validation cohort meant to protect them.

## Required Unblock Steps

1. Materialize and maintain a dated RC issue-closure record for the actual GA
   candidate. That record must enumerate every known RC-era issue in scope and
   its disposition.
2. For each current open item (`#1435`, `#1409`, `#1429`, `#1430`, `#1432`,
   and `#1436` as of `2026-04-21`), do exactly one of the following:
   - fix it in the candidate and verify the affected user-visible surface
   - prove it invalid with evidence
   - conservatively supersede it with a linked canonical issue only if the
     original user-visible failure is actually resolved or explicitly narrowed
3. Do not treat open user-visible RC-era issues as acceptable GA carryover.
4. Change the gate from `blocked` only once the dated closure record shows no
   remaining open RC-era issue intended for v6 GA.
