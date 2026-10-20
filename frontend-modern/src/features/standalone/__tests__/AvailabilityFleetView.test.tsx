import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import type { AvailabilityHistoryTarget } from '@/api/availabilityHistory';
import type { Resource } from '@/types/resource';
import { AvailabilityFleetView } from '../AvailabilityFleetView';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: { resource: Resource; onClose?: () => void }) => (
    <div
      data-testid="resource-detail-drawer"
      data-resource-id={props.resource.id}
      data-resource-status={props.resource.status}
    >
      <button type="button" onClick={props.onClose}>
        Close
      </button>
      <input aria-label="Detail note" />
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

const scrollIntoView = vi.fn();
beforeEach(() => {
  scrollIntoView.mockClear();
  Object.defineProperty(Element.prototype, 'scrollIntoView', {
    configurable: true,
    value: scrollIntoView,
  });
});
afterEach(cleanup);

describe('AvailabilityFleetView', () => {
  it('reveals/focuses details on activation and returns to the current same-ID tile on collapse', async () => {
    const [resources, setResources] = createSignal([resource(1), resource(2)]);
    render(() => (
      <AvailabilityFleetView
        resources={resources()}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));
    const originalTile = screen.getByRole('button', { name: 'Open details for Service 1' });
    fireEvent.click(originalTile);
    const panel = screen.getByRole('group', { name: 'Service 1 details' });
    await waitFor(() => expect(panel).toHaveFocus());
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'instant' });
    expect(originalTile).toHaveAttribute('aria-expanded', 'true');
    expect(originalTile).toHaveAttribute('aria-controls', panel.id);

    const note = screen.getByRole('textbox', { name: 'Detail note' });
    fireEvent.input(note, { target: { value: 'keep local state' } });
    note.focus();
    scrollIntoView.mockClear();
    setResources([resource(2), { ...resource(1), name: 'Renamed service', status: 'offline' }]);
    await Promise.resolve();
    expect(note).toHaveFocus();
    expect(note).toHaveValue('keep local state');
    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
      'data-resource-status',
      'offline',
    );
    expect(screen.getByRole('group', { name: 'Renamed service details' })).toBe(panel);

    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    const currentTile = screen.getByRole('button', { name: 'Open details for Renamed service' });
    expect(currentTile).not.toBe(originalTile);
    await waitFor(() => expect(currentTile).toHaveFocus());
    expect(currentTile).toHaveAttribute('aria-expanded', 'false');
    expect(currentTile).not.toHaveAttribute('aria-controls');
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest', behavior: 'instant' });
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
  });

  it('returns focus to a neighbouring live tile when the selected check disappears, without reopening it', async () => {
    const [resources, setResources] = createSignal([resource(0), resource(1), resource(2)]);
    render(() => (
      <AvailabilityFleetView
        resources={resources()}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Open details for Service 1' }));
    await waitFor(() =>
      expect(screen.getByRole('group', { name: 'Service 1 details' })).toHaveFocus(),
    );
    setResources([resource(0), resource(2)]);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Open details for Service 2' })).toHaveFocus(),
    );
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
    setResources([resource(0), resource(1), resource(2)]);
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
  });

  it('leaves outside controls focused when a filter removes the selected check', async () => {
    const [resources, setResources] = createSignal([resource(1), resource(2)]);
    render(() => (
      <>
        <input aria-label="Search checks" />
        <AvailabilityFleetView
          resources={resources()}
          historyByTarget={new Map()}
          historyLoading={false}
        />
      </>
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Open details for Service 1' }));
    await waitFor(() =>
      expect(screen.getByRole('group', { name: 'Service 1 details' })).toHaveFocus(),
    );
    const search = screen.getByRole('textbox', { name: 'Search checks' });
    search.focus();
    setResources([resource(2)]);
    await Promise.resolve();
    expect(search).toHaveFocus();
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
  });

  it('keeps an empty grid reachable when its last selected check disappears', async () => {
    const [resources, setResources] = createSignal([resource(1)]);
    render(() => (
      <AvailabilityFleetView
        resources={resources()}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Open details for Service 1' }));
    await waitFor(() =>
      expect(screen.getByRole('group', { name: 'Service 1 details' })).toHaveFocus(),
    );
    setResources([]);
    await waitFor(() =>
      expect(screen.getByRole('group', { name: 'Availability checks' })).toHaveFocus(),
    );
  });

  it('cancels a deferred reveal on collapse or unmount', async () => {
    const view = render(() => (
      <AvailabilityFleetView
        resources={[resource(1)]}
        historyByTarget={new Map()}
        historyLoading={false}
      />
    ));
    const tile = screen.getByRole('button', { name: 'Open details for Service 1' });
    fireEvent.click(tile);
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(tile).toHaveFocus());
    expect(scrollIntoView).not.toHaveBeenCalledWith({ block: 'start', behavior: 'instant' });
    scrollIntoView.mockClear();
    fireEvent.click(tile);
    view.unmount();
    await Promise.resolve();
    expect(scrollIntoView).not.toHaveBeenCalled();
  });

  it('lets the parent restore focus when focused details unmount with the entire fleet', async () => {
    const onFocusedDetailsUnmount = vi.fn();
    const view = render(() => (
      <AvailabilityFleetView
        resources={[resource(1)]}
        historyByTarget={new Map()}
        historyLoading={false}
        onFocusedDetailsUnmount={onFocusedDetailsUnmount}
      />
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Open details for Service 1' }));
    await waitFor(() =>
      expect(screen.getByRole('group', { name: 'Service 1 details' })).toHaveFocus(),
    );
    view.unmount();
    expect(onFocusedDetailsUnmount).toHaveBeenCalledOnce();
  });

  it('does not request parent focus when an outside control already owns it', async () => {
    const onFocusedDetailsUnmount = vi.fn();
    const view = render(() => (
      <>
        <button type="button">Outside</button>
        <AvailabilityFleetView
          resources={[resource(1)]}
          historyByTarget={new Map()}
          historyLoading={false}
          onFocusedDetailsUnmount={onFocusedDetailsUnmount}
        />
      </>
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Open details for Service 1' }));
    await waitFor(() =>
      expect(screen.getByRole('group', { name: 'Service 1 details' })).toHaveFocus(),
    );
    screen.getByRole('button', { name: 'Outside' }).focus();
    view.unmount();
    expect(onFocusedDetailsUnmount).not.toHaveBeenCalled();
  });

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
