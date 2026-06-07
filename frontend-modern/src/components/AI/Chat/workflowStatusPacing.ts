import type { StreamDisplayEvent, WorkflowStatus } from './types';

export const WORKFLOW_STATUS_PACE_MS = 900;
const WORKFLOW_STATUS_BURST_GAP_MS = 900;

const finiteTime = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const workflowStatusesMatch = (a?: WorkflowStatus, b?: WorkflowStatus) =>
  !!a &&
  !!b &&
  (a.phase || '') === (b.phase || '') &&
  a.message === b.message &&
  (a.state || '') === (b.state || '') &&
  (a.tool || '') === (b.tool || '') &&
  (a.provider || '') === (b.provider || '') &&
  (a.model || '') === (b.model || '') &&
  (a.attempt || 0) === (b.attempt || 0) &&
  (a.maxAttempts || 0) === (b.maxAttempts || 0) &&
  (a.retryAfterMs || 0) === (b.retryAfterMs || 0);

const isDurableStreamBoundary = (event: StreamDisplayEvent) =>
  event.type !== 'workflow_status' && event.type !== 'model_switch';

const immediateWorkflowStatusPhases = new Set(['provider_retry', 'stream_idle']);

const isImmediateWorkflowStatus = (status?: WorkflowStatus) => {
  const phase = status?.phase?.trim().toLowerCase();
  return (
    (phase ? immediateWorkflowStatusPhases.has(phase) : false) ||
    Boolean(status?.attempt || status?.retryAfterMs)
  );
};

const latestWorkflowEventIndex = (
  events: StreamDisplayEvent[] | undefined,
  currentStatus: WorkflowStatus | undefined,
): number => {
  for (let index = (events?.length || 0) - 1; index >= 0; index -= 1) {
    const event = events?.[index];
    if (event?.type !== 'workflow_status') continue;
    if (!currentStatus || workflowStatusesMatch(event.workflowStatus, currentStatus)) {
      return index;
    }
  }
  return -1;
};

const hasEarlierDurableBoundary = (
  events: StreamDisplayEvent[] | undefined,
  workflowIndex: number,
) => {
  if (!events || workflowIndex < 0) return false;
  for (let index = 0; index < workflowIndex; index += 1) {
    if (isDurableStreamBoundary(events[index])) return true;
  }
  return false;
};

const burstHistoryUpTo = (
  history: WorkflowStatus[] | undefined,
  currentStatus: WorkflowStatus | undefined,
): WorkflowStatus[] => {
  if (!history?.length || !currentStatus) return [];

  let currentIndex = -1;
  for (let index = history.length - 1; index >= 0; index -= 1) {
    if (workflowStatusesMatch(history[index], currentStatus)) {
      currentIndex = index;
      break;
    }
  }
  if (currentIndex <= 0) return [];

  const segment = history.slice(0, currentIndex + 1);
  for (let index = 1; index < segment.length; index += 1) {
    const previousAt = finiteTime(segment[index - 1].startedAt);
    const currentAt = finiteTime(segment[index].startedAt);
    if (previousAt === undefined || currentAt === undefined) return [];
    const gap = currentAt - previousAt;
    if (gap < 0 || gap > WORKFLOW_STATUS_BURST_GAP_MS) return [];
  }
  return segment;
};

export const pacedWorkflowStatusForDisplay = (
  history: WorkflowStatus[] | undefined,
  currentStatus: WorkflowStatus | undefined,
  events: StreamDisplayEvent[] | undefined,
  now?: number,
): WorkflowStatus | undefined => {
  if (isImmediateWorkflowStatus(currentStatus)) return currentStatus;

  const workflowIndex = latestWorkflowEventIndex(events, currentStatus);
  if (workflowIndex >= 0 && hasEarlierDurableBoundary(events, workflowIndex)) {
    return currentStatus;
  }

  const segment = burstHistoryUpTo(history, currentStatus);
  if (segment.length <= 1) return currentStatus;

  const start = finiteTime(segment[0].startedAt);
  const currentTime = finiteTime(now);
  if (start === undefined || currentTime === undefined) return currentStatus;

  const elapsed = Math.max(0, currentTime - start);
  const pacedIndex = Math.min(segment.length - 1, Math.floor(elapsed / WORKFLOW_STATUS_PACE_MS));
  return segment[pacedIndex] || currentStatus;
};
