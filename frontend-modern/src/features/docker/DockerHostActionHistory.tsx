import { Show, createMemo, type Component } from 'solid-js';

import { ActionAuditAPI } from '@/api/actionAudit';
import { ResourceActionHistory } from '@/components/Infrastructure/ResourceActionHistory';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import type { ActionAuditListResponse } from '@/types/actionAudit';
import { getAPIReadAccessErrorMessage } from '@/utils/apiAccessError';

type ActionAuditRequest = { id: string; limit: number };

const timestampOf = (value: string | undefined): number => {
  const parsed = Date.parse(value || '');
  return Number.isFinite(parsed) ? parsed : 0;
};

/**
 * The host's recent audited actions, read the way a resource drawer's Manage
 * tab reads them. It mounts with Manage, so nothing is fetched until Manage
 * opens, and it stays hidden where action history is not available.
 */
export const DockerHostActionHistory: Component<{ resourceId: string }> = (props) => {
  const request = createMemo((): ActionAuditRequest | null => {
    const id = props.resourceId.trim();
    return id ? { id, limit: 5 } : null;
  });
  const state = createNonSuspendingQuery<ActionAuditListResponse, ActionAuditRequest>({
    source: request,
    fetcher: async (current) =>
      ActionAuditAPI.listActionAudits({ resourceId: current.id, limit: current.limit }),
    initialValue: { audits: [], count: 0, available: false },
  });
  const audits = createMemo(() =>
    [...(state.value().audits ?? [])].sort(
      (left, right) =>
        timestampOf(right.updatedAt || right.createdAt) -
        timestampOf(left.updatedAt || left.createdAt),
    ),
  );
  const error = createMemo(() => {
    const failure = state.error();
    if (!failure) return '';
    return (
      getAPIReadAccessErrorMessage(failure) ||
      (failure as Error)?.message ||
      'Failed to load action history'
    );
  });
  const loadingLabel = () =>
    state.loading()
      ? 'Refreshing actions...'
      : state.error()
        ? 'Actions unavailable'
        : 'Actions loaded';

  return (
    <Show when={state.value().available || audits().length > 0 || error()}>
      <ResourceActionHistory
        audits={audits()}
        count={state.value().count ?? audits().length}
        loadingLabel={loadingLabel()}
        error={error()}
        onRetry={state.refetch}
      />
    </Show>
  );
};
