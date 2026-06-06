import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { formatAssistantWorkflowStatus } from './activeTurnStatus';
import type { WorkflowStatus } from './types';

export const WORKFLOW_STATUS_RENDER_PACE_MS = 420;

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

interface PacedWorkflowStatusClock {
  firstKey: string;
  startedAt: number;
}

const pacedWorkflowStatusCache = new Map<string, PacedWorkflowStatusClock>();
const pacedWorkflowStatusCleanupTimers = new Map<string, ReturnType<typeof setTimeout>>();

const cancelPacedWorkflowStatusCleanup = (key: string) => {
  const timer = pacedWorkflowStatusCleanupTimers.get(key);
  if (!timer) return;
  clearTimeout(timer);
  pacedWorkflowStatusCleanupTimers.delete(key);
};

const deletePacedWorkflowStatusCache = (key: string) => {
  cancelPacedWorkflowStatusCleanup(key);
  pacedWorkflowStatusCache.delete(key);
};

const schedulePacedWorkflowStatusCleanup = (key: string) => {
  cancelPacedWorkflowStatusCleanup(key);
  pacedWorkflowStatusCleanupTimers.set(
    key,
    setTimeout(() => {
      pacedWorkflowStatusCleanupTimers.delete(key);
      pacedWorkflowStatusCache.delete(key);
    }, 1000),
  );
};

export const createPacedWorkflowStatus = (
  getStatuses: () => WorkflowStatus[],
  live: () => boolean,
  cacheKey: () => string,
) => {
  const [now, setNow] = createSignal(Date.now());
  let interval: ReturnType<typeof setInterval> | undefined;

  createEffect(() => {
    const statuses = getStatuses();
    if (!live()) {
      if (interval) {
        clearInterval(interval);
        interval = undefined;
      }
      const key = cacheKey();
      if (key) deletePacedWorkflowStatusCache(key);
      return;
    }

    const key = cacheKey();
    if (key) {
      cancelPacedWorkflowStatusCleanup(key);
    }

    if (statuses.length > 1 && !interval) {
      interval = setInterval(() => setNow(Date.now()), WORKFLOW_STATUS_RENDER_PACE_MS);
    } else if (statuses.length <= 1 && interval) {
      clearInterval(interval);
      interval = undefined;
    }
  });

  onCleanup(() => {
    if (interval) {
      clearInterval(interval);
    }
    const key = cacheKey();
    if (key) schedulePacedWorkflowStatusCleanup(key);
  });

  return createMemo(() => {
    const statuses = getStatuses();
    if (statuses.length === 0) return undefined;
    if (!live()) return statuses[statuses.length - 1];
    if (statuses.length === 1) return statuses[0];

    const key = cacheKey();
    const firstKey = workflowStatusRenderKey(statuses[0]);
    if (!key || !firstKey) return statuses[0];

    const cached = pacedWorkflowStatusCache.get(key);
    const startedAt = cached?.firstKey === firstKey ? cached.startedAt : Date.now();
    if (!cached || cached.firstKey !== firstKey) {
      pacedWorkflowStatusCache.set(key, { firstKey, startedAt });
    }
    const currentTime = Math.max(now(), startedAt);

    const index = Math.min(
      statuses.length - 1,
      Math.floor(Math.max(0, currentTime - startedAt) / WORKFLOW_STATUS_RENDER_PACE_MS),
    );
    return statuses[index] || statuses[statuses.length - 1];
  });
};
