import type { PatrolAutonomyLevel } from '@/api/patrol';

export const PATROL_PAGE_TITLE = 'Patrol';

export const PATROL_PAGE_WATCH_ONLY_DESCRIPTION =
  'Patrol watches infrastructure and shows current issues.';

export const PATROL_PAGE_MONITOR_DESCRIPTION =
  'Watch only: Patrol reports issues without making changes.';

export const PATROL_PAGE_DESCRIPTION =
  'Patrol watches, investigates, acts within your chosen mode, verifies outcomes, and records what happened.';
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
      return 'Patrol investigates issues and asks before every change.';
    case 'assisted':
      return 'Patrol handles safe policy-allowed fixes and asks before anything riskier.';
    case 'full':
      return 'Patrol handles policy-approved work automatically and asks only when approval is required.';
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
