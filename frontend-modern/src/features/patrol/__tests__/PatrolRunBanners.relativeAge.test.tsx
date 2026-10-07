import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { PatrolRunRecord } from '@/api/patrol';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { PatrolIntelligenceBanners } from '../PatrolIntelligenceBanners';
import { PatrolIntelligenceWorkspace } from '../PatrolIntelligenceWorkspace';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

vi.mock('@/components/AI/FindingsPanel', () => ({
  FindingsPanel: () => <div data-testid="findings-panel" />,
}));

vi.mock('@/components/patrol/RunHistoryPanel', () => ({
  RunHistoryPanel: () => <div data-testid="run-history-panel" />,
}));

// Both surfaces read many accessors; the ones a test does not set answer
// with nothing, and list-valued ones with an empty list.
const LIST_ACCESSORS = new Set([
  'findingsTabBadgeFindings',
  'displayRunHistory',
  'selectedRunFindingIds',
  'selectedRunScopeResourceIds',
]);
const stubState = (overrides: Record<string, unknown>): PatrolIntelligenceState =>
  new Proxy(overrides, {
    get(target, key: string) {
      if (key in target) return target[key];
      if (key === 'patrolStream') {
        return new Proxy({}, { get: () => () => undefined });
      }
      if (key === 'patrolRunHistory') return { value: () => [], refetch: vi.fn() };
      return LIST_ACCESSORS.has(key) ? () => [] : () => undefined;
    },
  }) as unknown as PatrolIntelligenceState;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('Patrol run and pause banners relative ages', () => {
  it('keeps the Patrol paused Blocked age moving while Patrol stays paused', () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-07-19T08:05:00Z'),
    });

    render(() => (
      <PatrolIntelligenceBanners
        state={stubState({
          showBlockedBanner: () => true,
          blockedReason: () => 'The monthly AI budget is used up.',
          blockedAt: () => '2026-07-19T08:00:00Z',
        })}
      />
    ));

    expect(screen.getByText('Blocked 5m ago')).toBeInTheDocument();

    // Each status re-read returns the same blocked time: only the clock moves.
    vi.advanceTimersByTime(3 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(screen.getByText('Blocked 3h ago')).toBeInTheDocument();
  });

  it('keeps the selected Patrol run start age moving while the run stays selected', () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-07-19T08:05:00Z'),
    });
    const run = {
      id: 'run-1',
      started_at: '2026-07-19T08:00:00Z',
      trigger_reason: 'scheduled',
      status: 'healthy',
      finding_ids: [],
    } as unknown as PatrolRunRecord;

    render(() => (
      <PatrolIntelligenceWorkspace
        state={stubState({
          activeTab: () => 'findings',
          selectedRun: () => run,
        })}
      />
    ));

    expect(screen.getByText('Patrol run 5m ago')).toBeInTheDocument();

    vi.advanceTimersByTime(3 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(screen.getByText('Patrol run 3h ago')).toBeInTheDocument();
  });
});
