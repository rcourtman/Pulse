import type { PatrolAutonomyLevel } from '@/api/patrol';

export const PATROL_PAGE_TITLE = 'Patrol';

export const PATROL_PAGE_WATCH_ONLY_DESCRIPTION =
  'See what needs a decision, choose the next step, and keep a verified record.';

export const PATROL_PAGE_MONITOR_DESCRIPTION =
  'Watch only: Patrol checks your estate and brings every current issue to you.';

export const PATROL_PAGE_DESCRIPTION =
  'See what needs a decision, choose the next step, and keep a verified record.';
export const PATROL_PAGE_TITLE_TOOLTIP = PATROL_PAGE_DESCRIPTION;

export interface PatrolPageHeaderMetaInput {
  autonomyLevel?: PatrolAutonomyLevel;
  autonomyLocked?: boolean;
}

function getPatrolPageDescription(input: PatrolPageHeaderMetaInput): string {
  if (input.autonomyLocked) {
    return PATROL_PAGE_WATCH_ONLY_DESCRIPTION;
  }
  if (input.autonomyLevel === 'monitor') return PATROL_PAGE_MONITOR_DESCRIPTION;

  switch (input.autonomyLevel) {
    case 'approval':
      return 'Ask first: Patrol investigates and prepares fixes, but every change waits for approval.';
    case 'assisted':
      return 'Safe auto-fix: Patrol may run low- or medium-risk fixes allowed by policy.';
    case 'full':
      return 'Autopilot: Patrol may act automatically within policy and still asks when approval is required.';
    default:
      return PATROL_PAGE_DESCRIPTION;
  }
}

export function getPatrolPageHeaderMeta(input: PatrolPageHeaderMetaInput = {}) {
  const description = getPatrolPageDescription(input);
  return {
    title: PATROL_PAGE_TITLE,
    description,
    titleTooltip: description,
  } as const;
}
