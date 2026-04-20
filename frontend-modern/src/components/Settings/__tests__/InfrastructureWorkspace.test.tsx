import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Connection } from '@/api/connections';
import { InfrastructureWorkspace } from '../InfrastructureWorkspace';

const routeState = vi.hoisted(() => ({
  pathname: '/settings/infrastructure',
  search: '',
}));
const connectionState = vi.hoisted(() => ({
  connections: [] as Connection[],
}));
const navigateSpy = vi.hoisted(() => vi.fn());
const setSearchParamsSpy = vi.hoisted(() => vi.fn());
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const probeSpy = vi.hoisted(() => vi.fn().mockResolvedValue({ candidates: [], probedMs: 0 }));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: routeState.pathname, search: routeState.search }),
    useNavigate: () => navigateSpy,
    useSearchParams: () => [{}, setSearchParamsSpy],
  };
});

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
}));

vi.mock('../useInfrastructureOperationsState', () => ({
  InfrastructureOperationsStateProvider: (props: { children: unknown }) => <>{props.children}</>,
  useInfrastructureOperationsContext: () => ({
    getUninstallCommand: () => 'curl -fsSL http://pulse/install.sh | bash -s -- --uninstall',
    getWindowsUninstallCommand: () =>
      '$env:PULSE_URL="http://pulse"; $env:PULSE_UNINSTALL="true"; iwr /install.ps1 | iex',
  }),
}));

vi.mock('../useConnectionsLedger', () => ({
  useConnectionsLedger: () => ({
    connections: () => connectionState.connections,
    rows: () =>
      connectionState.connections.map((connection) => ({
        id: connection.id,
        name: connection.name || connection.id,
        subtitle: connection.type,
        host: connection.address,
        coverageLabels: ['VMs'],
        statusLabel: connection.state === 'paused' ? 'Paused' : 'Active',
        statusClassName: '',
        lastActivityText: '1m ago',
        lastErrorMessage: connection.lastError?.message,
        enabled: connection.enabled,
        canEdit: ['pve', 'pbs', 'pmg', 'vmware', 'truenas'].includes(connection.type),
        canPause: connection.capabilities.supportsPause,
        canRemove: connection.type !== 'docker' && connection.type !== 'kubernetes',
        isAgent: connection.type === 'agent',
        connection,
      })),
    findById: (id: string) => connectionState.connections.find((connection) => connection.id === id),
    reload: vi.fn(),
    loading: () => false,
    error: () => null,
  }),
}));

vi.mock('../InfrastructureInstallerSection', () => ({
  InfrastructureInstallerSection: () => <div data-testid="install-section">install</div>,
}));

vi.mock('../ConnectionEditor/CredentialSlots/NodeCredentialSlot', () => ({
  NodeCredentialSlot: (props: { nodeType: string }) => (
    <div data-testid="proxmox-section" data-node-type={props.nodeType}>
      proxmox
    </div>
  ),
}));

vi.mock('../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot', () => ({
  TrueNASCredentialSlot: () => <div data-testid="truenas-section">truenas</div>,
}));

vi.mock('../ConnectionEditor/CredentialSlots/VMwareCredentialSlot', () => ({
  VMwareCredentialSlot: () => <div data-testid="vmware-section">vmware</div>,
}));

vi.mock('../AgentProfilesPanel', () => ({
  AgentProfilesPanel: () => <div data-testid="agent-profiles">profiles</div>,
}));

vi.mock('@/api/connections', async () => {
  const actual = await vi.importActual<typeof import('@/api/connections')>('@/api/connections');
  return {
    ...actual,
    ConnectionsAPI: {
      ...actual.ConnectionsAPI,
      probe: probeSpy,
    },
  };
});

const connectionFixture = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:zeus',
  type: 'pve',
  name: 'zeus',
  address: 'https://10.0.0.1:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms', 'containers'],
  scope: { vms: true, containers: true },
  lastSeen: new Date().toISOString(),
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const baseProps = () =>
  ({
    selectedAgent: () => 'pve',
    onSelectAgent: vi.fn(),
    initialLoadComplete: () => true,
    discoveryEnabled: () => false,
    discoveryMode: () => 'auto',
    discoveryScanStatus: () => 'idle',
    discoveredNodes: () => [],
    savingDiscoverySettings: () => false,
    envOverrides: () => ({}),
    agentStateResources: () => [],
    pbsInstances: () => [],
    pmgInstances: () => [],
    pveNodes: () => [],
    pbsNodes: () => [],
    pmgNodes: () => [],
    trueNASSettings: {} as any,
    vmwareSettings: {} as any,
    temperatureMonitoringEnabled: () => false,
    triggerDiscoveryScan: vi.fn(),
    loadDiscoveredNodes: vi.fn(),
    handleDiscoveryEnabledChange: vi.fn(),
    testNodeConnection: vi.fn(),
    requestDeleteNode: vi.fn(),
    refreshClusterNodes: vi.fn(),
    setShowNodeModal: vi.fn(),
    editingNode: () => null,
    setEditingNode: vi.fn(),
    setCurrentNodeType: vi.fn(),
    modalResetKey: () => 0,
    setModalResetKey: vi.fn(),
    isNodeModalVisible: () => false,
    securityStatus: () => null,
    resolveTemperatureMonitoringEnabled: () => false,
    temperatureMonitoringLocked: () => false,
    savingTemperatureSetting: () => false,
    handleTemperatureMonitoringChange: vi.fn(),
    disableDockerUpdateActions: () => false,
    disableDockerUpdateActionsLocked: () => false,
    savingDockerUpdateActions: () => false,
    handleDisableDockerUpdateActionsChange: vi.fn(),
    handleNodeTemperatureMonitoringChange: vi.fn(),
    saveNode: vi.fn(),
    showDeleteNodeModal: () => false,
    cancelDeleteNode: vi.fn(),
    deleteNode: vi.fn(),
    deleteNodeLoading: () => false,
    nodePendingDeleteLabel: () => '',
    nodePendingDeleteHost: () => '',
    nodePendingDeleteType: () => '',
    nodePendingDeleteTypeLabel: () => '',
  }) as any;

describe('InfrastructureWorkspace', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    setSearchParamsSpy.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    probeSpy.mockClear();
    routeState.pathname = '/settings/infrastructure';
    routeState.search = '';
    connectionState.connections = [connectionFixture()];
  });

  afterEach(() => {
    cleanup();
  });

  const renderWorkspace = (propOverrides: Record<string, unknown> = {}) =>
    render(() => (<InfrastructureWorkspace {...{ ...baseProps(), ...propOverrides }} />) as any);

  it('renders the inventory table at the base infrastructure route', async () => {
    renderWorkspace();

    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: 'Add infrastructure' })).toBeInTheDocument();
    expect(screen.getByText('zeus')).toBeInTheDocument();
    expect(screen.queryByTestId('install-section')).toBeNull();
  });

  it('renders unified connection rows from the ledger', async () => {
    connectionState.connections = [
      connectionFixture({ id: 'pve:zeus', name: 'zeus', type: 'pve', state: 'active' }),
      connectionFixture({
        id: 'truenas:tower',
        name: 'tower-nas',
        type: 'truenas',
        state: 'paused',
        enabled: false,
      }),
    ];
    renderWorkspace();

    await waitFor(() => expect(screen.getByText('zeus')).toBeInTheDocument());
    expect(screen.getByText('tower-nas')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Paused')).toBeInTheDocument();
  });

  it('opens the catalog landing when Add infrastructure is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));

    // The catalog landing surfaces the probe input and a tile grid of every
    // supported product — including Install Pulse Agent as a peer tile — with
    // no intermediate picker screen.
    expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Install Pulse Agent/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Proxmox VE/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /TrueNAS SCALE/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /VMware vCenter \/ ESXi/i })).toBeInTheDocument();
    expect(navigateSpy).not.toHaveBeenCalled();
    expect(setSearchParamsSpy).not.toHaveBeenCalled();
  });

  it('routes to the agent install slot when Install Pulse Agent tile is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByRole('button', { name: /Install Pulse Agent/i }));

    expect(screen.getByTestId('install-section')).toBeInTheDocument();
  });

  it('routes to the TrueNAS credential slot when TrueNAS tile is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByRole('button', { name: /TrueNAS SCALE/i }));

    expect(screen.getByTestId('truenas-section')).toBeInTheDocument();
  });

  it('routes to the VMware credential slot when VMware tile is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByRole('button', { name: /VMware vCenter \/ ESXi/i }));

    expect(screen.getByTestId('vmware-section')).toBeInTheDocument();
  });

  it('routes to the Proxmox credential slot when Proxmox VE tile is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Proxmox VE/i }));

    expect(screen.getByTestId('proxmox-section')).toBeInTheDocument();
  });

  it('can return to the catalog from a credential slot', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByRole('button', { name: /Install Pulse Agent/i }));
    fireEvent.click(screen.getByRole('button', { name: /Back to catalog/i }));

    expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Install Pulse Agent/i })).toBeInTheDocument();
    expect(screen.queryByTestId('install-section')).toBeNull();
  });

  it('toggles agent profiles inside the agent install slot', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByRole('button', { name: /Install Pulse Agent/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Manage agent profiles' }));

    expect(screen.getByTestId('agent-profiles')).toBeInTheDocument();
  });

  it('exposes Edit / Pause / Remove on each ledger row directly', async () => {
    renderWorkspace();

    await waitFor(() => expect(screen.getByText('zeus')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pause' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'View details' })).toBeNull();
  });

  it('opens the inline edit flow when Edit is clicked on a pve row', async () => {
    renderWorkspace({
      pveNodes: () => [{ name: 'zeus', host: 'https://10.0.0.1:8006' } as any],
    });

    await waitFor(() => expect(screen.getByText('zeus')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

    await waitFor(() => expect(screen.getByTestId('proxmox-section')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /Back to systems/i })).toBeInTheDocument();
  });

  // `within` was previously used for the detail panel test; retain a smoke reference so the
  // import stays live until the broader test harness evolves.
  it('isolates row content by cell', async () => {
    renderWorkspace();
    await waitFor(() => expect(screen.getByText('zeus')).toBeInTheDocument());
    const table = screen.getByRole('table');
    expect(within(table).getByText('zeus')).toBeInTheDocument();
  });

  it('redirects legacy install deep links and pre-selects the agent install slot', async () => {
    routeState.pathname = '/settings/infrastructure/install';
    renderWorkspace();

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', { replace: true });
    expect(setSearchParamsSpy).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.getByTestId('install-section')).toBeInTheDocument());
  });

  it('clears the canonical query onboarding route and pre-selects the agent install slot', async () => {
    routeState.search = '?add=agent';
    renderWorkspace();

    expect(setSearchParamsSpy).toHaveBeenCalledWith({ add: null }, { replace: true });
    expect(navigateSpy).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.getByTestId('install-section')).toBeInTheDocument());
  });

  it('clears the canonical query onboarding route for platform picking and lands on the catalog', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    expect(setSearchParamsSpy).toHaveBeenCalledWith({ add: null }, { replace: true });
    expect(navigateSpy).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument(),
    );
    // Catalog landing — probe input + tile grid are visible, no credential
    // slot has been entered yet.
    expect(screen.getByRole('button', { name: /Install Pulse Agent/i })).toBeInTheDocument();
    expect(screen.queryByTestId('install-section')).toBeNull();
  });

  it('keeps legacy platform-management paths out of add mode', async () => {
    routeState.pathname = '/settings/infrastructure/platforms/truenas';
    renderWorkspace();

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', { replace: true });
    expect(setSearchParamsSpy).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument(),
    );
    expect(screen.queryByRole('button', { name: /Probe address/i })).toBeNull();
    expect(screen.queryByTestId('truenas-section')).toBeNull();
  });

  it('hides Add infrastructure and every add sub-flow in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    renderWorkspace();

    expect(screen.queryByRole('button', { name: /Add infrastructure/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Install Pulse Agent/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Probe address/i })).toBeNull();
    expect(screen.queryByTestId('install-section')).toBeNull();
  });

  it('clears query onboarding but keeps read-only sessions on the inventory', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    routeState.search = '?add=agent';
    renderWorkspace();

    expect(setSearchParamsSpy).toHaveBeenCalledWith({ add: null }, { replace: true });
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.queryByRole('button', { name: /Probe address/i })).toBeNull();
  });
});
