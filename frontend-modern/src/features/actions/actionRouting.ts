import { ACTIONS_PATH } from '@/routing/resourceLinks';
import type { ActionInboxView } from '@/types/actionAudit';

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

// The Open and History views are routes, so a bookmark or shared link opens
// the view it was copied from and back/forward moves between them.
export const ACTIONS_HISTORY_PATH = `${ACTIONS_PATH}/history`;

export function buildActionsViewPath(view: ActionInboxView): string {
  return view === 'settled' ? ACTIONS_HISTORY_PATH : ACTIONS_PATH;
}

export function parseActionsView(pathname: string): ActionInboxView {
  return /^\/actions\/history\/?$/.test(pathname) ? 'settled' : 'pending';
}
