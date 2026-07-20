import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { PatrolRunRecord, PatrolStatus } from '@/api/patrol';
import {
  schedulePatrolRunAcceptanceReconciliation,
  type PatrolRunAcceptanceOutcome,
} from '../patrolRunAcceptance';

const runningStatus = (runId: string): PatrolStatus =>
  ({
    runtime_state: 'running',
    running: true,
    current_run_id: runId,
    enabled: true,
  }) as PatrolStatus;

const completedRun = (runId: string, status: 'healthy' | 'error' = 'healthy'): PatrolRunRecord =>
  ({
    id: runId,
    status,
    error_count: status === 'error' ? 1 : 0,
  }) as PatrolRunRecord;

describe('Patrol accepted-run reconciliation', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  for (const firstProviderEventMs of [0, 15_000, 30_000, 60_000]) {
    it(`keeps one accepted run authoritative when the first provider event arrives at ${firstProviderEventMs / 1000}s`, async () => {
      const runId = `run-${firstProviderEventMs}`;
      let providerEventObserved = false;
      const history: PatrolRunRecord[] = [];
      const errorToast = vi.fn();
      const onResult = vi.fn<(outcome: PatrolRunAcceptanceOutcome) => void>((outcome) => {
        if (outcome.kind === 'missing') errorToast();
      });
      const getStatus = vi.fn(async () =>
        history.length > 0
          ? ({ ...runningStatus(runId), runtime_state: 'active', running: false } as PatrolStatus)
          : runningStatus(runId),
      );
      const getHistory = vi.fn(async () => [...history]);
      let providerCalls = 0;

      providerCalls += 1;
      setTimeout(() => {
        providerEventObserved = true;
      }, firstProviderEventMs);
      setTimeout(() => {
        history.push(completedRun(runId));
      }, firstProviderEventMs + 1_000);

      schedulePatrolRunAcceptanceReconciliation({
        runId,
        delayMs: 15_000,
        refreshTimeoutMs: 5_000,
        getStatus,
        getHistory,
        isCurrent: () => true,
        onResult,
      });

      await vi.advanceTimersByTimeAsync(15_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      expect(onResult.mock.calls[0][0].kind).toMatch(/running|recorded/);
      expect(onResult).not.toHaveBeenCalledWith(expect.objectContaining({ kind: 'missing' }));
      expect(errorToast).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(Math.max(0, firstProviderEventMs + 1_000 - 15_000));
      expect(providerCalls).toBe(1);
      expect(providerEventObserved).toBe(true);
      expect(history).toHaveLength(1);
      expect(history[0].id).toBe(runId);
      expect(getStatus).toHaveBeenCalledTimes(1);
      expect(getHistory).toHaveBeenCalledTimes(1);
    });
  }

  it('preserves a provider/runtime failure as a recorded run instead of a start failure', async () => {
    const onResult = vi.fn<(outcome: PatrolRunAcceptanceOutcome) => void>();
    schedulePatrolRunAcceptanceReconciliation({
      runId: 'run-provider-error',
      delayMs: 15_000,
      refreshTimeoutMs: 5_000,
      getStatus: async () => ({ ...runningStatus('run-provider-error'), running: false }),
      getHistory: async () => [completedRun('run-provider-error', 'error')],
      isCurrent: () => true,
      onResult,
    });

    await vi.advanceTimersByTimeAsync(15_000);
    expect(onResult).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'recorded',
        run: expect.objectContaining({ id: 'run-provider-error', status: 'error' }),
      }),
    );
  });

  it('bounds hung reconciliation reads and reports a refresh failure', async () => {
    const onResult = vi.fn<(outcome: PatrolRunAcceptanceOutcome) => void>();
    schedulePatrolRunAcceptanceReconciliation({
      runId: 'run-hung-read',
      delayMs: 15_000,
      refreshTimeoutMs: 5_000,
      getStatus: () => new Promise(() => undefined),
      getHistory: () => new Promise(() => undefined),
      isCurrent: () => true,
      onResult,
    });

    await vi.advanceTimersByTimeAsync(20_000);
    expect(onResult).toHaveBeenCalledTimes(1);
    expect(onResult.mock.calls[0][0].kind).toBe('refresh_failed');
  });

  it('cancels without reading status or history and permits a clean retry', async () => {
    const firstStatus = vi.fn(async () => runningStatus('run-cancelled'));
    const firstHistory = vi.fn(async () => [] as PatrolRunRecord[]);
    const firstResult = vi.fn<(outcome: PatrolRunAcceptanceOutcome) => void>();
    const cancel = schedulePatrolRunAcceptanceReconciliation({
      runId: 'run-cancelled',
      delayMs: 15_000,
      refreshTimeoutMs: 5_000,
      getStatus: firstStatus,
      getHistory: firstHistory,
      isCurrent: () => true,
      onResult: firstResult,
    });
    cancel();

    await vi.advanceTimersByTimeAsync(15_000);
    expect(firstStatus).not.toHaveBeenCalled();
    expect(firstHistory).not.toHaveBeenCalled();
    expect(firstResult).not.toHaveBeenCalled();

    const retryResult = vi.fn<(outcome: PatrolRunAcceptanceOutcome) => void>();
    schedulePatrolRunAcceptanceReconciliation({
      runId: 'run-retry',
      delayMs: 15_000,
      refreshTimeoutMs: 5_000,
      getStatus: async () => runningStatus('run-retry'),
      getHistory: async () => [],
      isCurrent: () => true,
      onResult: retryResult,
    });
    await vi.advanceTimersByTimeAsync(15_000);
    expect(retryResult).toHaveBeenCalledWith(expect.objectContaining({ kind: 'running' }));
  });
});
