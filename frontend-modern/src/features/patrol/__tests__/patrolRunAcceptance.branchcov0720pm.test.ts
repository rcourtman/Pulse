import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { PatrolRunRecord, PatrolStatus } from '@/api/patrol';
import {
  schedulePatrolRunAcceptanceReconciliation,
  type PatrolRunAcceptanceOutcome,
} from '../patrolRunAcceptance';

const runningStatus = (currentRunId?: string): PatrolStatus =>
  ({
    runtime_state: 'running',
    running: true,
    enabled: true,
    ...(currentRunId !== undefined ? { current_run_id: currentRunId } : {}),
  }) as PatrolStatus;

const idleStatus = (): PatrolStatus =>
  ({ runtime_state: 'active', running: false, enabled: true }) as PatrolStatus;

const completedRun = (id: string): PatrolRunRecord =>
  ({ id, status: 'healthy' }) as PatrolRunRecord;

interface Scenario {
  runId?: string;
  delayMs?: number;
  refreshTimeoutMs?: number;
  isCurrent?: () => boolean;
  getStatus: () => Promise<PatrolStatus | null>;
  getHistory: () => Promise<PatrolRunRecord[]>;
}

const scheduleScenario = (scenario: Scenario) => {
  const onResult = vi.fn<(outcome: PatrolRunAcceptanceOutcome) => void>();
  const cancel = schedulePatrolRunAcceptanceReconciliation({
    runId: scenario.runId ?? 'run-1',
    delayMs: scenario.delayMs ?? 1_000,
    refreshTimeoutMs: scenario.refreshTimeoutMs ?? 5_000,
    getStatus: scenario.getStatus,
    getHistory: scenario.getHistory,
    isCurrent: scenario.isCurrent ?? (() => true),
    onResult,
  });
  return { onResult, cancel };
};

describe('schedulePatrolRunAcceptanceReconciliation — branch coverage', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("kind 'running'", () => {
    it('accepts when status.current_run_id === runId', async () => {
      const { onResult } = scheduleScenario({
        getStatus: async () => runningStatus('run-1'),
        getHistory: async () => [],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      const outcome = onResult.mock.calls[0][0];
      expect(outcome.kind).toBe('running');
      expect((outcome as { status: PatrolStatus }).status.current_run_id).toBe('run-1');
    });

    it('accepts when runId is empty (no specific run tracked)', async () => {
      const { onResult } = scheduleScenario({
        runId: '',
        getStatus: async () => runningStatus('run-1'),
        getHistory: async () => [],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      expect(onResult.mock.calls[0][0].kind).toBe('running');
    });

    it('accepts when status has no current_run_id', async () => {
      const { onResult } = scheduleScenario({
        getStatus: async () => runningStatus(undefined),
        getHistory: async () => [],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      expect(onResult.mock.calls[0][0].kind).toBe('running');
    });
  });

  describe("kind 'recorded'", () => {
    it('records when patrol is idle but history contains the accepted run', async () => {
      const { onResult } = scheduleScenario({
        getStatus: async () => idleStatus(),
        getHistory: async () => [completedRun('run-1')],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      const outcome = onResult.mock.calls[0][0];
      expect(outcome.kind).toBe('recorded');
      expect((outcome as { run: PatrolRunRecord }).run.id).toBe('run-1');
    });

    it('records when a newer run has taken over the patrol slot', async () => {
      const { onResult } = scheduleScenario({
        getStatus: async () => runningStatus('run-newer'),
        getHistory: async () => [completedRun('run-1')],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledWith(
        expect.objectContaining({
          kind: 'recorded',
          run: expect.objectContaining({ id: 'run-1' }),
        }),
      );
    });
  });

  describe("kind 'refresh_failed'", () => {
    it('reports the status read rejection reason', async () => {
      const statusError = new Error('status down');
      const { onResult } = scheduleScenario({
        getStatus: () => Promise.reject(statusError),
        getHistory: async () => [],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      const outcome = onResult.mock.calls[0][0];
      expect(outcome.kind).toBe('refresh_failed');
      expect((outcome as { error: unknown }).error).toBe(statusError);
    });

    it('reports the history read rejection reason', async () => {
      const historyError = new Error('history down');
      const { onResult } = scheduleScenario({
        getStatus: async () => idleStatus(),
        getHistory: () => Promise.reject(historyError),
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      const outcome = onResult.mock.calls[0][0];
      expect(outcome.kind).toBe('refresh_failed');
      expect((outcome as { error: unknown }).error).toBe(historyError);
    });

    it('bounds hung reads with the refresh timeout', async () => {
      const { onResult } = scheduleScenario({
        delayMs: 1_000,
        refreshTimeoutMs: 2_000,
        getStatus: () => new Promise<PatrolStatus | null>(() => undefined),
        getHistory: () => new Promise<PatrolRunRecord[]>(() => undefined),
      });
      await vi.advanceTimersByTimeAsync(3_000);
      expect(onResult).toHaveBeenCalledTimes(1);
      expect(onResult.mock.calls[0][0].kind).toBe('refresh_failed');
    });
  });

  describe("kind 'missing'", () => {
    it('falls through when idle, history lacks the run, and no refresh failed', async () => {
      const { onResult } = scheduleScenario({
        getStatus: async () => idleStatus(),
        getHistory: async () => [completedRun('run-other')],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledWith({ kind: 'missing' });
    });

    it('falls through to missing when no runId is tracked and nothing is running', async () => {
      const { onResult } = scheduleScenario({
        runId: '',
        getStatus: async () => idleStatus(),
        getHistory: async () => [completedRun('run-1')],
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(onResult).toHaveBeenCalledWith({ kind: 'missing' });
    });
  });

  describe('guard branches', () => {
    it('suppresses onResult when isCurrent() is false after the delay', async () => {
      const getStatus = vi.fn(async () => runningStatus('run-1'));
      const getHistory = vi.fn(async () => [] as PatrolRunRecord[]);
      const { onResult } = scheduleScenario({
        getStatus,
        getHistory,
        isCurrent: () => false,
      });
      await vi.advanceTimersByTimeAsync(1_000);
      expect(getStatus).toHaveBeenCalledTimes(1);
      expect(getHistory).toHaveBeenCalledTimes(1);
      expect(onResult).not.toHaveBeenCalled();
    });

    it('cancels before the delay fires: clears the timer and skips the reads', async () => {
      const getStatus = vi.fn(async () => runningStatus('run-1'));
      const getHistory = vi.fn(async () => [] as PatrolRunRecord[]);
      const { onResult, cancel } = scheduleScenario({ getStatus, getHistory });
      cancel();
      await vi.advanceTimersByTimeAsync(10_000);
      expect(getStatus).not.toHaveBeenCalled();
      expect(getHistory).not.toHaveBeenCalled();
      expect(onResult).not.toHaveBeenCalled();
    });
  });
});
