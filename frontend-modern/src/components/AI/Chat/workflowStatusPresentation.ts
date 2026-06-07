import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  untrack,
  type Accessor,
} from 'solid-js';
import { formatAssistantWorkflowStatus } from './activeTurnStatus';
import type { WorkflowStatus } from './types';

export const ASSISTANT_WORKFLOW_STATUS_BURST_VISIBLE_MS = 650;

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

interface PacedWorkflowStatusOptions {
  enabled?: Accessor<boolean>;
  visibleMs?: number;
}

interface PacedWorkflowStatus {
  sequence: Accessor<WorkflowStatus[]>;
  status: Accessor<WorkflowStatus | undefined>;
  pacing: Accessor<boolean>;
}

export const createPacedWorkflowStatus = (
  getStatuses: Accessor<Array<WorkflowStatus | undefined>>,
  options: PacedWorkflowStatusOptions = {},
): PacedWorkflowStatus => {
  const sequence = createMemo(() => normalizeWorkflowStatusSequence(getStatuses()));
  const [displayedStatusKey, setDisplayedStatusKey] = createSignal('');
  const [pacing, setPacing] = createSignal(false);
  const visibleMs = options.visibleMs ?? ASSISTANT_WORKFLOW_STATUS_BURST_VISIBLE_MS;
  let lastSequenceFingerprint = '';
  let lastSequenceLength = 0;
  let advanceTimer: number | undefined;

  const clearAdvanceTimer = () => {
    if (advanceTimer !== undefined) {
      window.clearTimeout(advanceTimer);
      advanceTimer = undefined;
    }
  };

  const showLatest = (keys: string[]) => {
    clearAdvanceTimer();
    setDisplayedStatusKey(keys[keys.length - 1] || '');
    setPacing(false);
  };

  const scheduleAdvance = (keys: string[], currentIndex: number) => {
    clearAdvanceTimer();
    if (currentIndex >= keys.length - 1) {
      setPacing(false);
      return;
    }

    setPacing(true);
    advanceTimer = window.setTimeout(() => {
      const nextIndex = Math.min(currentIndex + 1, keys.length - 1);
      setDisplayedStatusKey(keys[nextIndex] || '');
      scheduleAdvance(keys, nextIndex);
    }, visibleMs);
  };

  createEffect(() => {
    const statuses = sequence();
    const keys = statuses.map(workflowStatusRenderKey);
    const fingerprint = keys.join('\u001e');
    const previousFingerprint = lastSequenceFingerprint;
    const previousLength = lastSequenceLength;
    const sequenceChanged = fingerprint !== previousFingerprint;
    const addedCount = sequenceChanged ? Math.max(0, statuses.length - previousLength) : 0;
    const enabled = options.enabled?.() ?? true;

    lastSequenceFingerprint = fingerprint;
    lastSequenceLength = statuses.length;

    if (statuses.length === 0) {
      clearAdvanceTimer();
      setDisplayedStatusKey('');
      setPacing(false);
      return;
    }

    if (!enabled) {
      showLatest(keys);
      return;
    }

    const currentKey = untrack(displayedStatusKey);
    const currentIndex = keys.indexOf(currentKey);
    if (currentIndex === -1) {
      const initialIndex = statuses.length > 1 ? 0 : statuses.length - 1;
      setDisplayedStatusKey(keys[initialIndex] || '');
      if (initialIndex < statuses.length - 1) {
        scheduleAdvance(keys, initialIndex);
      } else {
        clearAdvanceTimer();
        setPacing(false);
      }
      return;
    }

    if (currentIndex >= statuses.length - 1) {
      clearAdvanceTimer();
      setPacing(false);
      return;
    }

    const pendingCount = statuses.length - currentIndex - 1;
    const shouldPace = untrack(pacing) || addedCount > 1 || pendingCount > 1;
    if (shouldPace) {
      scheduleAdvance(keys, currentIndex);
      return;
    }

    showLatest(keys);
  });

  onCleanup(clearAdvanceTimer);

  const status = createMemo<WorkflowStatus | undefined>(() => {
    const statuses = sequence();
    const key = displayedStatusKey();
    if (key) {
      const matched = statuses.find((candidate) => workflowStatusRenderKey(candidate) === key);
      if (matched) return matched;
    }
    return latestWorkflowStatus(statuses);
  });

  return {
    sequence,
    status,
    pacing,
  };
};
