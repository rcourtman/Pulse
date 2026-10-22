import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal, onCleanup } from 'solid-js';
import type { AvailabilityHistoryTarget } from '@/api/availabilityHistory';
import type { Resource } from '@/types/resource';
import { AvailabilityFleetView } from '../AvailabilityFleetView';
import { getStandaloneResourceStatusIndicator } from '../standalonePageModel';

const drawerMounts = vi.hoisted(() => ({ count: 0 }));

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: {
    resource: Resource;
    onClose?: () => void;
    presentation?: string;
    statusIndicator?: { variant: string; label: string };
  }) => {
    drawerMounts.count += 1;
    onCleanup(() => {
      drawerMounts.count -= 1;
    });
    return (
      <div
        data-testid="resource-detail-drawer"
        data-resource-id={props.resource.id}
        data-resource-status={props.resource.status}
        data-presentation={props.presentation}
        data-status-variant={props.statusIndicator?.variant}
      >
        <button type="button" onClick={props.onClose}>
          Close
        </button>
      </div>
    );
  },
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
  it('keeps open details bound to the selected ID in the current fleet, not the clicked snapshot', () => {
    const original = resource(1);
    const [resources, setResources] = createSignal([original]);
    render(() => (
      <AvailabilityFleetView
        resources={resources()}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Open details for Service 1' }));
    setResources([{ ...original, status: 'offline' }]);
    expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
      'data-resource-status',
      'offline',
    );
    setResources([]);
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
    setResources([original]);
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
  });

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

  it('makes each tile a disclosure for the shared drawer, closed by a second click', () => {
    render(() => (
      <AvailabilityFleetView
        resources={[resource(1), resource(5)]}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));

    const first = screen.getByRole('button', { name: 'Open details for Service 1' });
    const second = screen.getByRole('button', { name: 'Open details for Service 5' });
    expect(first).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('availability-fleet-detail')).not.toBeInTheDocument();

    fireEvent.click(first);
    // The drawer opens right after its tile as a full-width grid item, so it
    // lands beneath that tile's row rather than below the whole fleet.
    const detail = screen.getByTestId('availability-fleet-detail');
    expect(first).toHaveAttribute('aria-controls', detail.id);
    expect(first.nextElementSibling).toBe(detail);
    expect(detail).toHaveClass('col-span-full');
    expect(screen.getByTestId('availability-fleet-grid')).toHaveClass('grid-flow-row-dense');
    expect(first).toHaveAttribute('aria-expanded', 'true');
    expect(first).toHaveClass('border-blue-500');
    // The same drawer the checks table opens, headed by the tile's own dot.
    const drawer = screen.getByTestId('resource-detail-drawer');
    expect(drawer).toHaveAttribute('data-presentation', 'table-row');
    expect(drawer).toHaveAttribute(
      'data-status-variant',
      getStandaloneResourceStatusIndicator(resource(1), Date.now()).variant,
    );

    fireEvent.click(second);
    expect(first).toHaveAttribute('aria-expanded', 'false');
    expect(second).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
      'data-status-variant',
      getStandaloneResourceStatusIndicator(resource(5), Date.now()).variant,
    );

    fireEvent.click(second);
    expect(second).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
  });

  it('returns focus to the tile when the drawer is closed from its header', async () => {
    render(() => (
      <AvailabilityFleetView
        resources={[resource(1), resource(2)]}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));

    const tile = screen.getByRole('button', { name: 'Open details for Service 2' });
    fireEvent.click(tile);
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await Promise.resolve();
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
    expect(tile).toHaveAttribute('aria-expanded', 'false');
    expect(tile).toHaveFocus();
  });

  it('keeps an open drawer mounted when a live snapshot replaces the resources', () => {
    const [resources, setResources] = createSignal([resource(1), resource(2)]);
    render(() => (
      <AvailabilityFleetView
        resources={resources()}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Open details for Service 2' }));
    expect(drawerMounts.count).toBe(1);
    const drawer = screen.getByTestId('resource-detail-drawer');

    // New objects for the same checks, as the websocket store delivers them.
    setResources([resource(1), { ...resource(2), status: 'offline' } as Resource]);

    expect(screen.getByTestId('resource-detail-drawer')).toBe(drawer);
    expect(drawerMounts.count).toBe(1);
    expect(drawer).toHaveAttribute('data-resource-status', 'offline');
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
