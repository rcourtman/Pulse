import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AvailabilityHistoryTarget } from '@/api/availabilityHistory';
import type { Resource } from '@/types/resource';
import { AvailabilityFleetView } from '../AvailabilityFleetView';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: { resource: Resource; onClose?: () => void }) => (
    <div data-testid="resource-detail-drawer" data-resource-id={props.resource.id}>
      <button type="button" onClick={props.onClose}>
        Close
      </button>
    </div>
  ),
}));

const resource = (index: number): Resource =>
  ({
    id: `availability:target-${index}`,
    name: `Service ${index}`,
    displayName: `Service ${index}`,
    type: 'network-endpoint',
    platformId: `target-${index}`,
    platformType: 'availability',
    sourceType: 'api',
    sources: ['availability'],
    status: index % 5 === 0 ? 'offline' : 'online',
    lastSeen: Date.parse('2026-08-30T11:59:00Z'),
    availability: {
      targetId: `target-${index}`,
      protocol: index % 2 === 0 ? 'https' : 'tcp',
      address: `service-${index}.lab.local`,
      enabled: true,
      available: index % 5 !== 0,
      outcome: index % 5 === 0 ? 'unreachable' : 'reachable',
      latencyMillis: index % 5 === 0 ? undefined : 10 + index,
      lastChecked: '2026-08-30T11:59:00Z',
      pollIntervalSeconds: 60,
    },
  }) as Resource;

const history = (index: number): AvailabilityHistoryTarget => ({
  targetId: `target-${index}`,
  summary: {
    reachableSeconds: 3600,
    unreachableSeconds: index % 5 === 0 ? 600 : 0,
    indeterminateSeconds: 300,
    unknownSeconds: 82_500,
    coveragePercent: 4.51,
    availabilityPercent: index % 5 === 0 ? 85.71 : 100,
    reachableLatencyMillis: { average: 15 + index, min: 8, max: 42 },
  },
  buckets: [
    {
      start: '2026-08-30T08:00:00Z',
      end: '2026-08-30T09:00:00Z',
      reachableSeconds: 3600,
      unreachableSeconds: 0,
      indeterminateSeconds: 0,
      unknownSeconds: 0,
      latencyMillis: { average: 12, min: 8, max: 16 },
    },
    {
      start: '2026-08-30T09:00:00Z',
      end: '2026-08-30T10:00:00Z',
      reachableSeconds: 0,
      unreachableSeconds: 0,
      indeterminateSeconds: 3600,
      unknownSeconds: 0,
    },
    {
      start: '2026-08-30T10:00:00Z',
      end: '2026-08-30T11:00:00Z',
      reachableSeconds: 0,
      unreachableSeconds: index % 5 === 0 ? 3600 : 0,
      indeterminateSeconds: 0,
      unknownSeconds: index % 5 === 0 ? 0 : 3600,
    },
    {
      start: '2026-08-30T11:00:00Z',
      end: '2026-08-30T12:00:00Z',
      reachableSeconds: 3600,
      unreachableSeconds: 0,
      indeterminateSeconds: 0,
      unknownSeconds: 0,
      latencyMillis: { average: 18, min: 14, max: 22 },
    },
  ],
  revisionBoundaries: [{ revision: 2, at: '2026-08-30T10:30:00Z' }],
});

afterEach(cleanup);

describe('AvailabilityFleetView', () => {
  it('renders fifty keyboard-accessible attention tiles with non-color history labels', () => {
    const resources = Array.from({ length: 50 }, (_, index) => resource(index));
    const historyByTarget = new Map(
      resources.map((_item, index) => [`target-${index}`, history(index)]),
    );

    const view = render(() => (
      <AvailabilityFleetView
        resources={resources}
        historyByTarget={historyByTarget}
        historyLoading={false}
      />
    ));

    expect(view.container.querySelectorAll('[data-availability-fleet-tile]')).toHaveLength(50);
    expect(screen.getByText('Reachable')).toBeInTheDocument();
    expect(screen.getByText('Unreachable')).toBeInTheDocument();
    expect(screen.getByText('Indeterminate')).toBeInTheDocument();
    expect(screen.getByText('Unknown')).toBeInTheDocument();
    expect(
      view.container.querySelector('[data-testid="availability-state-strip"]'),
    ).toHaveAttribute('aria-label', expect.stringContaining('indeterminate'));
    expect(
      view.container.querySelectorAll('[data-testid="availability-latency-line"]').length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText(/Insufficient coverage/)).toHaveLength(50);

    const tile = screen.getByRole('button', { name: 'Open details for Service 0' });
    expect(tile).toHaveAttribute('type', 'button');
    fireEvent.click(tile);
    expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
      'data-resource-id',
      'availability:target-0',
    );
  });

  it('keeps current health visible when history is unavailable', () => {
    render(() => (
      <AvailabilityFleetView
        resources={[resource(1)]}
        historyByTarget={new Map()}
        historyLoading={false}
        historyError="request failed"
      />
    ));

    expect(
      screen.getByText('History unavailable. Current health is unchanged.'),
    ).toBeInTheDocument();
    expect(screen.getByText('History unavailable')).toBeInTheDocument();
    expect(screen.getByText('Reachable')).toBeInTheDocument();
  });
});
