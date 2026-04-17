export const PATROL_PAGE_TITLE = 'Patrol';
export const PATROL_PAGE_DESCRIPTION =
  'Continuously verify infrastructure health, review findings, and control Patrol runtime behavior.';

export function getPatrolPageHeaderMeta() {
  return {
    title: PATROL_PAGE_TITLE,
    description: PATROL_PAGE_DESCRIPTION,
  } as const;
}
