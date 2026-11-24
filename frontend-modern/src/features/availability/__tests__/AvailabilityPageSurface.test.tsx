import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent, screen, waitFor, cleanup } from '@solidjs/testing-library';
import { MemoryRouter, Route, createMemoryHistory } from '@solidjs/router';
import { createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
import { AvailabilityTargetsAPI, type AvailabilityTarget } from '@/api/availabilityTargets';
import { AvailabilityHistoryAPI } from '@/api/availabilityHistory';
import { syncSessionSettingsCapabilities } from '@/stores/sessionSettingsCapabilities';
import { WebSocketContext } from '@/contexts/appRuntime';
import { AvailabilityPageSurface } from '../AvailabilityPageSurface';

const [mockResources, setMockResources] = createSignal<Resource[]>([]);
const refetchMock = vi.fn(async () => undefined);

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({
    resources: mockResources,
    loading: () => false,
    error: () => null,
    refetch: refetchMock,
  }),
}));

vi.mock('@/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot', () => ({
  AvailabilityTargetSlot: (props: {
    editingTargetId?: string | null;
    onCancel: () => void;
    onSaved: () => void;
  }) => (
    <div data-testid="mock-availability-target-slot">
      <span>editing:{props.editingTargetId ?? 'new'}</span>
      <button type="button" onClick={props.onSaved}>
        Save mock check
      </button>
      <button type="button" onClick={props.onCancel}>
        Cancel mock check
      </button>
    </div>
  ),
}));

const makeEndpointResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'availability:target-1',
    name: 'Jellyfin',
    displayName: 'Jellyfin',
    type: 'network-endpoint',
    platformId: 'target-1',
    platformType: 'availability',
    sourceType: 'api',
    sources: ['availability'],
    status: 'online',
    lastSeen: Date.now(),
    availability: {
      targetId: 'target-1',
      protocol: 'https',
      address: 'jellyfin.home.arpa',
      port: 443,
      available: true,
      aggregateState: 'healthy',
      latencyMillis: 18,
      lastChecked: Date.now(),
      lastSuccess: Date.now(),
      pollIntervalSeconds: 60,
      consecutiveFailures: 0,
      failureThreshold: 2,
    },
    ...overrides,
  }) as Resource;

const makeTarget = (overrides: Partial<AvailabilityTarget> = {}): AvailabilityTarget =>
  ({
    id: 'target-1',
    name: 'Jellyfin',
    targetKind: 'service',
    address: 'jellyfin.home.arpa',
    protocol: 'https',
    port: 443,
    enabled: true,
    pollIntervalSeconds: 60,
    timeoutMillis: 2000,
    failureThreshold: 2,
    ...overrides,
  }) as AvailabilityTarget;

const mockWsContext = {
  state: () => ({
    nodes: [],
    vms: [],
    containers: [],
    dockerHosts: [],
    kubernetesClusters: [],
    hosts: [],
    storage: [],
    pbs: [],
    pmg: [],
    resources: mockResources(),
    activeAlerts: [],
    recentlyResolved: [],
    lastUpdate: Date.now(),
  }),
  connected: () => true,
  reconnecting: () => false,
  reconnect: () => undefined,
  activeAlerts: {},
  initialDataReceived: () => true,
} as never;

const renderPage = (initialPath = '/availability') => {
  const history = createMemoryHistory();
  history.set({ value: initialPath });
  return render(() => (
    <WebSocketContext.Provider value={mockWsContext}>
      <MemoryRouter history={history} root={(props) => <>{props.children}</>}>
        <Route path="/availability" component={AvailabilityPageSurface} />
        <Route path="/availability/*" component={AvailabilityPageSurface} />
      </MemoryRouter>
    </WebSocketContext.Provider>
  ));
};

describe('AvailabilityPageSurface', () => {
  beforeEach(() => {
    window.localStorage.clear();
    setMockResources([]);
    refetchMock.mockClear();
    syncSessionSettingsCapabilities({
      requiresAuth: false,
      settingsCapabilities: {
        availabilityRead: true,
      } as never,
    });
    vi.spyOn(AvailabilityTargetsAPI, 'list').mockResolvedValue([makeTarget()]);
    vi.spyOn(AvailabilityTargetsAPI, 'testSaved').mockResolvedValue({
      outcome: 'reachable',
      latencyMillis: 14,
    } as never);
    vi.spyOn(AvailabilityTargetsAPI, 'update').mockResolvedValue(makeTarget({ enabled: false }));
    vi.spyOn(AvailabilityHistoryAPI, 'batch').mockResolvedValue({
      range: '24h',
      bucketSeconds: 900,
      targets: [],
    } as never);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('opens the inline Add check modal from the empty state', async () => {
    renderPage();

    const addButton = await screen.findByTestId('availability-add-check-button');
    fireEvent.click(addButton);

    expect(await screen.findByTestId('mock-availability-target-slot')).toHaveTextContent(
      'editing:new',
    );
  });

  it('hides mutating controls for read-only sessions without availability settings access', async () => {
    syncSessionSettingsCapabilities({
      requiresAuth: true,
      settingsCapabilities: {
        availabilityRead: false,
      } as never,
    });
    setMockResources([makeEndpointResource()]);

    renderPage();

    expect(await screen.findByText('Jellyfin')).toBeInTheDocument();
    expect(screen.queryByTestId('availability-posture-add-button')).toBeNull();
    expect(screen.queryByTestId('availability-add-check-button')).toBeNull();

    fireEvent.click(screen.getByText('Jellyfin'));
    expect(screen.queryByTestId('availability-drawer-edit-button')).toBeNull();
    expect(screen.queryByTestId('availability-drawer-test-button')).toBeNull();
    expect(screen.queryByTestId('availability-drawer-toggle-button')).toBeNull();
  });

  it('runs Test now, Pause, and Edit inline from the expanded check drawer', async () => {
    setMockResources([makeEndpointResource()]);

    renderPage();

    fireEvent.click(await screen.findByText('Jellyfin'));

    const testButton = await screen.findByTestId('availability-drawer-test-button');
    fireEvent.click(testButton);
    await waitFor(() => {
      expect(AvailabilityTargetsAPI.testSaved).toHaveBeenCalledWith('target-1');
    });

    const toggleButton = await screen.findByTestId('availability-drawer-toggle-button');
    await waitFor(() => {
      expect(toggleButton).not.toBeDisabled();
    });
    fireEvent.click(toggleButton);
    await waitFor(() => {
      expect(AvailabilityTargetsAPI.update).toHaveBeenCalledWith('target-1', { enabled: false });
    });

    const editButton = await screen.findByTestId('availability-drawer-edit-button');
    await waitFor(() => {
      expect(editButton).not.toBeDisabled();
    });
    fireEvent.click(editButton);
    expect(await screen.findByTestId('mock-availability-target-slot')).toHaveTextContent(
      'editing:target-1',
    );
  });

  it('remembers the selected Fleet presentation for small inventories', async () => {
    setMockResources([makeEndpointResource()]);

    const first = renderPage('/availability');
    const viewButton = await screen.findByRole('button', { name: /View/i });
    fireEvent.click(viewButton);
    const fleetButton = await screen.findByRole('button', { name: 'Fleet' });
    fireEvent.click(fleetButton);

    expect(await screen.findByTestId('availability-fleet-grid')).toBeInTheDocument();
    expect(window.localStorage.getItem('pulse_availability_checks_view_v1')).toBe('fleet');

    first.unmount();

    renderPage('/availability');
    expect(await screen.findByTestId('availability-fleet-grid')).toBeInTheDocument();
  });
});
