import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: routeState.pathname, search: routeState.search }),
    useNavigate: () => navigateSpy,
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
        subtitle:
          connection.type === 'agent'
            ? 'Pulse Unified Agent'
            : `Platform API · ${connection.type === 'truenas' ? 'TrueNAS SCALE' : connection.type}`,
        host: connection.address,
        coverageLabels: ['VMs'],
        statusLabel: connection.state === 'paused' ? 'Paused' : 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        lastActivityText: '1m ago',
        lastErrorMessage: connection.lastError?.message,
        enabled: connection.enabled,
        canEdit: ['pve', 'pbs', 'pmg', 'vmware', 'truenas'].includes(connection.type),
        canPause: connection.capabilities.supportsPause,
        canRemove: connection.type !== 'docker' && connection.type !== 'kubernetes',
        isAgent: connection.type === 'agent',
        connection,
      })),
    findById: (id: string) =>
      connectionState.connections.find((connection) => connection.id === id),
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

vi.mock('@/utils/infrastructureOnboardingMetrics', () => ({
  createInfrastructureOnboardingMetricsTracker: () => ({
    recordOpened: vi.fn(),
    recordPathSelected: vi.fn(),
    recordProbeResult: vi.fn(),
    recordCatalogSelected: vi.fn(),
    recordCredentialsOpened: vi.fn(),
  }),
}));

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
    trueNASSettings: {
      closeDialog: vi.fn(),
      connections: () => [],
    } as any,
    vmwareSettings: {
      closeDialog: vi.fn(),
      connections: () => [],
    } as any,
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
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    routeState.pathname = '/settings/infrastructure';
    routeState.search = '';
    connectionState.connections = [connectionFixture()];
  });

  afterEach(() => {
    cleanup();
  });

  const renderWorkspace = (propOverrides: Record<string, unknown> = {}) =>
    render(() => (<InfrastructureWorkspace {...{ ...baseProps(), ...propOverrides }} />) as any);

  it('renders the source-manager landing and keeps the monitored systems summary below it', async () => {
    renderWorkspace();

    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Connection types' })).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: 'Detect from address' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add VMware vCenter' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add TrueNAS' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument();
    expect(screen.getAllByText('zeus').length).toBeGreaterThan(0);
  });

  it('opens the detect dialog through the canonical onboarding route', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: 'Detect from address' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=pick', {
      scroll: false,
    });
  });

  it('opens a type-specific add route from the source cards', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: 'Add TrueNAS' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=truenas', {
      scroll: false,
    });
  });

  it('renders the route-backed detect dialog when the canonical pick query is present', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Add infrastructure')).toBeInTheDocument();
    expect(screen.getByText('Choose how Pulse should connect')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
  });

  it('renders the route-backed agent dialog when the canonical agent query is present', async () => {
    routeState.search = '?add=agent';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Add Pulse Agent')).toBeInTheDocument();
    expect(screen.getByTestId('install-section')).toBeInTheDocument();
  });

  it('opens the edit dialog directly from an existing source row', async () => {
    renderWorkspace({
      pveNodes: () => [{ name: 'zeus', host: 'https://10.0.0.1:8006' } as any],
    });

    await waitFor(() => expect(screen.getAllByText('zeus').length).toBeGreaterThan(0));
    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0]);

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Edit zeus')).toBeInTheDocument();
    expect(screen.getByTestId('proxmox-section')).toBeInTheDocument();
  });

  it('hides source-manager actions and route-backed dialogs in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    routeState.search = '?add=agent';
    renderWorkspace();

    expect(screen.queryByRole('button', { name: 'Detect from address' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Add VMware vCenter' })).toBeNull();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument();
  });
});
