import { Route, Router } from '@solidjs/router';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { AvailabilityChecksTable } from '../AvailabilityChecksTable';

vi.mock('@/api/availabilityHistory', () => ({
  AvailabilityHistoryAPI: { batch: vi.fn(async () => ({ targets: [] })) },
}));

const START = Date.parse('2026-08-30T12:00:20Z');

const check = (): Resource =>
  ({
    id: 'availability:mqtt-meter',
    name: 'MQTT power meter',
    displayName: 'MQTT power meter',
    type: 'network-endpoint',
    platformId: 'mqtt-meter',
    platformType: 'availability',
    sourceType: 'api',
    sources: ['availability'],
    status: 'online',
    lastSeen: Date.parse('2026-08-30T12:00:00Z'),
    availability: {
      targetId: 'mqtt-meter',
      protocol: 'tcp',
      address: 'power-meter-01.lab.local',
      port: 1883,
      enabled: true,
      available: true,
      latencyMillis: 7,
      lastChecked: '2026-08-30T12:00:00Z',
      pollIntervalSeconds: 60,
    },
  }) as Resource;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('AvailabilityChecksTable relative ages', () => {
  it('keeps a stalled check row aging into stale while its data does not change', () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: START });

    const { container } = render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <AvailabilityChecksTable
              resources={[check()]}
              emptyIcon={<span />}
              emptyTitle="No checks"
              emptyDescription="Add checks"
            />
          )}
        />
      </Router>
    ));
    const row = () =>
      container.querySelector('[data-availability-check-row="availability:mqtt-meter"]')!;
    const result = () => screen.getByText('7 ms', { selector: 'span[title]' });

    expect(row().querySelector('[title="Online"]')).not.toBeNull();
    expect(result()).toHaveAttribute('title', expect.stringContaining('fresh'));
    expect(result()).toHaveAttribute('title', expect.stringContaining('checked 20s ago'));
    expect(screen.getByRole('button', { name: /Degraded/ })).toHaveTextContent('0');

    // The probe stalls: lastChecked never changes and only the clock moves.
    vi.advanceTimersByTime(20 * RELATIVE_TIME_TICK_MS);

    expect(row().querySelector('[title="Stale"]')).not.toBeNull();
    expect(result()).toHaveAttribute('title', expect.stringContaining('stale'));
    expect(result()).toHaveAttribute('title', expect.stringContaining('checked 10 mins ago'));
    // The status filter buckets the row by the same clock, so it counts as
    // needing attention rather than online.
    expect(screen.getByRole('button', { name: /Degraded/ })).toHaveTextContent('1');
  });
});
