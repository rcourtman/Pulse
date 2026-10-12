import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';

const requests = vi.hoisted(() => ({
  facets: vi.fn(),
  intelligence: vi.fn(),
  audits: vi.fn(),
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ state: { pmg: [] }, activeAlerts: {}, connected: () => true }),
  useDarkMode: () => () => false,
}));
vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string };
    currentMetrics?: Record<string, number | undefined>;
    range: string;
  }) => (
    <div
      data-testid="snapshot-history"
      data-target={props.target.resourceId}
      data-cpu={props.currentMetrics?.cpu}
      data-range={props.range}
    />
  ),
  GuestDrawerHistoryRangeSelect: (props: {
    range: string;
    onRangeChange: (range: string) => void;
  }) => (
    <select
      aria-label="History range"
      value={props.range}
      onChange={(event) => props.onRangeChange(event.currentTarget.value)}
    >
      <option value="24h">24 hours</option>
      <option value="7d">7 days</option>
    </select>
  ),
}));
vi.mock('@/api/resources', () => ({ ResourceAPI: { getFacetBundle: requests.facets } }));
vi.mock('@/api/ai', () => ({ AIAPI: { getResourceIntelligence: requests.intelligence } }));
vi.mock('@/api/actionAudit', () => ({ ActionAuditAPI: { listActionAudits: requests.audits } }));
vi.mock('@/api/resourceOperatorState', () => ({
  getResourceOperatorState: vi.fn(async () => null),
}));

const snapshot = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'pbs-a',
  type: 'pbs',
  name: 'PBS A',
  displayName: 'PBS A',
  platformId: 'pbs-a',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_790_982_000_000,
  metricsTarget: { resourceType: 'agent', resourceId: 'history-a' },
  cpu: { current: 12 },
  ...overrides,
});

beforeEach(() => {
  requests.facets.mockResolvedValue({ capabilities: [], relationships: [], recentChanges: [] });
  requests.intelligence.mockResolvedValue(null);
  requests.audits.mockResolvedValue({ audits: [], count: 0, available: false });
});
afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
});

describe('ResourceDetailDrawer snapshot ownership', () => {
  it('updates derived identity, current metrics and the History target without remounting a same-ID drawer', () => {
    const [resource, setResource] = createSignal(snapshot());
    render(() => <ResourceDetailDrawer resource={resource()} presentation="table-row" />);
    fireEvent.click(screen.getByRole('tab', { name: 'History' }));
    fireEvent.change(screen.getByRole('combobox', { name: 'History range' }), {
      target: { value: '7d' },
    });
    const history = screen.getByTestId('snapshot-history');
    setResource(
      snapshot({
        displayName: 'PBS A current',
        cpu: { current: 77 },
        metricsTarget: { resourceType: 'agent', resourceId: 'history-a-current' },
      }),
    );
    expect(screen.getByRole('heading', { name: 'PBS A current' })).toBeInTheDocument();
    expect(screen.getByTestId('snapshot-history')).toBe(history);
    expect(history).toHaveAttribute('data-target', 'history-a-current');
    expect(history).toHaveAttribute('data-cpu', '77');
    expect(history).toHaveAttribute('data-range', '7d');
  });

  it('withdraws a lost metrics target on an immutable snapshot and recovers the selected tab when it returns', () => {
    const [resource, setResource] = createSignal(snapshot());
    render(() => <ResourceDetailDrawer resource={resource()} presentation="table-row" />);
    fireEvent.click(screen.getByRole('tab', { name: 'History' }));
    setResource(snapshot({ metricsTarget: undefined }));
    expect(screen.queryByTestId('snapshot-history')).not.toBeInTheDocument();
    expect(screen.getByText('Metrics history is unavailable.')).toBeInTheDocument();
    setResource(snapshot());
    expect(screen.getByTestId('snapshot-history')).toHaveAttribute('data-target', 'history-a');
    expect(screen.getByRole('tab', { name: 'History' })).toHaveAttribute('aria-selected', 'true');
  });

  it('updates availability evidence in Overview instead of retaining the initial snapshot', () => {
    const initial = snapshot({
      type: 'network-endpoint',
      platformType: 'availability',
      metricsTarget: undefined,
      availability: {
        protocol: 'https',
        address: 'check-a.invalid',
        port: 443,
        available: true,
        latencyMillis: 12,
      },
    });
    const [resource, setResource] = createSignal(initial);
    const { container } = render(() => (
      <ResourceDetailDrawer resource={resource()} presentation="table-row" />
    ));
    expect(screen.getByTestId('availability-probe-status')).toHaveTextContent('Up');
    setResource({
      ...initial,
      availability: { ...initial.availability!, available: false, latencyMillis: 0 },
    });
    expect(screen.getByTestId('availability-probe-status')).toHaveTextContent('Down');
    expect(container).not.toHaveTextContent('12ms');
  });

  it('disposes per-resource disclosure state and late facet results when a different ID is selected', async () => {
    let completeOld!: (value: unknown) => void;
    requests.facets.mockReturnValueOnce(new Promise((resolve) => (completeOld = resolve)));
    const [resource, setResource] = createSignal(snapshot());
    const { container } = render(() => <ResourceDetailDrawer resource={resource()} />);
    fireEvent.click(screen.getByRole('tab', { name: 'History' }));
    setResource(
      snapshot({
        id: 'pbs-b',
        name: 'PBS B',
        displayName: 'PBS B',
        metricsTarget: { resourceType: 'agent', resourceId: 'history-b' },
      }),
    );
    expect(screen.getByRole('heading', { name: 'PBS B' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByTestId('snapshot-history')).not.toBeInTheDocument();
    await waitFor(() => expect(requests.facets).toHaveBeenCalledWith('pbs-b', { limit: 25 }));
    completeOld({
      capabilities: [],
      relationships: [],
      recentChanges: [
        { id: 'old', reason: 'Former resource change', observedAt: '2026-10-02T23:00:00Z' },
      ],
    });
    await Promise.resolve();
    expect(container).not.toHaveTextContent('Former resource change');
    expect(container).not.toHaveTextContent('PBS A');
    fireEvent.click(screen.getByRole('tab', { name: 'History' }));
    expect(screen.getByTestId('snapshot-history')).toHaveAttribute('data-target', 'history-b');
  });

  it('keeps the reconciled-store client and same-ID tab continuity working', () => {
    const [resource, setResource] = createStore(snapshot());
    render(() => <ResourceDetailDrawer resource={resource} presentation="table-row" />);
    fireEvent.click(screen.getByRole('tab', { name: 'History' }));
    const history = screen.getByTestId('snapshot-history');
    setResource(reconcile(snapshot({ cpu: { current: 35 } })));
    expect(screen.getByTestId('snapshot-history')).toBe(history);
    expect(history).toHaveAttribute('data-cpu', '35');
  });
});
