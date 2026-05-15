export const PATROL_PAGE_TITLE = 'Patrol';
// Page description names the Patrol ownership boundary: scheduled probing,
// context assembly, model-assisted investigation, and governed action.
export const PATROL_PAGE_DESCRIPTION =
  'Pulse checks your infrastructure on a schedule, gives your configured model the right evidence, and keeps fixes behind your approval policy.';
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
