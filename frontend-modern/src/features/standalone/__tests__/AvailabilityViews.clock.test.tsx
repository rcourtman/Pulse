import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { AvailabilityProbeStatusCard } from '@/components/Infrastructure/AvailabilityProbeStatusCard';
import { AvailabilityFleetView } from '../AvailabilityFleetView';

const start = Date.parse('2026-10-10T12:00:00Z');
const resource = {
  id: 'clock-service',
  type: 'network-endpoint',
  platformType: 'availability',
  sources: ['availability'],
  name: 'Clock service',
  displayName: 'Clock service',
  status: 'online',
  lastSeen: start,
  availability: {
    targetId: 'clock-service',
    name: 'Clock service',
    address: 'clock.example.test',
    protocol: 'https',
    enabled: true,
    available: true,
    lastChecked: new Date(start - 60_000).toISOString(),
    pollIntervalSeconds: 30,
    evidence: { validUntil: new Date(start + 10_000).toISOString() },
  },
} as Resource;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('availability views share the mounted freshness clock', () => {
  it('ages unchanged fleet evidence and switches its health without a refresh', () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
    const { container } = render(() => (
      <AvailabilityFleetView
        resources={[resource]}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));
    expect(container.querySelector('[title="Stale"]')).toBeNull();
    expect(screen.getByText('Checked 1m ago')).toBeInTheDocument();
    vi.advanceTimersByTime(60_000);
    expect(container.querySelector('[title="Stale"]')).not.toBeNull();
    expect(screen.getByText('Checked 2m ago')).toBeInTheDocument();
  });

  it('ages an open probe card without changing its summary or source paths', () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
    render(() => <AvailabilityProbeStatusCard availability={resource.availability!} />);
    expect(screen.getByText('Up')).toBeInTheDocument();
    expect(screen.getByText('1 min ago')).toBeInTheDocument();
    vi.advanceTimersByTime(60_000);
    expect(screen.queryByText('Up')).not.toBeInTheDocument();
    expect(screen.getByText('Stale')).toBeInTheDocument();
    expect(screen.getByText('2 mins ago')).toBeInTheDocument();
  });
});
