import { formatAssistantWorkflowStatus } from './activeTurnStatus';
import type { WorkflowStatus } from './types';

export const workflowStatusRenderKey = (status?: WorkflowStatus) =>
  status
    ? [
        status?.phase || '',
        status?.message || '',
        status?.state || '',
        status?.tool || '',
        status?.attempt || 0,
        status?.maxAttempts || 0,
        status?.retryAfterMs || 0,
        status?.startedAt || 0,
      ].join('\u001f')
    : '';

export const normalizeWorkflowStatusSequence = (statuses: Array<WorkflowStatus | undefined>) => {
  const sequence: WorkflowStatus[] = [];
  for (const status of statuses) {
    if (!status || !formatAssistantWorkflowStatus(status)) continue;
    const previous = sequence[sequence.length - 1];
    if (workflowStatusRenderKey(previous) === workflowStatusRenderKey(status)) continue;
    sequence.push(status);
  }
  return sequence;
};

export const latestWorkflowStatus = (statuses: WorkflowStatus[]) =>
  statuses.length > 0 ? statuses[statuses.length - 1] : undefined;
