import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';
import type { StreamDisplayEvent, WorkflowStatus } from './types';

export const WORKFLOW_STATUS_REFRESH_MS = 300;
export const WORKFLOW_STATUS_PACE_MS = 380;

const workflowStatusDisplayKey = (status?: WorkflowStatus): string =>
  [
    status?.phase,
    status?.message,
    status?.state,
    status?.tool,
    status?.provider,
    status?.model,
    status?.attempt,
    status?.maxAttempts,
    status?.retryAfterMs,
    status?.startedAt,
  ]
    .map((value) => String(value ?? ''))
    .join('\u001f');

const normalizeWorkflowStatusHistory = (statuses?: WorkflowStatus[]): WorkflowStatus[] => {
  const normalized: WorkflowStatus[] = [];
  for (const status of statuses || []) {
    if (!status?.message?.trim()) continue;
    const previous = normalized[normalized.length - 1];
    if (workflowStatusDisplayKey(previous) === workflowStatusDisplayKey(status)) continue;
    normalized.push(status);
  }
  return normalized;
};

export const createPacedWorkflowStatus = (
  statuses: Accessor<WorkflowStatus[] | undefined>,
  live: Accessor<boolean>,
  sequenceKey: Accessor<string> = () => '',
): Accessor<WorkflowStatus | undefined> => {
  const history = createMemo(() => normalizeWorkflowStatusHistory(statuses()));
  const [displayIndex, setDisplayIndex] = createSignal(0);
  let previousSequenceKey = '';

  createEffect(() => {
    const currentHistory = history();
    const currentSequenceKey = sequenceKey();
    const isLive = live();

    if (currentHistory.length === 0) {
      previousSequenceKey = currentSequenceKey;
      setDisplayIndex(0);
      return;
    }

    if (!isLive || currentHistory.length === 1) {
      previousSequenceKey = currentSequenceKey;
      setDisplayIndex(currentHistory.length - 1);
      return;
    }

    if (currentSequenceKey !== previousSequenceKey) {
      previousSequenceKey = currentSequenceKey;
      setDisplayIndex(0);
      return;
    }

    setDisplayIndex((index) => Math.min(Math.max(index, 0), currentHistory.length - 1));
  });

  createEffect(() => {
    const currentHistory = history();
    const index = displayIndex();
    if (!live() || currentHistory.length <= 1 || index >= currentHistory.length - 1) return;

    const timer = window.setTimeout(() => {
      setDisplayIndex((current) => Math.min(current + 1, history().length - 1));
    }, WORKFLOW_STATUS_PACE_MS);
    onCleanup(() => window.clearTimeout(timer));
  });

  return () => {
    const currentHistory = history();
    if (currentHistory.length === 0) return undefined;
    const index = Math.min(Math.max(displayIndex(), 0), currentHistory.length - 1);
    return currentHistory[index];
  };
};

export const replaceLatestWorkflowStatusEventForDisplay = (
  events: StreamDisplayEvent[] | undefined,
  workflowStatus: WorkflowStatus | undefined,
): StreamDisplayEvent[] | undefined => {
  if (!events || !workflowStatus) return events;

  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index];
    if (event.type !== 'workflow_status') continue;
    return [
      ...events.slice(0, index),
      {
        ...event,
        workflowStatus,
        startedAt: workflowStatus.startedAt ?? event.startedAt,
        updatedAt: workflowStatus.startedAt ?? event.updatedAt,
      },
      ...events.slice(index + 1),
    ];
  }

  return events;
};
