import { Route, Router } from '@solidjs/router';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { AvailabilityChecksTable } from '../AvailabilityChecksTable';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));
vi.mock('@/api/availabilityHistory', () => ({
  AvailabilityHistoryAPI: { batch: vi.fn(async () => ({ targets: [] })) },
}));

const start = Date.parse('2026-10-09T20:00:00Z');
const check = (id: string, validFor: number): Resource =>
  ({
    id,
    type: 'network-endpoint',
    name: id,
    displayName: id,
    status: 'online',
    lastSeen: start,
    platformType: 'availability',
    sources: ['availability'],
    availability: {
      targetId: id,
      protocol: 'tcp',
      address: `${id}.invalid`,
      port: 443,
      enabled: true,
      available: true,
      lastChecked: new Date(start).toISOString(),
      pollIntervalSeconds: 30,
      evidence: { validUntil: new Date(start + validFor).toISOString() },
    },
  }) as Resource;
const resources = [check('A-fresh', 180_000), check('Z-stalling', 10_000)];
function mount(status: 'all' | 'online' | 'degraded' = 'all') {
  const [filter, setFilter] = createSignal(status);
  const view = render(() => (
    <Router>
      <Route
        path="/"
        component={() => (
          <AvailabilityChecksTable
            resources={resources}
            emptyIcon={<span />}
            emptyTitle="No checks"
            emptyDescription="Add checks"
            externalStatus={filter}
            onExternalStatusChange={setFilter}
          />
        )}
      />
    </Router>
  ));
  return { ...view, setFilter };
}
const names = (container: HTMLElement) =>
  [...container.querySelectorAll('[data-availability-check-row]')].map((row) =>
    row.querySelector('[data-row-action]')?.getAttribute('aria-label'),
  );

describe('mounted availability check freshness', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(start);
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });
  it('updates status, probe detail and attention ordering without a resource refresh', () => {
    const { container } = mount();
    expect(names(container)[0]).toContain('A-fresh');
    expect(container.querySelector('[title="Stale"]')).not.toBeInTheDocument();
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(container.querySelector('[title="Stale"]')).toBeInTheDocument();
    expect(names(container)[0]).toContain('Z-stalling');
    const stale = container.querySelector('[data-availability-check-row="Z-stalling"]');
    expect(stale?.querySelector('[title*="stale"]')).not.toBeNull();
  });
  it('moves a stalled check from healthy to degraded filters on the shared tick', () => {
    const { setFilter } = mount('online');
    expect(screen.getByText('Z-stalling')).toBeInTheDocument();
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(screen.queryByText('Z-stalling')).not.toBeInTheDocument();
    expect(screen.getByText('A-fresh')).toBeInTheDocument();
    setFilter('degraded');
    expect(screen.getByText('Z-stalling')).toBeInTheDocument();
    expect(screen.queryByText('A-fresh')).not.toBeInTheDocument();
  });
});
