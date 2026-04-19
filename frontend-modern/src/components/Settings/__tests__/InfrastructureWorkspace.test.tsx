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
        collectionLabel: connection.type === 'agent' ? 'Agent' : 'API',
        statusLabel: connection.state === 'paused' ? 'Paused' : 'Active',
        statusClassName: '',
        lastActivityText: '1m ago',
        manageLabel: 'View details',
        manage: { kind: 'connection' as const, connectionId: connection.id },
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

vi.mock('../ConnectionDetailPanel', () => ({
  ConnectionDetailPanel: (props: { connection: () => Connection | undefined }) => (
    <>
      {props.connection() ? (
        <div data-testid="connection-detail-panel">
          <div>{props.connection()!.name || props.connection()!.id}</div>
        </div>
      ) : null}
    </>
  ),
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
    expect(screen.getByRole('button', { name: 'Add connection' })).toBeInTheDocument();
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

  it('opens the inline add flow when Add connection is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));

    expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Enter credentials manually/i })).toBeInTheDocument();
    expect(navigateSpy).not.toHaveBeenCalled();
    expect(setSearchParamsSpy).not.toHaveBeenCalled();
  });

  it('routes to the agent install slot when Pulse agent is picked manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /Agent \(install on host\)/i }));

    expect(screen.getByTestId('install-section')).toBeInTheDocument();
  });

  it('routes to the TrueNAS credential slot when TrueNAS is picked manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /TrueNAS SCALE/i }));

    expect(screen.getByTestId('truenas-section')).toBeInTheDocument();
  });

  it('routes to the VMware credential slot when VMware is picked manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /VMware vCenter \/ ESXi/i }));

    expect(screen.getByTestId('vmware-section')).toBeInTheDocument();
  });

  it('routes to the Proxmox credential slot when Proxmox VE is picked manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Proxmox VE/i }));

    expect(screen.getByTestId('proxmox-section')).toBeInTheDocument();
  });

  it('can return to the probe step from a credential slot', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /Agent \(install on host\)/i }));
    fireEvent.click(screen.getByRole('button', { name: /Back to probe/i }));

    expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
    expect(screen.queryByTestId('install-section')).toBeNull();
  });

  it('toggles agent profiles inside the agent install slot', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /Agent \(install on host\)/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Manage agent profiles' }));

    expect(screen.getByTestId('agent-profiles')).toBeInTheDocument();
  });

  it('opens the inline connection detail panel when a ledger row is viewed', async () => {
    renderWorkspace();

    await waitFor(() => expect(screen.getByText('zeus')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'View details' }));

    const panel = screen.getByTestId('connection-detail-panel');
    expect(panel).toBeInTheDocument();
    expect(within(panel).getByText('zeus')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Back to systems/i })).toBeInTheDocument();
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

  it('clears the canonical query onboarding route for platform picking without pre-selecting a type', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    expect(setSearchParamsSpy).toHaveBeenCalledWith({ add: null }, { replace: true });
    expect(navigateSpy).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument(),
    );
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

  it('hides Add connection and the add flow in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    renderWorkspace();

    expect(screen.queryByRole('button', { name: /Add connection/i })).toBeNull();
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
