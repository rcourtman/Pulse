export const PATROL_PAGE_TITLE = 'Patrol';
// Page description names the trust loop the surface owns end-to-end:
// investigate the infrastructure, capture evidence, propose safe fixes
// under operator approval. Replaces the prior "verify health, review
// findings, control runtime" copy, which framed the page as a passive
// monitoring console rather than the proactive-intelligence engine
// described in the Pulse Intelligence vision (investigation +
// explanation + governed action).
export const PATROL_PAGE_DESCRIPTION =
  'Pulse investigates your infrastructure, gathers evidence for every finding, and proposes safe fixes under your approval policy.';
// Tooltip on the page title; shares the same framing as the description so
// hover and inline copy don't tell different stories about what Patrol does.
export const PATROL_PAGE_TITLE_TOOLTIP = PATROL_PAGE_DESCRIPTION;

export function getPatrolPageHeaderMeta() {
  return {
    title: PATROL_PAGE_TITLE,
    description: PATROL_PAGE_DESCRIPTION,
    titleTooltip: PATROL_PAGE_TITLE_TOOLTIP,
  } as const;
}
