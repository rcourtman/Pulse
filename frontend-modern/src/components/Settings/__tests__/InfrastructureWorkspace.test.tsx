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
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const onboardingMetricsTrackers = vi.hoisted(
  () =>
    [] as Array<{
      recordOpened: ReturnType<typeof vi.fn>;
      recordPathSelected: ReturnType<typeof vi.fn>;
      recordProbeResult: ReturnType<typeof vi.fn>;
      recordCatalogSelected: ReturnType<typeof vi.fn>;
      recordCredentialsOpened: ReturnType<typeof vi.fn>;
    }>,
);
const createInfrastructureOnboardingMetricsTrackerMock = vi.hoisted(() =>
  vi.fn(() => {
    const tracker = {
      recordOpened: vi.fn(),
      recordPathSelected: vi.fn(),
      recordProbeResult: vi.fn(),
      recordCatalogSelected: vi.fn(),
      recordCredentialsOpened: vi.fn(),
    };
    onboardingMetricsTrackers.push(tracker);
    return tracker;
  }),
);

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
  createInfrastructureOnboardingMetricsTracker: createInfrastructureOnboardingMetricsTrackerMock,
  getSharedInfrastructureOnboardingMetricsTracker: createInfrastructureOnboardingMetricsTrackerMock,
  clearSharedInfrastructureOnboardingMetricsTracker: vi.fn(),
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
    discoveryScanStatus: () => ({ scanning: false }),
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
    createInfrastructureOnboardingMetricsTrackerMock.mockClear();
    onboardingMetricsTrackers.length = 0;
    routeState.pathname = '/settings/infrastructure';
    routeState.search = '';
    connectionState.connections = [connectionFixture()];
  });

  afterEach(() => {
    cleanup();
  });

  const renderWorkspace = (propOverrides: Record<string, unknown> = {}) =>
    render(() => (<InfrastructureWorkspace {...{ ...baseProps(), ...propOverrides }} />) as any);

  it('renders the instance-first source manager as the only landing surface', async () => {
    renderWorkspace();

    await waitFor(() =>
      expect(screen.getByText('Infrastructure sources')).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: /Run discovery/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Discovery settings/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Detect from address/i })).toBeInTheDocument();
    expect(screen.getByText('VMware vCenter')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS SCALE')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Add Proxmox VE/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Edit/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Monitored systems' })).not.toBeInTheDocument();
  });

  it('routes discovery actions from the manager and shows discovered candidates in the matching platform group', async () => {
    const triggerDiscoveryScan = vi.fn();
    renderWorkspace({
      discoveredNodes: () => [
        {
          ip: '10.0.0.55',
          port: 8006,
          type: 'pve',
          version: '8.2.2',
          hostname: 'discovered-pve.lab',
        },
      ],
      discoveryScanStatus: () => ({ scanning: false, lastResultAt: Date.now() }),
      triggerDiscoveryScan,
    });

    await waitFor(() =>
      expect(screen.getByText('discovered-pve.lab')).toBeInTheDocument(),
    );
    expect(screen.getByText('Discovered')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Run discovery/i }));
    expect(triggerDiscoveryScan).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /Discovery settings/i }));
    expect(navigateSpy).toHaveBeenCalledWith('/settings/system-network', {
      scroll: false,
    });

    fireEvent.click(screen.getByRole('button', { name: /^Review$/i }));
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=pve', {
      scroll: false,
    });
  });

  it('opens the detect dialog from the source manager header action', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Detect from address/i }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=detect', {
      scroll: false,
    });
  });

  it('opens a type-specific add route from the matching platform section', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add TrueNAS SCALE/i }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=truenas', {
      scroll: false,
    });
    expect(createInfrastructureOnboardingMetricsTrackerMock).toHaveBeenCalledTimes(1);
    expect(onboardingMetricsTrackers[0]?.recordOpened).toHaveBeenCalledTimes(1);
  });

  it('renders the route-backed picker dialog when the pick query is present', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    const dialog = screen.getByRole('dialog');
    expect(screen.getAllByText('Add infrastructure').length).toBeGreaterThan(0);
    expect(screen.getByText('Choose the source type you want to add.')).toBeInTheDocument();
    expect(within(dialog).getByText('Choose a source type')).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Detect from address' })).toBeInTheDocument();
    expect(within(dialog).getByText('Virtualization')).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: /TrueNAS SCALE/i })).toBeInTheDocument();
  });

  it('routes the picker detect action into the detect dialog', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Detect from address' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=detect', {
      scroll: false,
    });
  });

  it('routes the picker source cards into direct type-specific dialogs', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /TrueNAS SCALE/i }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=truenas', {
      scroll: false,
    });
  });

  it('renders the detect dialog as a secondary utility flow', async () => {
    routeState.search = '?add=detect';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Detect infrastructure source')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Probe an address and let Pulse open the matching credential flow when it recognizes the platform.',
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText('Detect from address').length).toBeGreaterThan(0);
    expect(within(screen.getByRole('dialog')).getByRole('button', { name: /Back to source types/i })).toBeInTheDocument();
    expect(within(screen.getByRole('dialog')).getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
  });

  it('renders the route-backed agent dialog when the agent query is present', async () => {
    routeState.search = '?add=agent';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getAllByText('Add Pulse Agent').length).toBeGreaterThan(0);
    expect(screen.getByTestId('install-section')).toBeInTheDocument();
  });

  it('creates one onboarding tracker for a direct type route before the add dialog mounts', async () => {
    routeState.search = '?add=truenas';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(createInfrastructureOnboardingMetricsTrackerMock).toHaveBeenCalledTimes(1);
    expect(onboardingMetricsTrackers).toHaveLength(1);
    expect(onboardingMetricsTrackers[0]?.recordOpened).toHaveBeenCalledTimes(1);
  });

  it('opens the edit dialog directly from an existing source card', async () => {
    renderWorkspace({
      pveNodes: () => [{ name: 'zeus', host: 'https://10.0.0.1:8006' } as any],
    });

    await waitFor(() => expect(screen.getByRole('button', { name: /^Edit$/i })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /^Edit$/i }));

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Edit zeus')).toBeInTheDocument();
    expect(screen.getByTestId('proxmox-section')).toBeInTheDocument();
  });

  it('hides the add flow and source-manager actions in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    routeState.search = '?add=agent';
    renderWorkspace();

    expect(screen.queryByRole('button', { name: /Detect from address/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Add Proxmox VE/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /^Edit$/i })).toBeNull();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.getByText('Infrastructure sources')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Monitored systems' })).not.toBeInTheDocument();
  });
});
