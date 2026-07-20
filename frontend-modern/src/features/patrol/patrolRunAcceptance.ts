import type { PatrolRunRecord, PatrolStatus } from '@/api/patrol';

export type PatrolRunAcceptanceOutcome =
  | { kind: 'running'; status: PatrolStatus }
  | { kind: 'recorded'; run: PatrolRunRecord }
  | { kind: 'missing' }
  | { kind: 'refresh_failed'; error: unknown };

interface PatrolRunAcceptanceReconciliationOptions {
  runId: string;
  delayMs: number;
  refreshTimeoutMs: number;
  getStatus: () => Promise<PatrolStatus | null>;
  getHistory: () => Promise<PatrolRunRecord[]>;
  isCurrent: () => boolean;
  onResult: (outcome: PatrolRunAcceptanceOutcome) => void;
}

const timeoutError = () => new Error('Patrol acceptance reconciliation timed out');

async function settleReconciliationReads(
  getStatus: () => Promise<PatrolStatus | null>,
  getHistory: () => Promise<PatrolRunRecord[]>,
  timeoutMs: number,
): Promise<
  | {
      status: PromiseSettledResult<PatrolStatus | null>;
      history: PromiseSettledResult<PatrolRunRecord[]>;
    }
  | { error: unknown }
> {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      Promise.allSettled([getStatus(), getHistory()]).then(([status, history]) => ({
        status,
        history,
      })),
      new Promise<{ error: unknown }>((resolve) => {
        timeout = setTimeout(() => resolve({ error: timeoutError() }), timeoutMs);
      }),
    ]);
  } finally {
    if (timeout !== undefined) clearTimeout(timeout);
  }
}

export function schedulePatrolRunAcceptanceReconciliation(
  options: PatrolRunAcceptanceReconciliationOptions,
): () => void {
  let cancelled = false;
  const timer = setTimeout(() => {
    void settleReconciliationReads(
      options.getStatus,
      options.getHistory,
      options.refreshTimeoutMs,
    ).then((reads) => {
      if (cancelled || !options.isCurrent()) return;
      if ('error' in reads) {
        options.onResult({ kind: 'refresh_failed', error: reads.error });
        return;
      }

      const status = reads.status.status === 'fulfilled' ? reads.status.value : null;
      const history = reads.history.status === 'fulfilled' ? reads.history.value : null;
      const statusOwnsAcceptedRun =
        status?.running === true &&
        (!options.runId || !status.current_run_id || status.current_run_id === options.runId);
      if (statusOwnsAcceptedRun) {
        options.onResult({ kind: 'running', status });
        return;
      }

      const recordedRun = options.runId
        ? history?.find((run) => run.id === options.runId)
        : undefined;
      if (recordedRun) {
        options.onResult({ kind: 'recorded', run: recordedRun });
        return;
      }

      const refreshError =
        reads.status.status === 'rejected'
          ? reads.status.reason
          : reads.history.status === 'rejected'
            ? reads.history.reason
            : undefined;
      if (refreshError !== undefined) {
        options.onResult({ kind: 'refresh_failed', error: refreshError });
        return;
      }
      options.onResult({ kind: 'missing' });
    });
  }, options.delayMs);

  return () => {
    cancelled = true;
    clearTimeout(timer);
  };
}
