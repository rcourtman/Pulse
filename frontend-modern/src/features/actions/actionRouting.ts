import { ACTIONS_PATH } from '@/routing/resourceLinks';

export const ACTION_REVIEW_QUERY_PARAM = 'action';

const normalizeActionId = (value: string | null | undefined): string => (value || '').trim();

export function buildActionReviewPath(actionId?: string | null): string {
  const normalizedActionId = normalizeActionId(actionId);
  if (!normalizedActionId) return ACTIONS_PATH;

  const search = new URLSearchParams({ [ACTION_REVIEW_QUERY_PARAM]: normalizedActionId });
  return `${ACTIONS_PATH}?${search.toString()}`;
}

export function parseActionReviewId(search: string): string {
  return normalizeActionId(new URLSearchParams(search).get(ACTION_REVIEW_QUERY_PARAM));
}
